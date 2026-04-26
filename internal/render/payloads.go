package render

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/ruoruoji/clip2agent/internal/imagex"
)

type PathPayload struct {
	Mode         string
	Text         string
	TempFilePath string
	Image        imagex.NormalizedImage
}

func NewPathPayload(img imagex.NormalizedImage, path string) PathPayload {
	return PathPayload{Mode: "path", Text: path, TempFilePath: path, Image: img}
}

type Base64JSON struct {
	SchemaVersion    int    `json:"schema_version"`
	Source           string `json:"source"`
	MimeType         string `json:"mime_type"`
	Encoding         string `json:"encoding"`
	Filename         string `json:"filename"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	SizeBytes        int64  `json:"size_bytes"`
	Sha256           string `json:"sha256"`
	StrippedMetadata bool   `json:"stripped_metadata"`
	DataBase64       string `json:"data_base64"`
}

type Base64Payload struct {
	Mode string
	JSON Base64JSON
	Text string
}

func NewBase64Payload(img imagex.NormalizedImage) Base64Payload {
	data := base64.StdEncoding.EncodeToString(img.Bytes)
	js := Base64JSON{
		SchemaVersion:    1,
		Source:           "clipboard",
		MimeType:         img.MimeType,
		Encoding:         "base64",
		Filename:         fmt.Sprintf("clipboard%s", extFromMime(img.MimeType)),
		Width:            img.Width,
		Height:           img.Height,
		SizeBytes:        img.SizeBytes,
		Sha256:           img.Sha256,
		StrippedMetadata: img.StrippedMetadata,
		DataBase64:       data,
	}
	b, _ := json.MarshalIndent(js, "", "  ")
	return Base64Payload{Mode: "base64", JSON: js, Text: string(b)}
}

type DataURIJSON struct {
	SchemaVersion    int    `json:"schema_version"`
	Source           string `json:"source"`
	MimeType         string `json:"mime_type"`
	Encoding         string `json:"encoding"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	SizeBytes        int64  `json:"size_bytes"`
	Sha256           string `json:"sha256"`
	StrippedMetadata bool   `json:"stripped_metadata"`
	DataURI          string `json:"data_uri"`
}

type DataURIPayload struct {
	Mode    string
	DataURI string
	JSON    DataURIJSON
}

func NewDataURIPayload(img imagex.NormalizedImage) DataURIPayload {
	b64 := base64.StdEncoding.EncodeToString(img.Bytes)
	uri := fmt.Sprintf("data:%s;base64,%s", img.MimeType, b64)
	js := DataURIJSON{
		SchemaVersion:    1,
		Source:           "clipboard",
		MimeType:         img.MimeType,
		Encoding:         "base64",
		Width:            img.Width,
		Height:           img.Height,
		SizeBytes:        img.SizeBytes,
		Sha256:           img.Sha256,
		StrippedMetadata: img.StrippedMetadata,
		DataURI:          uri,
	}
	return DataURIPayload{Mode: "data-uri", DataURI: uri, JSON: js}
}

func extFromMime(m string) string {
	switch m {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	default:
		return ".bin"
	}
}
