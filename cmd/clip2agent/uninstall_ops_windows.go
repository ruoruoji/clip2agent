//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func uninstallRemoveFile(path string) error {
	if err := os.Remove(path); err == nil {
		return nil
	} else {
		// 文件被占用时常见；退化为后台延迟删除。
		if err2 := scheduleWindowsDelete(false, path); err2 == nil {
			return uninstallScheduled{msg: "scheduled"}
		}
		return err
	}
}

func uninstallRemoveAll(path string) error {
	if err := os.RemoveAll(path); err == nil {
		return nil
	} else {
		if err2 := scheduleWindowsDelete(true, path); err2 == nil {
			return uninstallScheduled{msg: "scheduled"}
		}
		return err
	}
}

func scheduleWindowsDelete(isDir bool, path string) error {
	// 简化：不支持包含双引号的路径，避免 cmd 注入与转义复杂度。
	if strings.Contains(path, "\"") {
		return fmt.Errorf("path contains quote: %q", path)
	}
	quoted := "\"" + path + "\""
	var action string
	if isDir {
		action = "rmdir /S /Q " + quoted
	} else {
		action = "del /F /Q " + quoted
	}
	// 延迟 2 秒再删，尽量等当前进程退出。
	// 使用 ping 作为无依赖延时。
	cmdline := "ping -n 3 127.0.0.1 >NUL & " + action
	cmd := exec.Command("cmd.exe", "/C", cmdline)
	return cmd.Start()
}
