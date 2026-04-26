package paths

import (
	"os"
	"path/filepath"
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
