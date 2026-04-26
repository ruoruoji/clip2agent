package clipboard

import (
	"context"

	"github.com/ruoruoji/clip2agent/internal/errs"
)

type unsupportedProvider struct{ goos string }

func (p *unsupportedProvider) GetImage(_ context.Context) (ClipboardImageRaw, error) {
	return ClipboardImageRaw{}, errs.E("E006", "当前平台暂不支持", errs.Hint("仅支持 macOS/Linux/Windows"))
}
