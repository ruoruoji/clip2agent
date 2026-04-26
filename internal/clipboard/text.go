package clipboard

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/ruoruoji/clip2agent/internal/errs"
)

// WriteText 将文本写入系统剪贴板。
// 目的：配合系统快捷键一键触发（命令执行后用户直接在终端粘贴）。
func WriteText(ctx context.Context, text string) error {
	switch runtime.GOOS {
	case "darwin":
		return writeTextByCmd(ctx, "pbcopy", nil, text, "E003", "缺少 pbcopy")
	case "linux":
		// Wayland 优先 wl-copy
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			if _, err := exec.LookPath("wl-copy"); err == nil {
				return writeTextByCmd(ctx, "wl-copy", nil, text, "E003", "缺少 wl-copy")
			}
			return errs.E("E003", "缺少 wl-copy", errs.Hint("安装 wl-clipboard（如: sudo apt install wl-clipboard）"))
		}
		// X11：优先 xclip，再 xsel
		if _, err := exec.LookPath("xclip"); err == nil {
			return writeTextByCmd(ctx, "xclip", []string{"-selection", "clipboard"}, text, "E003", "缺少 xclip")
		}
		if _, err := exec.LookPath("xsel"); err == nil {
			return writeTextByCmd(ctx, "xsel", []string{"--clipboard", "--input"}, text, "E003", "缺少 xsel")
		}
		return errs.E("E003", "缺少 Linux 剪贴板写入工具", errs.Hint("Wayland: 安装 wl-clipboard；X11: 安装 xclip 或 xsel"))
	case "windows":
		// Windows: 通过 PowerShell 的 Set-Clipboard 写回文本。
		ps := "powershell"
		if _, err := exec.LookPath(ps); err != nil {
			ps = "powershell.exe"
			if _, err := exec.LookPath(ps); err != nil {
				ps = "pwsh"
				if _, err := exec.LookPath(ps); err != nil {
					ps = "pwsh.exe"
				}
			}
		}
		cmd := exec.CommandContext(ctx, ps, "-NoProfile", "-NonInteractive", "-Command", "Set-Clipboard -Value ([Console]::In.ReadToEnd())")
		cmd.Stdin = bytes.NewBufferString(normalizeNewline(text))
		if out, err := cmd.CombinedOutput(); err != nil {
			hint := strings.TrimSpace(string(out))
			if hint != "" {
				return errs.E("E003", "写回剪贴板失败", errs.Hint(hint), errs.Cause(err))
			}
			return errs.WrapCode("E003", "写回剪贴板失败", err)
		}
		return nil
	default:
		return errs.E("E006", "当前平台暂不支持写回剪贴板", errs.Hint("仅支持 macOS/Linux/Windows"))
	}
}

func writeTextByCmd(ctx context.Context, bin string, args []string, text string, code string, msg string) error {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = bytes.NewBufferString(normalizeNewline(text))
	if out, err := cmd.CombinedOutput(); err != nil {
		hint := ""
		if len(out) > 0 {
			hint = strings.TrimSpace(string(out))
		}
		if hint != "" {
			return errs.E(code, msg, errs.Hint(hint), errs.Cause(err))
		}
		return errs.WrapCode(code, msg, err)
	}
	return nil
}

func normalizeNewline(s string) string {
	// 保持用户输出原样，但确保末尾有换行，便于粘贴体验。
	if s == "" {
		return ""
	}
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}
