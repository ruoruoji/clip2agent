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
func runExternal(ctx context.Context, command string, args []string, extraEnv map[string]string, traceID string, invoker string) error {
	if strings.TrimSpace(command) == "" {
		if strings.TrimSpace(traceID) != "" {
			hotkeyLogf("command skipped: trace_id=%s empty_command=true", traceID)
		} else {
			hotkeyLogf("command skipped: empty command")
		}
		return nil
	}
	if strings.TrimSpace(traceID) != "" {
		hotkeyLogf("command start: trace_id=%s %s", traceID, hotkeyCommandPreview(command, args))
	} else {
		hotkeyLogf("command start: %s", hotkeyCommandPreview(command, args))
	}
	cmd := exec.CommandContext(ctx, command, args...)
	env := os.Environ()
	if strings.TrimSpace(traceID) != "" {
		env = append(env, "CLIP2AGENT_TRACE_ID="+strings.TrimSpace(traceID))
	}
	if strings.TrimSpace(invoker) != "" {
		env = append(env, "CLIP2AGENT_INVOKER="+strings.TrimSpace(invoker))
	}
	if extraEnv != nil {
		for k, v := range extraEnv {
			env = append(env, k+"="+v)
		}
	}
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		hint := strings.TrimSpace(string(out))
		if strings.TrimSpace(traceID) != "" {
			hotkeyLogf("command failed: trace_id=%s %s output=%s", traceID, hotkeyCommandPreview(command, args), hotkeyOutputPreview(hint))
		} else {
			hotkeyLogf("command failed: %s output=%s", hotkeyCommandPreview(command, args), hotkeyOutputPreview(hint))
		}
		if hint != "" {
			return errs.E("E003", fmt.Sprintf("hotkey action 执行失败: %s", command), errs.Hint(hint), errs.Cause(err))
		}
		return errs.WrapCode("E003", fmt.Sprintf("hotkey action 执行失败: %s", command), err)
	}
	if strings.TrimSpace(traceID) != "" {
		hotkeyLogf("command ok: trace_id=%s %s output=%s", traceID, hotkeyCommandPreview(command, args), hotkeyOutputPreview(string(out)))
	} else {
		hotkeyLogf("command ok: %s output=%s", hotkeyCommandPreview(command, args), hotkeyOutputPreview(string(out)))
	}
	return nil
}
