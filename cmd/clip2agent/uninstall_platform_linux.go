//go:build linux

package main

import (
	"context"
	"strings"

	"github.com/ruoruoji/clip2agent/internal/paths"
)

func uninstallPlatformHotkey(_ context.Context, _ uninstallOptions, log *uninstallLogger) {
	// 与 `clip2agent hotkey uninstall` 对齐：只删除 xbindkeys/autostart 文件。
	log.removeFile(linuxXbindkeysConfigPath(), "删除 xbindkeys 配置")
	log.removeFile(linuxAutostartDesktopPath(), "删除 autostart desktop")
}

func uninstallPlatformLogs(_ context.Context, _ uninstallOptions, log *uninstallLogger) {
	if p := strings.TrimSpace(paths.LogPath()); p != "" {
		log.removeFile(p, "删除日志")
	}
}
