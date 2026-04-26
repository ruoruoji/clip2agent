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

type linuxProvider struct{}

func (p *linuxProvider) GetImage(ctx context.Context) (ClipboardImageRaw, error) {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return getWayland(ctx)
	}
	if os.Getenv("DISPLAY") != "" {
		return getX11(ctx)
	}
	// 没有明确会话，仍尝试 wl-paste/xclip
	if _, err := exec.LookPath("wl-paste"); err == nil {
		return getWayland(ctx)
	}
	if _, err := exec.LookPath("xclip"); err == nil {
		return getX11(ctx)
	}
	if _, err := exec.LookPath("xsel"); err == nil {
		return getX11(ctx)
	}
	return ClipboardImageRaw{}, errs.E("E003", "缺少 Linux 剪贴板工具", errs.Hint("Wayland: 安装 wl-clipboard；X11: 安装 xclip 或 xsel"))
}

func getWayland(ctx context.Context) (ClipboardImageRaw, error) {
	if _, err := exec.LookPath("wl-paste"); err != nil {
		return ClipboardImageRaw{}, errs.E("E003", "缺少 wl-paste", errs.Hint("安装 wl-clipboard（如: sudo apt install wl-clipboard）"))
	}
	listCmd := exec.CommandContext(ctx, "wl-paste", "--list-types")
	out, err := listCmd.Output()
	if err != nil {
		return ClipboardImageRaw{}, errs.WrapCode("E002", "无法读取剪贴板类型列表", err)
	}
	types := splitLines(string(out))
	mime := pickImageMime(types)
	if mime == "" {
		if len(types) == 0 {
			return ClipboardImageRaw{}, errs.E("E001", "剪贴板为空")
		}
		return ClipboardImageRaw{}, errs.E("E002", "剪贴板中无图片")
	}

	cmd := exec.CommandContext(ctx, "wl-paste", "--type", mime)
	bytes, err := cmd.Output()
	if err != nil {
		return ClipboardImageRaw{}, errs.WrapCode("E002", "拉取剪贴板图片失败", err)
	}
	if len(bytes) == 0 {
		return ClipboardImageRaw{}, errs.E("E002", "剪贴板中无图片")
	}
	return ClipboardImageRaw{SourcePlatform: runtime.GOOS, SourceTool: "wl-paste", MimeType: mime, RawBytes: bytes, AcquiredAt: time.Now()}, nil
}

func getX11(ctx context.Context) (ClipboardImageRaw, error) {
	tool := ""
	if _, err := exec.LookPath("xclip"); err == nil {
		tool = "xclip"
	} else if _, err := exec.LookPath("xsel"); err == nil {
		tool = "xsel"
	}
	if tool == "" {
		return ClipboardImageRaw{}, errs.E("E003", "缺少 xclip/xsel", errs.Hint("安装 xclip（如: sudo apt install xclip）"))
	}

	types, err := listX11Types(ctx, tool)
	if err != nil {
		return ClipboardImageRaw{}, err
	}
	mime := pickImageMime(types)
	if mime == "" {
		if len(types) == 0 {
			return ClipboardImageRaw{}, errs.E("E001", "剪贴板为空")
		}
		return ClipboardImageRaw{}, errs.E("E002", "剪贴板中无图片")
	}

	bytes, err := readX11(ctx, tool, mime)
	if err != nil {
		return ClipboardImageRaw{}, err
	}
	if len(bytes) == 0 {
		return ClipboardImageRaw{}, errs.E("E002", "剪贴板中无图片")
	}
	return ClipboardImageRaw{SourcePlatform: runtime.GOOS, SourceTool: tool, MimeType: mime, RawBytes: bytes, AcquiredAt: time.Now()}, nil
}

func listX11Types(ctx context.Context, tool string) ([]string, error) {
	switch tool {
	case "xclip":
		cmd := exec.CommandContext(ctx, "xclip", "-selection", "clipboard", "-t", "TARGETS", "-o")
		out, err := cmd.Output()
		if err != nil {
			return nil, errs.WrapCode("E002", "无法读取剪贴板类型列表", err)
		}
		return splitLines(string(out)), nil
	case "xsel":
		// xsel 没有 TARGETS；退化为尝试若干常见类型
		return []string{"image/png", "image/jpeg", "image/webp"}, nil
	default:
		return nil, errs.E("E003", "未知 X11 工具")
	}
}

func readX11(ctx context.Context, tool, mime string) ([]byte, error) {
	switch tool {
	case "xclip":
		cmd := exec.CommandContext(ctx, "xclip", "-selection", "clipboard", "-t", mime, "-o")
		out, err := cmd.Output()
		if err != nil {
			return nil, errs.WrapCode("E002", "拉取剪贴板图片失败", err)
		}
		return out, nil
	case "xsel":
		// xsel 不支持 mime 选择；直接输出并交给解码层判断
		cmd := exec.CommandContext(ctx, "xsel", "--clipboard", "--output")
		out, err := cmd.Output()
		if err != nil {
			return nil, errs.WrapCode("E002", "拉取剪贴板内容失败", err)
		}
		return out, nil
	default:
		return nil, errs.E("E003", "未知 X11 工具")
	}
}

func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func pickImageMime(types []string) string {
	prefer := []string{"image/png", "image/jpeg", "image/webp", "image/tiff", "image/bmp"}
	set := map[string]struct{}{}
	for _, t := range types {
		set[strings.ToLower(t)] = struct{}{}
	}
	for _, p := range prefer {
		if _, ok := set[p]; ok {
			return p
		}
	}
	return ""
}
