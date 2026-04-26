//go:build !windows

package main

import "os"

func uninstallRemoveFile(path string) error {
	return os.Remove(path)
}

func uninstallRemoveAll(path string) error {
	return os.RemoveAll(path)
}
