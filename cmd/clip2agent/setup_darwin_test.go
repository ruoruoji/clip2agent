//go:build darwin

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFileSkipsSelfCopy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	p := filepath.Join(dir, "clip2agent")
	want := []byte("hello")
	if err := os.WriteFile(p, want, 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	if err := copyFile(p, p, 0o755); err != nil {
		t.Fatalf("copy self: %v", err)
	}

	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read after self copy: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("content changed after self copy: got %q want %q", got, want)
	}
}

func TestCopyFileSkipsSymlinkToSameFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "clip2agent")
	link := filepath.Join(dir, "clip2agent-link")
	want := []byte("hello")
	if err := os.WriteFile(src, want, 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := os.Symlink(src, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	if err := copyFile(src, link, 0o755); err != nil {
		t.Fatalf("copy to symlink target: %v", err)
	}

	got, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read after symlink copy: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("content changed after symlink self copy: got %q want %q", got, want)
	}
}
