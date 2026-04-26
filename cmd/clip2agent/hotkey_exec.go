package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ruoruoji/clip2agent/internal/errs"
)

// runExternal 执行外部命令（用于 hotkey action）。
// 设计：成功时不打印 stdout，避免热键触发时输出堆积；失败时打印简明错误与建议。
func runExternal(ctx context.Context, command string, args []string, extraEnv map[string]string) error {
	if strings.TrimSpace(command) == "" {
		return nil
	}
	cmd := exec.CommandContext(ctx, command, args...)
	if extraEnv != nil {
		env := os.Environ()
		for k, v := range extraEnv {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		hint := strings.TrimSpace(string(out))
		if hint != "" {
			return errs.E("E003", fmt.Sprintf("hotkey action 执行失败: %s", command), errs.Hint(hint), errs.Cause(err))
		}
		return errs.WrapCode("E003", fmt.Sprintf("hotkey action 执行失败: %s", command), err)
	}
	return nil
}
