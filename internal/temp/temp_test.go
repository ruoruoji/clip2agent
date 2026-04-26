package temp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ruoruoji/clip2agent/internal/imagex"
)

func TestGC_RemovesOldFiles(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(Options{BaseDir: dir})

	old := filepath.Join(dir, "old.png")
	newf := filepath.Join(dir, "new.png")
	if err := os.WriteFile(old, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newf, []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	_ = os.Chtimes(old, now.Add(-2*time.Hour), now.Add(-2*time.Hour))
	_ = os.Chtimes(newf, now, now)

	if err := m.GC(context.Background(), time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old should be removed, stat err=%v", err)
	}
	if _, err := os.Stat(newf); err != nil {
		t.Fatalf("new should exist, stat err=%v", err)
	}
}

func TestWrite_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(Options{BaseDir: dir})
	img := imagex.NormalizedImage{MimeType: "image/png", Bytes: []byte{1, 2, 3}, Sha256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	path, err := m.Write(context.Background(), img)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat err=%v", err)
	}
}
