//go:build !darwin && !linux

package main

import (
	"context"
	"strings"

	"github.com/ruoruoji/clip2agent/internal/paths"
)

func uninstallPlatformHotkey(_ context.Context, _ uninstallOptions, _ *uninstallLogger) {
	// no-op
}

func uninstallPlatformLogs(_ context.Context, _ uninstallOptions, log *uninstallLogger) {
	if p := strings.TrimSpace(paths.LogPath()); p != "" {
		log.removeFile(p, "删除日志")
	}
}
