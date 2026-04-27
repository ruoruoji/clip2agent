package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ruoruoji/clip2agent/internal/paths"
)

func newTraceID() string {
	// 轻量 trace id：用于把 hotkey 侧日志与 CLI 侧 invocation log 关联起来。
	// 形式：<unixnano>-<pid>
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid())
}

func hotkeyLogf(format string, args ...any) {
	logPath := strings.TrimSpace(paths.HotkeyLogPath())
	if logPath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "[clip2agent-hotkey] %s %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
}

func hotkeyBindingLabel(b hotkeyBinding, fallback string) string {
	if v := strings.TrimSpace(b.Name); v != "" {
		return v
	}
	if v := strings.TrimSpace(b.ID); v != "" {
		return v
	}
	if v := strings.TrimSpace(b.Shortcut); v != "" {
		return v
	}
	return fallback
}

func hotkeyCommandPreview(command string, args []string) string {
	joined := strings.TrimSpace(strings.Join(append([]string{command}, args...), " "))
	if joined == "" {
		return "-"
	}
	if len(joined) <= 180 {
		return joined
	}
	return joined[:180] + "…"
}

func hotkeyOutputPreview(s string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return "-"
	}
	if len(trimmed) <= 180 {
		return trimmed
	}
	return trimmed[:180] + "…"
}

func hotkeyActionPreview(action hotkeyAction) string {
	t := strings.ToLower(strings.TrimSpace(action.Type))
	switch t {
	case "clip2agent":
		return hotkeyCommandPreview(resolveHotkeyActionCommand(action), action.Args)
	case "exec":
		return hotkeyCommandPreview(strings.TrimSpace(action.Command), action.Args)
	default:
		return t
	}
}

func resolveHotkeyActionCommand(action hotkeyAction) string {
	if v := strings.TrimSpace(action.Command); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("CLIP2AGENT_BIN")); v != "" {
		return v
	}
	if runtime.GOOS == "windows" {
		if exe, err := os.Executable(); err == nil && strings.TrimSpace(exe) != "" {
			return exe
		}
	}
	return "clip2agent"
}

func runHotkeyLogs(ctx context.Context) int {
	logPath := strings.TrimSpace(paths.HotkeyLogPath())
	if logPath == "" {
		fmt.Fprintln(os.Stderr, "当前平台未定义 hotkey 日志路径")
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "创建 hotkey 日志目录失败: %v\n", err)
		return 1
	}
	if _, err := os.OpenFile(logPath, os.O_CREATE, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "创建 hotkey 日志文件失败: %v\n", err)
		return 1
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		ps, err := exec.LookPath("powershell.exe")
		if err != nil {
			ps, err = exec.LookPath("powershell")
			if err != nil {
				fmt.Printf("hotkey_log: %s\n", logPath)
				fmt.Fprintln(os.Stderr, "未找到 PowerShell，无法实时跟踪日志；请手动打开上述文件")
				return 1
			}
		}
		cmd = exec.CommandContext(ctx, ps, "-NoProfile", "-Command", fmt.Sprintf("Get-Content -Path '%s' -Wait", strings.ReplaceAll(logPath, "'", "''")))
	} else {
		cmd = exec.CommandContext(ctx, "tail", "-f", logPath)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return 1
	}
	return 0
}
