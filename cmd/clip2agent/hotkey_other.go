//go:build !darwin && !windows && !linux

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ruoruoji/clip2agent/internal/errs"
)

func runHotkey(_ context.Context, _ []string) int {
	errs.Fprint(os.Stderr, errs.E("E006", "当前平台不支持 hotkey"))
	fmt.Fprintln(os.Stderr)
	return 2
}
