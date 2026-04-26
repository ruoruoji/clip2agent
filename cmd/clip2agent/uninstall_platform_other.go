//go:build !darwin && !linux

package main

import "context"

func uninstallPlatformHotkey(_ context.Context, _ uninstallOptions, _ *uninstallLogger) {
	// no-op
}

func uninstallPlatformLogs(_ context.Context, _ uninstallOptions, _ *uninstallLogger) {
	// no-op
}
