package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const HotkeyLaunchAgentLabel = "dev.clip2agent-hotkey"

func HotkeyConfigPath() string {
	if v := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); v != "" {
		return filepath.Join(v, "clip2agent", "hotkey.json")
	}
	h, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(h) == "" {
		// 兜底：尽量不 panic
		return filepath.Join(".", "hotkey.json")
	}
	return filepath.Join(h, ".config", "clip2agent", "hotkey.json")
}

func DefaultKeptDir() string {
	// 持久目录：尽量落在用户 cache 目录中，避免系统临时目录被清理。
	if d, err := os.UserCacheDir(); err == nil && strings.TrimSpace(d) != "" {
		return filepath.Join(d, "clip2agent", "kept")
	}
	h, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(h) == "" {
		return filepath.Join(".", "clip2agent", "kept")
	}
	return filepath.Join(h, ".cache", "clip2agent", "kept")
}

func CLILogPath() string {
	return LogPath()
}

// LogPath returns the unified log file path.
//
// Note: we intentionally keep the directory stable per platform:
// - darwin: ~/Library/Logs
// - linux:  ${XDG_STATE_HOME:-~/.local/state}/clip2agent
// - windows: %LocalAppData%\clip2agent (via os.UserCacheDir)
func LogPath() string {
	switch runtime.GOOS {
	case "darwin":
		h, err := os.UserHomeDir()
		if err != nil || strings.TrimSpace(h) == "" {
			return ""
		}
		return filepath.Join(h, "Library", "Logs", "clip2agent.log")
	case "linux":
		if v := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); v != "" {
			return filepath.Join(v, "clip2agent", "clip2agent.log")
		}
		h, err := os.UserHomeDir()
		if err != nil || strings.TrimSpace(h) == "" {
			return ""
		}
		return filepath.Join(h, ".local", "state", "clip2agent", "clip2agent.log")
	case "windows":
		d, err := os.UserCacheDir()
		if err != nil || strings.TrimSpace(d) == "" {
			return ""
		}
		return filepath.Join(d, "clip2agent", "clip2agent.log")
	default:
		return ""
	}
}

func HotkeyLogPath() string {
	return LogPath()
}
