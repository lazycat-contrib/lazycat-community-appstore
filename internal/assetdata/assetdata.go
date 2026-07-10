package assetdata

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

type Payload struct {
	MediaType string
	Data      []byte
}

var (
	ErrNotDataURL       = errors.New("not a data URL")
	ErrUnsupportedImage = errors.New("unsupported image type")
	ErrTooLarge         = errors.New("image is too large")
	ErrTooManyPixels    = errors.New("image dimensions are too large")
)

func ParseDataURL(value string, maxBytes int64) (Payload, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "data:") {
		return Payload{}, ErrNotDataURL
	}
	header, encoded, ok := strings.Cut(value, ",")
	if !ok {
		return Payload{}, fmt.Errorf("invalid data URL")
	}
	if maxBytes <= 0 {
		return Payload{}, fmt.Errorf("max image size must be positive")
	}
	if int64(len(encoded)) > maxBytes*2 {
		return Payload{}, ErrTooLarge
	}
	mediaType, ok := parseBase64DataHeader(header)
	if !ok {
		return Payload{}, fmt.Errorf("data URL must use base64 image data")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return Payload{}, fmt.Errorf("decode data URL: %w", err)
	}
	return NewImagePayload(raw, mediaType, maxBytes)
}

func NewImagePayload(data []byte, mediaType string, maxBytes int64) (Payload, error) {
	if maxBytes <= 0 {
		return Payload{}, fmt.Errorf("max image size must be positive")
	}
	if len(data) == 0 {
		return Payload{}, fmt.Errorf("image is empty")
	}
	if int64(len(data)) > maxBytes {
		return Payload{}, ErrTooLarge
	}
	declared := normalizeMediaType(mediaType)
	if declared != "" && !supportedImageType(declared) {
		return Payload{}, ErrUnsupportedImage
	}
	detected := detectMediaType(data)
	if !supportedImageType(detected) {
		return Payload{}, ErrUnsupportedImage
	}
	if declared != "" && declared != detected {
		return Payload{}, fmt.Errorf("image content type does not match declared media type")
	}
	return Payload{MediaType: detected, Data: append([]byte(nil), data...)}, nil
}

func NormalizeImage(data []byte, mediaType string, maxBytes int64, maxSide int) (Payload, error) {
	payload, err := NewImagePayload(data, mediaType, maxBytes)
	if err != nil {
		return Payload{}, err
	}
	if maxSide <= 0 {
		return Payload{}, fmt.Errorf("max image side must be positive")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(payload.Data))
	if err != nil {
		return Payload{}, fmt.Errorf("decode image config: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return Payload{}, fmt.Errorf("invalid image dimensions")
	}
	if cfg.Width > 4096 || cfg.Height > 4096 || int64(cfg.Width)*int64(cfg.Height) > 16*1024*1024 {
		return Payload{}, ErrTooManyPixels
	}
	img, _, err := image.Decode(bytes.NewReader(payload.Data))
	if err != nil {
		return Payload{}, fmt.Errorf("decode image: %w", err)
	}
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return Payload{}, fmt.Errorf("invalid image dimensions")
	}
	targetWidth, targetHeight := width, height
	if width > maxSide || height > maxSide {
		if width >= height {
			targetWidth = maxSide
			targetHeight = max(1, height*maxSide/width)
		} else {
			targetHeight = maxSide
			targetWidth = max(1, width*maxSide/height)
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)
	var out bytes.Buffer
	if err := png.Encode(&out, dst); err != nil {
		return Payload{}, fmt.Errorf("encode png: %w", err)
	}
	if int64(out.Len()) > maxBytes {
		return Payload{}, ErrTooLarge
	}
	return Payload{MediaType: "image/png", Data: out.Bytes()}, nil
}

func ServeImage(w http.ResponseWriter, r *http.Request, mediaType, sha256 string, data []byte) {
	if ServeImageMetadata(w, r, mediaType, sha256, int64(len(data))) {
		return
	}
	_, _ = w.Write(data)
}

func ServeImageMetadata(w http.ResponseWriter, r *http.Request, mediaType, sha256 string, size int64) bool {
	etag := ETag(sha256)
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	if etag != "" {
		w.Header().Set("ETag", etag)
	}
	if etag != "" && headerHasETag(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return true
	}
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	return r.Method == http.MethodHead
}

func ETag(sha256 string) string {
	sha256 = strings.TrimSpace(strings.ToLower(sha256))
	if sha256 == "" {
		return ""
	}
	return `"sha256-` + sha256 + `"`
}

func AssetIDFromURL(prefix, value string) (int, bool) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		idx := strings.Index(value, prefix)
		if idx < 0 {
			return 0, false
		}
		value = value[idx:]
	}
	if !strings.HasPrefix(value, prefix) {
		return 0, false
	}
	rest := strings.TrimPrefix(value, prefix)
	rest = strings.Trim(rest, "/")
	if rest == "" || strings.Contains(rest, "/") {
		return 0, false
	}
	id, err := strconv.Atoi(rest)
	return id, err == nil && id > 0
}

func URL(prefix string, id int) string {
	if id <= 0 {
		return ""
	}
	return strings.TrimRight(prefix, "/") + "/" + strconv.Itoa(id)
}

func parseBase64DataHeader(header string) (string, bool) {
	raw := strings.TrimPrefix(strings.TrimSpace(header), "data:")
	if raw == "" {
		raw = "text/plain"
	}
	parts := strings.Split(raw, ";")
	mediaType := normalizeMediaType(parts[0])
	base64Part := false
	for _, part := range parts[1:] {
		if strings.EqualFold(strings.TrimSpace(part), "base64") {
			base64Part = true
			break
		}
	}
	return mediaType, base64Part
}

func normalizeMediaType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, _, err := mime.ParseMediaType(value)
	if err == nil {
		value = parsed
	}
	return strings.ToLower(strings.TrimSpace(value))
}

func detectMediaType(data []byte) string {
	sample := data
	if len(sample) > 512 {
		sample = sample[:512]
	}
	return normalizeMediaType(http.DetectContentType(sample))
}

func supportedImageType(mediaType string) bool {
	switch normalizeMediaType(mediaType) {
	case "image/png", "image/jpeg", "image/webp":
		return true
	default:
		return false
	}
}

func headerHasETag(header, etag string) bool {
	for _, part := range strings.Split(header, ",") {
		if strings.TrimSpace(part) == etag {
			return true
		}
	}
	return false
}
