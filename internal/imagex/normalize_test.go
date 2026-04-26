package imagex

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/ruoruoji/clip2agent/internal/clipboard"
	"github.com/ruoruoji/clip2agent/internal/errs"
)

func TestNormalize_ResizeAndHash(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}

	raw := clipboard.ClipboardImageRaw{MimeType: "image/png", RawBytes: buf.Bytes(), SourceTool: "test"}
	out, err := Normalize(raw, NormalizeOptions{Format: "png", MaxWidth: 2, MaxHeight: 0, MaxBytes: 0})
	if err != nil {
		t.Fatal(err)
	}
	if out.Width != 2 || out.Height != 1 {
		t.Fatalf("got size %dx%d, want 2x1", out.Width, out.Height)
	}
	if out.MimeType != "image/png" {
		t.Fatalf("mime=%q", out.MimeType)
	}
	if len(out.Sha256) != 64 {
		t.Fatalf("sha256 len=%d", len(out.Sha256))
	}
	if !out.StrippedMetadata {
		t.Fatalf("expected stripped_metadata=true")
	}
	if out.SizeBytes <= 0 {
		t.Fatalf("expected size_bytes > 0")
	}
}

func TestNormalize_MaxBytesTooSmall(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)

	raw := clipboard.ClipboardImageRaw{MimeType: "image/png", RawBytes: buf.Bytes(), SourceTool: "test"}
	_, err := Normalize(raw, NormalizeOptions{Format: "png", MaxWidth: 0, MaxHeight: 0, MaxBytes: 1})
	if err == nil {
		t.Fatalf("expected error")
	}
	if errs.CodeOf(err) != "E005" {
		t.Fatalf("code=%q, err=%v", errs.CodeOf(err), err)
	}
}

func TestNormalize_NoStripMetadataFastPath(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.RGBA{R: 1, G: 2, B: 3, A: 255})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}

	rawBytes := buf.Bytes()
	raw := clipboard.ClipboardImageRaw{MimeType: "image/png", RawBytes: rawBytes, SourceTool: "test"}
	out, err := Normalize(raw, NormalizeOptions{Format: "keep", MaxWidth: 0, MaxHeight: 0, MaxBytes: 0, NoStripMetadata: true})
	if err != nil {
		t.Fatal(err)
	}
	if out.StrippedMetadata {
		t.Fatalf("expected stripped_metadata=false")
	}
	if out.MimeType != "image/png" {
		t.Fatalf("mime=%q", out.MimeType)
	}
	if out.Width != 2 || out.Height != 1 {
		t.Fatalf("got size %dx%d, want 2x1", out.Width, out.Height)
	}
	if !bytes.Equal(out.Bytes, rawBytes) {
		t.Fatalf("expected raw bytes passthrough")
	}
}
