//go:build darwin

package paths

import (
	"os"
	"path/filepath"
	"strings"
)

func HotkeyLaunchAgentPlistPath() string {
	h, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(h) == "" {
		return ""
	}
	return filepath.Join(h, "Library", "LaunchAgents", HotkeyLaunchAgentLabel+".plist")
}
