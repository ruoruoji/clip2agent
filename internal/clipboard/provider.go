package clipboard

import (
	"context"
	"runtime"
)

type Provider interface {
	GetImage(ctx context.Context) (ClipboardImageRaw, error)
}

type Selector struct{}

func NewSelector() *Selector { return &Selector{} }

func (s *Selector) Select() Provider {
	switch runtime.GOOS {
	case "darwin":
		return &macOSProvider{}
	case "linux":
		return &linuxProvider{}
	case "windows":
		return &windowsProvider{}
	default:
		return &unsupportedProvider{goos: runtime.GOOS}
	}
}
