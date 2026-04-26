//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/ruoruoji/clip2agent/internal/errs"
	"github.com/ruoruoji/clip2agent/internal/paths"
)

// Windows：
// - `clip2agent hotkey run`：前台阻塞运行，全局热键触发后执行配置动作
// - `clip2agent hotkey trigger --id N`：执行单个 binding（用于调试/脚本）
func runHotkey(ctx context.Context, args []string) int {
	if len(args) < 1 {
		errs.Fprint(os.Stderr, errs.E("E006", "用法: clip2agent hotkey <run|trigger|doctor|test>"))
		fmt.Fprintln(os.Stderr)
		return 2
	}
	sub := strings.ToLower(strings.TrimSpace(args[0]))
	if sub == "doctor" {
		cfgPath := paths.HotkeyConfigPath()
		_, err := os.Stat(cfgPath)
		fmt.Printf("hotkey_config: %s (exists=%v)\n", cfgPath, err == nil)
		if _, err := loadHotkeyConfig(); err != nil {
			errs.Fprint(os.Stderr, errs.E("E006", "hotkey 配置不可用", errs.Hint("运行: clip2agent config init --force"), errs.Cause(err)))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		fmt.Println("note: Windows 全局热键通过 `clip2agent hotkey run` 前台常驻实现")
		fmt.Println("next: clip2agent hotkey test --id 1")
		return 0
	}
	if sub == "test" {
		id := 1
		for i := 1; i < len(args); i++ {
			if args[i] == "--id" && i+1 < len(args) {
				id, _ = strconv.Atoi(args[i+1])
				i++
			}
		}
		cfg, err := loadHotkeyConfig()
		if err != nil {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
		if id <= 0 || id > len(cfg.Bindings) {
			errs.Fprint(os.Stderr, errs.E("E006", "binding id 超出范围"))
			fmt.Fprintln(os.Stderr)
			return 2
		}
		if err := executeHotkeyAction(ctx, cfg.Bindings[id-1].Action); err != nil {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
		fmt.Println("ok")
		return 0
	}
	if sub == "trigger" {
		id := 0
		for i := 1; i < len(args); i++ {
			if args[i] == "--id" && i+1 < len(args) {
				id, _ = strconv.Atoi(args[i+1])
				i++
			}
		}
		if id <= 0 {
			errs.Fprint(os.Stderr, errs.E("E006", "缺少 --id", errs.Hint("示例: clip2agent hotkey trigger --id 1")))
			fmt.Fprintln(os.Stderr)
			return 2
		}
		cfg, err := loadHotkeyConfig()
		if err != nil {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
		if id > len(cfg.Bindings) {
			errs.Fprint(os.Stderr, errs.E("E006", "binding id 超出范围"))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		if err := executeHotkeyAction(ctx, cfg.Bindings[id-1].Action); err != nil {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
		return 0
	}
	if sub != "run" {
		errs.Fprint(os.Stderr, errs.E("E006", "不支持的 hotkey 子命令", errs.Hint("支持: run/trigger/doctor/test")))
		fmt.Fprintln(os.Stderr)
		return 2
	}

	cfg, err := loadHotkeyConfig()
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 1
	}
	if len(cfg.Bindings) == 0 {
		errs.Fprint(os.Stderr, errs.E("E006", "bindings 为空", errs.Hint("运行: clip2agent config init")))
		fmt.Fprintln(os.Stderr)
		return 1
	}

	// WinAPI
	user32 := syscall.NewLazyDLL("user32.dll")
	procRegisterHotKey := user32.NewProc("RegisterHotKey")
	procUnregisterHotKey := user32.NewProc("UnregisterHotKey")
	procGetMessageW := user32.NewProc("GetMessageW")
	procTranslateMessage := user32.NewProc("TranslateMessage")
	procDispatchMessageW := user32.NewProc("DispatchMessageW")

	const (
		WM_HOTKEY   = 0x0312
		MOD_ALT     = 0x0001
		MOD_CONTROL = 0x0002
		MOD_SHIFT   = 0x0004
		MOD_WIN     = 0x0008
	)

	type MSG struct {
		hwnd    uintptr
		message uint32
		wParam  uintptr
		lParam  uintptr
		time    uint32
		pt      struct{ x, y int32 }
	}

	bindingsByID := map[int]hotkeyBinding{}
	registered := []int{}
	nextID := 1
	for _, b := range cfg.Bindings {
		vk, mods, perr := parseShortcutWindows(b.Shortcut)
		if perr != nil {
			errs.Fprint(os.Stderr, errs.E("E006", "快捷键解析失败", errs.Hint(perr.Error())))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		var modFlags uintptr
		if mods.control {
			modFlags |= MOD_CONTROL
		}
		if mods.alt {
			modFlags |= MOD_ALT
		}
		if mods.shift {
			modFlags |= MOD_SHIFT
		}
		if mods.win {
			modFlags |= MOD_WIN
		}

		r1, _, _ := procRegisterHotKey.Call(0, uintptr(nextID), modFlags, uintptr(vk))
		if r1 == 0 {
			errs.Fprint(os.Stderr, errs.E("E003", "RegisterHotKey 失败", errs.Hint("可能与系统/应用快捷键冲突")))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		bindingsByID[nextID] = b
		registered = append(registered, nextID)
		nextID++
	}

	// 退出时注销
	defer func() {
		for _, id := range registered {
			procUnregisterHotKey.Call(0, uintptr(id))
		}
	}()

	// debounce
	var last time.Time
	runAction := func(b hotkeyBinding) {
		if time.Since(last) < 200*time.Millisecond {
			return
		}
		last = time.Now()
		if err := executeHotkeyAction(ctx, b.Action); err != nil {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
		}
	}

	// 消息循环（阻塞）
	var msg MSG
	for {
		r1, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r1) == 0 {
			return 0 // WM_QUIT
		}
		if int32(r1) < 0 {
			errs.Fprint(os.Stderr, errs.E("E003", "GetMessageW 失败"))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		if msg.message == WM_HOTKEY {
			id := int(msg.wParam)
			if b, ok := bindingsByID[id]; ok {
				runAction(b)
			}
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

type winMods struct{ control, alt, shift, win bool }

func parseShortcutWindows(raw string) (vk uint32, mods winMods, err error) {
	low := strings.ToLower(strings.TrimSpace(raw))
	if low == "" {
		return 0, winMods{}, fmt.Errorf("shortcut 不能为空")
	}
	parts := strings.Split(low, "+")
	var key string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		switch p {
		case "control", "ctrl":
			mods.control = true
		case "option", "alt":
			mods.alt = true
		case "shift":
			mods.shift = true
		case "win", "command", "cmd":
			mods.win = true
		default:
			key = p
		}
	}
	if key == "" {
		return 0, winMods{}, fmt.Errorf("shortcut 缺少 key: %s", raw)
	}
	if len(key) == 1 {
		c := key[0]
		if c >= 'a' && c <= 'z' {
			return uint32('A' + (c - 'a')), mods, nil
		}
		if c >= '0' && c <= '9' {
			return uint32(c), mods, nil
		}
	}
	keyMap := map[string]uint32{
		"space":  0x20,
		"tab":    0x09,
		"enter":  0x0D,
		"return": 0x0D,
		"esc":    0x1B,
		"escape": 0x1B,
	}
	if v, ok := keyMap[key]; ok {
		return v, mods, nil
	}
	return 0, winMods{}, fmt.Errorf("不支持的 key: %s", key)
}

func executeHotkeyAction(ctx context.Context, action hotkeyAction) error {
	t := strings.ToLower(strings.TrimSpace(action.Type))
	switch t {
	case "clip2agent":
		cmd := resolveClip2AgentCommandWindows(action)
		return runExternal(ctx, cmd, action.Args, action.Env)
	case "exec":
		if strings.TrimSpace(action.Command) == "" {
			return nil
		}
		return runExternal(ctx, action.Command, action.Args, action.Env)
	default:
		return errs.E("E006", "不支持的 hotkey action.type", errs.Hint("支持: clip2agent/exec"))
	}
}

func resolveClip2AgentCommandWindows(action hotkeyAction) string {
	if strings.TrimSpace(action.Command) != "" {
		return strings.TrimSpace(action.Command)
	}
	if v := strings.TrimSpace(os.Getenv("CLIP2AGENT_BIN")); v != "" {
		return v
	}
	if exe, err := os.Executable(); err == nil && strings.TrimSpace(exe) != "" {
		return exe
	}
	return "clip2agent"
}
