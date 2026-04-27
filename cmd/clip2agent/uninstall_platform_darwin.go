//go:build darwin

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ruoruoji/clip2agent/internal/paths"
)

func uninstallPlatformHotkey(ctx context.Context, _ uninstallOptions, log *uninstallLogger) {
	label := paths.HotkeyLaunchAgentLabel
	service := fmt.Sprintf("gui/%d/%s", os.Getuid(), label)
	if log.op.dryRun {
		log.planf("卸载 hotkey LaunchAgent（bootout）: launchctl bootout %s", service)
	} else {
		out, err := exec.CommandContext(ctx, "launchctl", "bootout", service).CombinedOutput()
		if err != nil {
			// best-effort：服务可能未安装/未运行。
			if s := strings.TrimSpace(string(out)); s != "" {
				log.warnf("launchctl bootout 失败: %s", s)
			} else {
				log.warnf("launchctl bootout 失败: %v", err)
			}
		}
	}

	plist := strings.TrimSpace(paths.HotkeyLaunchAgentPlistPath())
	if plist != "" {
		log.removeFile(plist, "删除 LaunchAgent plist")
	}
}

func uninstallPlatformLogs(_ context.Context, _ uninstallOptions, log *uninstallLogger) {
	if _, err := os.UserHomeDir(); err != nil {
		log.warnf("无法获取 HOME，跳过删除 hotkey 日志")
		return
	}
	seen := map[string]bool{}
	if p := strings.TrimSpace(paths.LogPath()); p != "" {
		if !seen[p] {
			seen[p] = true
			log.removeFile(p, "删除日志")
		}
	}
}
