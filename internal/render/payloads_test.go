package render

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ruoruoji/clip2agent/internal/imagex"
)

func TestBase64Payload_JSONSchema(t *testing.T) {
	img := imagex.NormalizedImage{MimeType: "image/png", Bytes: []byte{1, 2, 3}, Width: 10, Height: 20, SizeBytes: 3, Sha256: "abc", StrippedMetadata: true}
	p := NewBase64Payload(img)
	var got Base64JSON
	if err := json.Unmarshal([]byte(p.Text), &got); err != nil {
		t.Fatalf("unmarshal err=%v, text=%s", err, p.Text)
	}
	if got.MimeType != "image/png" || got.Encoding != "base64" {
		t.Fatalf("got=%+v", got)
	}
	if got.SchemaVersion != 1 {
		t.Fatalf("schema_version=%d", got.SchemaVersion)
	}
	if got.Width != 10 || got.Height != 20 {
		t.Fatalf("size got=%dx%d", got.Width, got.Height)
	}
	if got.DataBase64 == "" {
		t.Fatalf("expected data_base64")
	}
}

func TestDataURIPayload_DataURI(t *testing.T) {
	img := imagex.NormalizedImage{MimeType: "image/png", Bytes: []byte{1, 2, 3}, Width: 10, Height: 20, SizeBytes: 3, Sha256: "abc", StrippedMetadata: true}
	p := NewDataURIPayload(img)
	if !strings.HasPrefix(p.DataURI, "data:image/png;base64,") {
		t.Fatalf("unexpected prefix: %s", p.DataURI)
	}
	parts := strings.SplitN(p.DataURI, ",", 2)
	if len(parts) != 2 {
		t.Fatalf("unexpected data-uri: %s", p.DataURI)
	}
	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode base64 err=%v", err)
	}
	if string(decoded) != string(img.Bytes) {
		t.Fatalf("decoded mismatch: got=%v want=%v", decoded, img.Bytes)
	}
	if p.JSON.MimeType != "image/png" || p.JSON.DataURI == "" {
		t.Fatalf("json mismatch: %+v", p.JSON)
	}
	if p.JSON.SchemaVersion != 1 {
		t.Fatalf("schema_version=%d", p.JSON.SchemaVersion)
	}
}
