package adapter

import (
	"strings"

	"github.com/ruoruoji/clip2agent/internal/errs"
	"github.com/ruoruoji/clip2agent/internal/render"
)

type Options struct {
	OnlyPayload bool
	JSON        bool
	CocoAtPath  bool
}

type Adapter interface {
	AdaptPath(p render.PathPayload, opt Options) (string, error)
	AdaptBase64(p render.Base64Payload, opt Options) (string, error)
}

func ByTarget(target string) Adapter {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "", "coco":
		return cocoAdapter{}
	case "openai":
		return openAIAdapter{}
	case "generic":
		return genericAdapter{}
	default:
		return nil
	}
}

type cocoAdapter struct{}

func (cocoAdapter) AdaptPath(p render.PathPayload, opt Options) (string, error) {
	ref := p.TempFilePath
	if opt.CocoAtPath {
		ref = "@" + ref
	}
	if opt.OnlyPayload {
		return ref, nil
	}
	// 默认只输出图片引用（不附带固定提示词）
	return ref, nil
}

func (cocoAdapter) AdaptBase64(_ render.Base64Payload, _ Options) (string, error) {
	return "", errs.E("E006", "coco 不支持 base64 输出模式", errs.Hint("请使用: clip2agent path --target coco"))
}

type openAIAdapter struct{}

func (openAIAdapter) AdaptPath(_ render.PathPayload, _ Options) (string, error) {
	return "", errs.E("E006", "openai 适配器不支持 path 输出", errs.Hint("请使用: clip2agent base64 --target openai"))
}

func (openAIAdapter) AdaptBase64(p render.Base64Payload, _ Options) (string, error) {
	return p.Text, nil
}

type genericAdapter struct{}

func (genericAdapter) AdaptPath(p render.PathPayload, _ Options) (string, error) {
	return p.TempFilePath, nil
}

func (genericAdapter) AdaptBase64(p render.Base64Payload, _ Options) (string, error) {
	return p.Text, nil
}
