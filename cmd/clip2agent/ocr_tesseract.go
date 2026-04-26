package main

import (
	"context"
	"os/exec"
	"strings"

	"github.com/ruoruoji/clip2agent/internal/errs"
)

func runTesseractOCR(ctx context.Context, imagePath string, lang string) (string, error) {
	bin, err := exec.LookPath("tesseract")
	if err != nil {
		return "", errs.E("E003", "缺少 tesseract（OCR 可选依赖）", errs.Hint("macOS: brew install tesseract；Ubuntu: sudo apt install tesseract-ocr"), errs.Cause(err))
	}
	lang = strings.TrimSpace(lang)
	argv := []string{imagePath, "stdout"}
	if lang != "" {
		argv = append(argv, "-l", lang)
	}
	cmd := exec.CommandContext(ctx, bin, argv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		h := strings.TrimSpace(string(out))
		if h != "" {
			return "", errs.E("E003", "tesseract 执行失败", errs.Hint(h), errs.Cause(err))
		}
		return "", errs.WrapCode("E003", "tesseract 执行失败", err)
	}
	return string(out), nil
}
