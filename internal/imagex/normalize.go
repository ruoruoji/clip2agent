package imagex

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/jpeg"
	"image/png"
	"strings"

	"github.com/ruoruoji/clip2agent/internal/clipboard"
	"github.com/ruoruoji/clip2agent/internal/errs"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

type NormalizeOptions struct {
	Format          string // keep|png|jpeg|webp
	MaxWidth        int
	MaxHeight       int
	MaxBytes        int64 // 0 表示不限制
	NoStripMetadata bool  // 尽可能保留原始字节（仅在无需 resize/压缩/转码时生效）
}

type NormalizedImage struct {
	MimeType          string `json:"mime_type"`
	Bytes             []byte `json:"-"`
	Width             int    `json:"width"`
	Height            int    `json:"height"`
	SizeBytes         int64  `json:"size_bytes"`
	Sha256            string `json:"sha256"`
	StrippedMetadata  bool   `json:"stripped_metadata"`
	Source            string `json:"source"`
	SourceTool        string `json:"source_tool"`
	OriginalMimeType  string `json:"original_mime_type"`
	OriginalSizeBytes int64  `json:"original_size_bytes"`
}

func Normalize(raw clipboard.ClipboardImageRaw, opt NormalizeOptions) (NormalizedImage, error) {
	if len(raw.RawBytes) == 0 {
		return NormalizedImage{}, errs.E("E002", "剪贴板中无图片")
	}

	// 快路径：在可行时直接复用原始字节，避免 decode+re-encode（从而尽可能保留元数据）。
	if opt.NoStripMetadata {
		outFormat := strings.ToLower(strings.TrimSpace(opt.Format))
		if outFormat == "" {
			outFormat = "keep"
		}
		if outFormat == "keep" {
			mime := strings.ToLower(strings.TrimSpace(raw.MimeType))
			if mime == "image/png" || mime == "image/jpeg" {
				cfg, format, err := image.DecodeConfig(bytes.NewReader(raw.RawBytes))
				if err == nil {
					w, hh := cfg.Width, cfg.Height
					needResize := (opt.MaxWidth > 0 && w > opt.MaxWidth) || (opt.MaxHeight > 0 && hh > opt.MaxHeight)
					needCompress := opt.MaxBytes > 0 && int64(len(raw.RawBytes)) > opt.MaxBytes
					// 确保“keep”不会触发格式变化。
					fmtLower := strings.ToLower(strings.TrimSpace(format))
					fmtOK := (mime == "image/png" && fmtLower == "png") || (mime == "image/jpeg" && (fmtLower == "jpeg" || fmtLower == "jpg"))
					if !needResize && !needCompress && fmtOK {
						hash := sha256.Sum256(raw.RawBytes)
						sha := hex.EncodeToString(hash[:])
						return NormalizedImage{
							MimeType:          mime,
							Bytes:             raw.RawBytes,
							Width:             w,
							Height:            hh,
							SizeBytes:         int64(len(raw.RawBytes)),
							Sha256:            sha,
							StrippedMetadata:  false,
							Source:            "clipboard",
							SourceTool:        raw.SourceTool,
							OriginalMimeType:  raw.MimeType,
							OriginalSizeBytes: int64(len(raw.RawBytes)),
						}, nil
					}
				}
			}
		}
	}

	img, format, err := image.Decode(bytes.NewReader(raw.RawBytes))
	if err != nil {
		return NormalizedImage{}, errs.WrapCode("E004", "图片格式无法解码", err)
	}
	img = resizeIfNeeded(img, opt.MaxWidth, opt.MaxHeight)
	b2 := img.Bounds()
	outW, outH := b2.Dx(), b2.Dy()

	outFormat := strings.ToLower(strings.TrimSpace(opt.Format))
	if outFormat == "" {
		outFormat = "keep"
	}
	if outFormat == "keep" {
		// keep：尽量保持输入主格式，但仍重编码以去元数据。
		switch strings.ToLower(format) {
		case "jpeg", "jpg":
			outFormat = "jpeg"
		case "png":
			outFormat = "png"
		default:
			outFormat = "png"
		}
	}

	// MVP：不支持输出 webp（但支持输入 webp -> png/jpeg）。
	if outFormat == "webp" {
		return NormalizedImage{}, errs.E("E006", "暂不支持输出 WebP", errs.Hint("请使用 --format png 或 --format jpeg"))
	}

	encoded, mime, err := encode(img, outFormat, opt.MaxBytes)
	if err != nil {
		return NormalizedImage{}, err
	}

	h := sha256.Sum256(encoded)
	sha := hex.EncodeToString(h[:])

	return NormalizedImage{
		MimeType:          mime,
		Bytes:             encoded,
		Width:             outW,
		Height:            outH,
		SizeBytes:         int64(len(encoded)),
		Sha256:            sha,
		StrippedMetadata:  true,
		Source:            "clipboard",
		SourceTool:        raw.SourceTool,
		OriginalMimeType:  raw.MimeType,
		OriginalSizeBytes: int64(len(raw.RawBytes)),
	}, nil
}

func resizeIfNeeded(src image.Image, maxW, maxH int) image.Image {
	if maxW <= 0 {
		maxW = 0
	}
	if maxH <= 0 {
		maxH = 0
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if (maxW == 0 || w <= maxW) && (maxH == 0 || h <= maxH) {
		return src
	}

	// 计算等比缩放
	scaleW := float64(w)
	scaleH := float64(h)
	if maxW > 0 && w > maxW {
		scaleW = float64(maxW)
	}
	if maxH > 0 && h > maxH {
		scaleH = float64(maxH)
	}
	scale := min(scaleW/float64(w), scaleH/float64(h))
	nw := int(float64(w)*scale + 0.5)
	nh := int(float64(h)*scale + 0.5)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func encode(img image.Image, format string, maxBytes int64) ([]byte, string, error) {
	// 先按目标格式编码一遍
	encodeOnce := func(fmt string, quality int) ([]byte, string, error) {
		var buf bytes.Buffer
		switch fmt {
		case "png":
			if err := png.Encode(&buf, img); err != nil {
				return nil, "", errs.WrapCode("E004", "PNG 编码失败", err)
			}
			return buf.Bytes(), "image/png", nil
		case "jpeg", "jpg":
			q := quality
			if q <= 0 {
				q = 85
			}
			if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
				return nil, "", errs.WrapCode("E004", "JPEG 编码失败", err)
			}
			return buf.Bytes(), "image/jpeg", nil
		default:
			return nil, "", errs.E("E006", "不支持的输出格式", errs.Hint("--format keep|png|jpeg"))
		}
	}

	out, mime, err := encodeOnce(format, 0)
	if err != nil {
		return nil, "", err
	}
	if maxBytes <= 0 || int64(len(out)) <= maxBytes {
		return out, mime, nil
	}

	// 超限：仅对非强制 PNG 的情况做 JPEG 质量递减尝试
	if format == "png" {
		return nil, "", errs.E("E005", "图片过大且压缩失败", errs.Hint("尝试减小 --max-width/--max-height 或改用 --format jpeg"))
	}

	for q := 90; q >= 40; q -= 10 {
		out2, mime2, err := encodeOnce("jpeg", q)
		if err != nil {
			return nil, "", err
		}
		if int64(len(out2)) <= maxBytes {
			return out2, mime2, nil
		}
	}
	return nil, "", errs.E("E005", "图片过大且压缩失败", errs.Hint("尝试降低分辨率或提高 max-bytes"))
}
