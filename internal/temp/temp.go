package temp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ruoruoji/clip2agent/internal/imagex"
)

type Options struct {
	BaseDir string
}

type Manager struct {
	opts Options
}

func NewManager(opts Options) *Manager {
	return &Manager{opts: opts}
}

func (m *Manager) baseDir() string {
	if strings.TrimSpace(m.opts.BaseDir) != "" {
		return m.opts.BaseDir
	}
	return filepath.Join(os.TempDir(), "clip2agent")
}

func (m *Manager) Write(_ context.Context, img imagex.NormalizedImage) (string, error) {
	dir := m.baseDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s_%s%s", time.Now().UTC().Format("20060102-150405"), img.Sha256[:12], extByMime(img.MimeType))
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, img.Bytes, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func (m *Manager) GC(_ context.Context, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	dir := m.baseDir()
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	cutoff := time.Now().Add(-ttl)
	for _, ent := range ents {
		if ent.IsDir() {
			continue
		}
		info, err := ent.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, ent.Name()))
		}
	}
	return nil
}

func extByMime(mime string) string {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	default:
		return ".bin"
	}
}
