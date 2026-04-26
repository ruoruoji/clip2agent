package clipboard

import "time"

type ClipboardImageRaw struct {
	SourcePlatform string
	SourceTool     string
	MimeType       string
	RawBytes       []byte
	AcquiredAt     time.Time
}
