//go:build !darwin

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ruoruoji/clip2agent/internal/errs"
)

func runSetup(_ context.Context, _ []string) int {
	errs.Fprint(os.Stderr, errs.E("E006", "setup 暂仅支持 macOS", errs.Hint("Windows/Linux 支持已包含在 CLI 主命令里；热键见: clip2agent hotkey --help")))
	fmt.Fprintln(os.Stderr)
	return 2
}
