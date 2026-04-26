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
	// 使用 plan.md 里的排障模板
	tpl := "请分析这张截图中的报错信息，提取关键异常、定位可能原因，并给出修复步骤：\n{image_ref}"
	out := strings.ReplaceAll(tpl, "{image_ref}", ref)
	return out, nil
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
