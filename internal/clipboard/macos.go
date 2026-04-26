package clipboard

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ruoruoji/clip2agent/internal/errs"
)

type macOSProvider struct{}

type macosHelperOK struct {
	MimeType   string `json:"mime_type"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	SourceTool string `json:"source_tool"`
}

type macosHelperErr struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (p *macOSProvider) GetImage(ctx context.Context) (ClipboardImageRaw, error) {
	helper, err := findMacOSHelper()
	if err != nil {
		return ClipboardImageRaw{}, err
	}

	// helper 只负责“从剪贴板导出为 PNG”，后续标准化在 Go 侧统一做。
	outFile, err := os.CreateTemp("", "clip2agent-raw-*.png")
	if err != nil {
		return ClipboardImageRaw{}, errs.WrapCode("E007", "创建临时文件失败", err)
	}
	path := outFile.Name()
	_ = outFile.Close()
	defer os.Remove(path)

	cmd := exec.CommandContext(ctx, helper, "--out", path)
	stdout, err := cmd.Output()
	if err != nil {
		// 优先解析 helper 的 stderr JSON
		if ee := parseHelperError(err); ee != nil {
			return ClipboardImageRaw{}, ee
		}
		return ClipboardImageRaw{}, errs.WrapCode("E003", "macOS helper 执行失败", err)
	}

	var ok macosHelperOK
	if jerr := json.Unmarshal(bytesTrim(stdout), &ok); jerr != nil {
		return ClipboardImageRaw{}, errs.WrapCode("E003", "macOS helper 输出解析失败", jerr)
	}
	if ok.MimeType == "" {
		ok.MimeType = "image/png"
	}
	if ok.SourceTool == "" {
		ok.SourceTool = "nspasteboard"
	}

	b, rerr := os.ReadFile(path)
	if rerr != nil {
		return ClipboardImageRaw{}, errs.WrapCode("E004", "读取剪贴板导出文件失败", rerr)
	}

	return ClipboardImageRaw{
		SourcePlatform: runtime.GOOS,
		SourceTool:     ok.SourceTool,
		MimeType:       ok.MimeType,
		RawBytes:       b,
		AcquiredAt:     time.Now(),
	}, nil
}

func findMacOSHelper() (string, error) {
	if v := strings.TrimSpace(os.Getenv("CLIP2AGENT_MACOS_HELPER")); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v, nil
		} else {
			return "", errs.WrapCode("E003", "CLIP2AGENT_MACOS_HELPER 指向的文件不存在", err)
		}
	}

	// 1) 与主程序同目录
	if exe, err := os.Executable(); err == nil {
		cand := filepath.Join(filepath.Dir(exe), "clip2agent-macos")
		if _, err := os.Stat(cand); err == nil {
			return cand, nil
		}
	}

	// 2) PATH
	if p, err := exec.LookPath("clip2agent-macos"); err == nil {
		return p, nil
	}

	return "", errs.E("E003", "缺少 macOS 剪贴板 helper（clip2agent-macos）", errs.Hint("在 native/macos 下构建后设置 CLIP2AGENT_MACOS_HELPER 或把 clip2agent-macos 放到 PATH/同目录"))
}

func parseHelperError(err error) error {
	var ee *exec.ExitError
	if !errorsAs(err, &ee) {
		return nil
	}
	if len(ee.Stderr) == 0 {
		return nil
	}
	var herr macosHelperErr
	if jerr := json.Unmarshal(bytesTrim(ee.Stderr), &herr); jerr != nil {
		return nil
	}
	if herr.Code == "" {
		return nil
	}
	switch herr.Code {
	case "E001", "E002":
		return errs.E(errs.Code(herr.Code), herr.Message)
	default:
		return errs.E("E003", fmt.Sprintf("macOS helper 错误: %s", herr.Message))
	}
}

func bytesTrim(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

// 避免在此文件引入 errors 包导致循环：小封装。
func errorsAs(err error, target any) bool {
	// 这里单独实现会很啰嗦；直接在本包其它文件引入 errors 会更简单。
	// 但为避免多文件拆分，这里用标准库 errors.As。
	return errorsAsStd(err, target)
}
