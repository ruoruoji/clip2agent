//go:build darwin

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/ruoruoji/clip2agent/internal/errs"
	"github.com/ruoruoji/clip2agent/internal/paths"
)

func runHotkey(ctx context.Context, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "用法: clip2agent hotkey <install|uninstall|status|restart|logs|doctor|test>")
		return 2
	}
	label := paths.HotkeyLaunchAgentLabel
	uid := os.Getuid()
	plistPath := paths.HotkeyLaunchAgentPlistPath()
	if strings.TrimSpace(plistPath) == "" {
		errs.Fprint(os.Stderr, errs.E("E007", "获取 LaunchAgent plist 路径失败"))
		fmt.Fprintln(os.Stderr)
		return 1
	}
	logPath := paths.HotkeyLogPath()
	service := "gui/" + strconv.Itoa(uid) + "/" + label

	sub := strings.ToLower(strings.TrimSpace(args[0]))
	switch sub {
	case "doctor":
		cfgPath := paths.HotkeyConfigPath()
		fmt.Printf("hotkey_config: %s (exists=%v)\n", cfgPath, fileExists(cfgPath))
		fmt.Printf("log: %s\n", strings.TrimSpace(paths.LogPath()))
		if _, err := loadHotkeyConfig(); err != nil {
			errs.Fprint(os.Stderr, errs.E("E006", "hotkey 配置不可用", errs.Hint("运行: clip2agent config init --force"), errs.Cause(err)))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		fmt.Printf("menubar: C2A (clip2agent-hotkey)\n")
		fmt.Printf("note: 自动粘贴需要辅助功能权限（菜单栏中可查看/引导）\n")
		// LaunchAgent PATH：用于解释“终端可用但热键里 command not found”的常见问题。
		binDir := ""
		if hk, err := exec.LookPath("clip2agent-hotkey"); err == nil && strings.TrimSpace(hk) != "" {
			binDir = filepath.Dir(hk)
		} else {
			if selfExe, _ := os.Executable(); strings.TrimSpace(selfExe) != "" {
				binDir = filepath.Dir(selfExe)
			}
		}
		if strings.TrimSpace(binDir) != "" {
			pathEnv := binDir + ":/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
			fmt.Printf("launchagent_env_path: %s\n", pathEnv)
			if runtime.GOARCH == "arm64" {
				fmt.Printf("hint: Apple Silicon Homebrew 默认在 /opt/homebrew/bin\n")
			}
		}
		if _, err := exec.LookPath("clip2agent-hotkey"); err != nil {
			fmt.Printf("clip2agent-hotkey: missing\n")
			fmt.Printf("fix: clip2agent setup\n")
		} else {
			fmt.Printf("clip2agent-hotkey: ok\n")
		}
		fmt.Printf("launchagent_plist: %s (exists=%v)\n", plistPath, fileExists(plistPath))
		out, err := exec.CommandContext(ctx, "launchctl", "print", service).CombinedOutput()
		if err != nil {
			fmt.Printf("hotkey_service: not running\n")
			if s := strings.TrimSpace(string(out)); s != "" {
				fmt.Printf("hint: %s\n", s)
			}
			fmt.Printf("fix: clip2agent hotkey install\n")
			fmt.Printf("fix: clip2agent hotkey restart\n")
		} else {
			fmt.Printf("hotkey_service: running\n")
		}
		fmt.Printf("next: clip2agent hotkey test --id 1\n")
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
		traceID := newTraceID()
		hotkeyLogf("trigger start: trace_id=%s source=test binding=%s action=%s", traceID, hotkeyBindingLabel(cfg.Bindings[id-1], fmt.Sprintf("id=%d", id)), hotkeyActionPreview(cfg.Bindings[id-1].Action))
		if err := executeActionOnceDarwin(ctx, cfg.Bindings[id-1].Action, traceID); err != nil {
			hotkeyLogf("trigger failed: trace_id=%s source=test binding=%s err=%v", traceID, hotkeyBindingLabel(cfg.Bindings[id-1], fmt.Sprintf("id=%d", id)), err)
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
		hotkeyLogf("trigger success: trace_id=%s source=test binding=%s", traceID, hotkeyBindingLabel(cfg.Bindings[id-1], fmt.Sprintf("id=%d", id)))
		fmt.Println("ok")
		return 0
	case "install":
		selfExe, _ := os.Executable()
		hotkeyPath, err := exec.LookPath("clip2agent-hotkey")
		if err != nil {
			errs.Fprint(os.Stderr, errs.E("E003", "缺少 clip2agent-hotkey", errs.Hint("运行: clip2agent setup"), errs.Cause(err)))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		helperPath, err := resolveMacOSHelperPath(selfExe)
		if err != nil {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
		binDir := filepath.Dir(hotkeyPath)
		if err := installAndStartLaunchAgent(binDir, selfExe, helperPath, hotkeyPath); err != nil {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
		fmt.Printf("ok: %s\n", plistPath)
		return 0
	case "uninstall":
		_ = exec.CommandContext(ctx, "launchctl", "bootout", service).Run()
		_ = os.Remove(plistPath)
		fmt.Println("ok")
		return 0
	case "restart":
		_ = exec.CommandContext(ctx, "launchctl", "kickstart", "-k", service).Run()
		fmt.Println("ok")
		return 0
	case "status":
		cmd := exec.CommandContext(ctx, "launchctl", "print", service)
		out, err := cmd.CombinedOutput()
		if len(out) > 0 {
			fmt.Print(string(out))
			if out[len(out)-1] != '\n' {
				fmt.Println()
			}
		}
		if err != nil {
			return 1
		}
		return 0
	case "logs":
		cmd := exec.CommandContext(ctx, "tail", "-f", logPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return 1
		}
		return 0
	default:
		errs.Fprint(os.Stderr, errs.E("E006", "不支持的 hotkey 子命令", errs.Hint("支持: install/uninstall/status/restart/logs/doctor/test")))
		fmt.Fprintln(os.Stderr)
		return 2
	}
}

func executeActionOnceDarwin(ctx context.Context, action hotkeyAction, traceID string) error {
	t := strings.ToLower(strings.TrimSpace(action.Type))
	switch t {
	case "clip2agent":
		cmd := resolveClip2AgentCommandDarwin(action)
		return runExternal(ctx, cmd, action.Args, action.Env, traceID, "hotkey")
	case "exec":
		if strings.TrimSpace(action.Command) == "" {
			return nil
		}
		return runExternal(ctx, action.Command, action.Args, action.Env, traceID, "hotkey")
	default:
		return errs.E("E006", "不支持的 hotkey action.type", errs.Hint("支持: clip2agent/exec"))
	}
}

func resolveClip2AgentCommandDarwin(action hotkeyAction) string {
	if strings.TrimSpace(action.Command) != "" {
		return strings.TrimSpace(action.Command)
	}
	if v := strings.TrimSpace(os.Getenv("CLIP2AGENT_BIN")); v != "" {
		return v
	}
	return "clip2agent"
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func resolveMacOSHelperPath(selfExe string) (string, error) {
	if v := strings.TrimSpace(os.Getenv("CLIP2AGENT_MACOS_HELPER")); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v, nil
		}
		return "", errs.E("E003", "CLIP2AGENT_MACOS_HELPER 指向的文件不存在", errs.Hint(v))
	}
	if strings.TrimSpace(selfExe) != "" {
		cand := filepath.Join(filepath.Dir(selfExe), "clip2agent-macos")
		if _, err := os.Stat(cand); err == nil {
			return cand, nil
		}
	}
	if p, err := exec.LookPath("clip2agent-macos"); err == nil {
		return p, nil
	}
	return "", errs.E("E003", "缺少 macOS 剪贴板 helper（clip2agent-macos）", errs.Hint("运行: clip2agent setup"))
}
