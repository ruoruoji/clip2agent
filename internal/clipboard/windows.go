package clipboard

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/ruoruoji/clip2agent/internal/errs"
)

// Windows 通过 PowerShell 从剪贴板导出图片（PNG）。
// 目标：零额外依赖（不引入 CGO / WinAPI 绑定），靠系统自带 PowerShell。
// 注意：需要 STA 线程，使用 -Sta。
type windowsProvider struct{}

func (p *windowsProvider) GetImage(ctx context.Context) (ClipboardImageRaw, error) {
	ps, err := exec.LookPath("powershell")
	if err != nil {
		// Windows 上通常是 powershell.exe；LookPath("powershell") 失败时再试一次。
		ps, err = exec.LookPath("powershell.exe")
		if err != nil {
			// PowerShell 7
			ps, err = exec.LookPath("pwsh")
			if err != nil {
				ps, err = exec.LookPath("pwsh.exe")
			}
			if err != nil {
				return ClipboardImageRaw{}, errs.E("E003", "缺少 PowerShell", errs.Hint("请确认系统 PowerShell 可用（powershell.exe 或 pwsh.exe）"), errs.Cause(err))
			}
		}
	}

	outFile, err := os.CreateTemp("", "clip2agent-raw-*.png")
	if err != nil {
		return ClipboardImageRaw{}, errs.WrapCode("E007", "创建临时文件失败", err)
	}
	path := outFile.Name()
	_ = outFile.Close()
	defer os.Remove(path)

	// 读取图片并保存为 PNG。
	// 退出码：2 表示剪贴板无图片；其它非 0 表示执行失败。
	script := strings.Join([]string{
		"$ErrorActionPreference='Stop'",
		"Add-Type -AssemblyName System.Windows.Forms",
		"Add-Type -AssemblyName System.Drawing",
		"$img=[System.Windows.Forms.Clipboard]::GetImage()",
		"if ($null -eq $img) { exit 2 }",
		"$img.Save('" + psEscapeSingleQuoted(path) + "',[System.Drawing.Imaging.ImageFormat]::Png)",
		"exit 0",
	}, ";")

	cmd := exec.CommandContext(ctx, ps, "-NoProfile", "-NonInteractive", "-Sta", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// 根据退出码给出更清晰的错误。
		if ee := exitCode(err); ee == 2 {
			return ClipboardImageRaw{}, errs.E("E002", "剪贴板中无图片")
		}
		hint := strings.TrimSpace(string(out))
		if hint != "" {
			return ClipboardImageRaw{}, errs.E("E003", "PowerShell 读取剪贴板失败", errs.Hint(hint), errs.Cause(err))
		}
		return ClipboardImageRaw{}, errs.WrapCode("E003", "PowerShell 读取剪贴板失败", err)
	}

	b, rerr := os.ReadFile(path)
	if rerr != nil {
		return ClipboardImageRaw{}, errs.WrapCode("E004", "读取剪贴板导出文件失败", rerr)
	}
	if len(b) == 0 {
		return ClipboardImageRaw{}, errs.E("E002", "剪贴板中无图片")
	}

	return ClipboardImageRaw{
		SourcePlatform: runtime.GOOS,
		SourceTool:     "powershell",
		MimeType:       "image/png",
		RawBytes:       b,
		AcquiredAt:     time.Now(),
	}, nil
}

func psEscapeSingleQuoted(s string) string {
	// PowerShell 单引号字符串内用 '' 表示一个 '。
	return strings.ReplaceAll(s, "'", "''")
}

func exitCode(err error) int {
	var ee *exec.ExitError
	if errorsAs(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}
