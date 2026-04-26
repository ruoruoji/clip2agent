//go:build linux

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ruoruoji/clip2agent/internal/errs"
	"github.com/ruoruoji/clip2agent/internal/paths"
)

// Linux 全局热键：
// - X11：通过 xbindkeys（外部工具）实现“全局监听按键 -> 执行命令”。
// - Wayland：不做通用实现（安全模型限制）。
func runHotkey(ctx context.Context, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "用法: clip2agent hotkey <install|uninstall|status|print|trigger>")
		return 2
	}

	sub := strings.ToLower(strings.TrimSpace(args[0]))
	switch sub {
	case "doctor":
		cfgPath := paths.HotkeyConfigPath()
		fmt.Printf("hotkey_config: %s (exists=%v)\n", cfgPath, fileExists(cfgPath))
		if _, err := loadHotkeyConfig(); err != nil {
			errs.Fprint(os.Stderr, errs.E("E006", "hotkey 配置不可用", errs.Hint("运行: clip2agent config init --force"), errs.Cause(err)))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		if os.Getenv("WAYLAND_DISPLAY") != "" && os.Getenv("DISPLAY") == "" {
			fmt.Println("session: wayland")
			fmt.Println("note: Wayland 下无法通用实现全局监听按键")
			fmt.Println("fix: 使用桌面环境快捷键绑定到: clip2agent hotkey trigger --id 1")
			fmt.Println("next: clip2agent hotkey test --id 1")
			return 0
		}
		fmt.Println("session: x11")
		fmt.Printf("xbindkeys_in_path: %v\n", inPath("xbindkeys"))
		fmt.Printf("xbindkeys_config: %s (exists=%v)\n", linuxXbindkeysConfigPath(), fileExists(linuxXbindkeysConfigPath()))
		fmt.Printf("autostart_desktop: %s (exists=%v)\n", linuxAutostartDesktopPath(), fileExists(linuxAutostartDesktopPath()))
		fmt.Println("fix: clip2agent hotkey install")
		fmt.Println("next: clip2agent hotkey test --id 1")
		return 0
	case "test":
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
		if err := executeActionOnce(ctx, cfg.Bindings[id-1].Action); err != nil {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
		fmt.Println("ok")
		return 0
	case "trigger":
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
		// 直接执行一次（给 xbindkeys 调用）
		if err := executeActionOnce(ctx, cfg.Bindings[id-1].Action); err != nil {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
		return 0

	case "print":
		cfg, err := loadHotkeyConfig()
		if err != nil {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
		text, err := renderXbindkeysConfig(cfg)
		if err != nil {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
		fmt.Print(text)
		if len(text) == 0 || text[len(text)-1] != '\n' {
			fmt.Println()
		}
		return 0

	case "install":
		// Wayland 不做通用全局热键。
		if os.Getenv("WAYLAND_DISPLAY") != "" && os.Getenv("DISPLAY") == "" {
			errs.Fprint(os.Stderr, errs.E("E006", "Wayland 下无法通用实现全局热键", errs.Hint("请使用桌面环境的快捷键绑定到: clip2agent hotkey trigger --id 1")))
			fmt.Fprintln(os.Stderr)
			return 2
		}
		if _, err := exec.LookPath("xbindkeys"); err != nil {
			errs.Fprint(os.Stderr, errs.E("E003", "缺少 xbindkeys", errs.Hint("Debian/Ubuntu: sudo apt install xbindkeys"), errs.Cause(err)))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		cfg, err := loadHotkeyConfig()
		if err != nil {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
		text, err := renderXbindkeysConfig(cfg)
		if err != nil {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
		cfgPath := linuxXbindkeysConfigPath()
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
			errs.Fprint(os.Stderr, errs.WrapCode("E007", "创建配置目录失败", err))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		if err := os.WriteFile(cfgPath, []byte(text), 0o600); err != nil {
			errs.Fprint(os.Stderr, errs.WrapCode("E007", "写入 xbindkeys 配置失败", err))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		// 写 autostart
		desktopPath := linuxAutostartDesktopPath()
		if err := os.MkdirAll(filepath.Dir(desktopPath), 0o755); err != nil {
			errs.Fprint(os.Stderr, errs.WrapCode("E007", "创建 autostart 目录失败", err))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		desktop := renderAutostartDesktop(cfgPath)
		_ = os.WriteFile(desktopPath, []byte(desktop), 0o644)

		// 尝试启动（不保证所有环境都能成功）
		_ = exec.CommandContext(ctx, "xbindkeys", "-f", cfgPath).Run()
		fmt.Printf("ok: %s\n", cfgPath)
		fmt.Printf("ok: %s\n", desktopPath)
		return 0

	case "uninstall":
		_ = os.Remove(linuxXbindkeysConfigPath())
		_ = os.Remove(linuxAutostartDesktopPath())
		fmt.Println("ok")
		return 0

	case "status":
		cfgPath := linuxXbindkeysConfigPath()
		desktopPath := linuxAutostartDesktopPath()
		fmt.Printf("xbindkeys_in_path: %v\n", inPath("xbindkeys"))
		fmt.Printf("xbindkeys_config: %s (exists=%v)\n", cfgPath, fileExists(cfgPath))
		fmt.Printf("autostart_desktop: %s (exists=%v)\n", desktopPath, fileExists(desktopPath))
		fmt.Printf("hint: use `clip2agent hotkey print` to inspect generated bindings\n")
		return 0

	default:
		errs.Fprint(os.Stderr, errs.E("E006", "不支持的 hotkey 子命令", errs.Hint("支持: install/uninstall/status/print/trigger/doctor/test")))
		fmt.Fprintln(os.Stderr)
		return 2
	}
}

func executeActionOnce(ctx context.Context, action hotkeyAction) error {
	t := strings.ToLower(strings.TrimSpace(action.Type))
	switch t {
	case "clip2agent":
		cmd := resolveClip2AgentCommandPortable(action)
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

func resolveClip2AgentCommandPortable(action hotkeyAction) string {
	if strings.TrimSpace(action.Command) != "" {
		return strings.TrimSpace(action.Command)
	}
	if v := strings.TrimSpace(os.Getenv("CLIP2AGENT_BIN")); v != "" {
		return v
	}
	// xbindkeys 场景：依赖 PATH。
	return "clip2agent"
}

func renderXbindkeysConfig(cfg hotkeyConfigFile) (string, error) {
	var b strings.Builder
	b.WriteString("# Generated by clip2agent\n")
	b.WriteString("# Each binding executes: clip2agent hotkey trigger --id N\n\n")
	for i, bd := range cfg.Bindings {
		keys, err := shortcutToXbindkeys(bd.Shortcut)
		if err != nil {
			return "", errs.E("E006", "Linux 热键转换失败", errs.Hint(err.Error()))
		}
		cmd := fmt.Sprintf("clip2agent hotkey trigger --id %d", i+1)
		b.WriteString("\"")
		b.WriteString(cmd)
		b.WriteString("\"\n")
		b.WriteString("  ")
		b.WriteString(keys)
		b.WriteString("\n\n")
	}
	return b.String(), nil
}

func shortcutToXbindkeys(raw string) (string, error) {
	low := strings.ToLower(strings.TrimSpace(raw))
	if low == "" {
		return "", fmt.Errorf("shortcut 不能为空")
	}
	parts := strings.Split(low, "+")
	mods := map[string]bool{}
	key := ""
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		switch p {
		case "control", "ctrl":
			mods["Control"] = true
		case "alt", "option":
			mods["Alt"] = true
		case "shift":
			mods["Shift"] = true
		case "super", "win", "command", "cmd":
			mods["Mod4"] = true
		default:
			key = p
		}
	}
	if key == "" {
		return "", fmt.Errorf("shortcut 缺少 key: %s", raw)
	}
	keyMap := map[string]string{
		"space":  "space",
		"tab":    "Tab",
		"enter":  "Return",
		"return": "Return",
		"esc":    "Escape",
		"escape": "Escape",
	}
	if v, ok := keyMap[key]; ok {
		key = v
	}
	// 仅做最小覆盖：字母/数字/常用 special。
	if len(key) == 1 {
		c := key[0]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			// ok
		} else {
			return "", fmt.Errorf("不支持的 key: %s", key)
		}
	}
	if len(key) > 1 {
		// Return/Tab/Escape/space 等
		// ok
	}

	// xbindkeys: Mod+Key（顺序不敏感，但输出稳定）
	order := []string{"Control", "Alt", "Shift", "Mod4"}
	out := []string{}
	for _, m := range order {
		if mods[m] {
			out = append(out, m)
		}
	}
	out = append(out, key)
	return strings.Join(out, "+"), nil
}

func linuxXbindkeysConfigPath() string {
	base := linuxXdgConfigHome()
	return filepath.Join(base, "clip2agent", "xbindkeys.conf")
}

func linuxAutostartDesktopPath() string {
	base := linuxXdgConfigHome()
	return filepath.Join(base, "autostart", "clip2agent-xbindkeys.desktop")
}

func linuxXdgConfigHome() string {
	if v := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); v != "" {
		return v
	}
	h, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(h) == "" {
		return "."
	}
	return filepath.Join(h, ".config")
}

func renderAutostartDesktop(cfgPath string) string {
	// 最小 XDG autostart .desktop
	return fmt.Sprintf("[Desktop Entry]\nType=Application\nName=clip2agent-hotkey (xbindkeys)\nExec=xbindkeys -f %s\nX-GNOME-Autostart-enabled=true\n", cfgPath)
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func inPath(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}
