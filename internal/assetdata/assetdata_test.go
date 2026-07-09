package assetdata

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var benchmarkPayload Payload
var benchmarkMetadataHandled bool

func TestParseDataURLAndAssetID(t *testing.T) {
	raw := testPNG(t, 2, 1)
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)

	payload, err := ParseDataURL(dataURL, int64(len(raw)))
	if err != nil {
		t.Fatalf("ParseDataURL returned error: %v", err)
	}
	if payload.MediaType != "image/png" {
		t.Fatalf("media type = %q, want image/png", payload.MediaType)
	}
	if !bytes.Equal(payload.Data, raw) {
		t.Fatal("payload data does not match input")
	}

	for _, value := range []string{"/api/v1/assets/42", "https://store.example/api/v1/assets/42"} {
		id, ok := AssetIDFromURL("/api/v1/assets", value)
		if !ok || id != 42 {
			t.Fatalf("AssetIDFromURL(%q) = %d, %v; want 42, true", value, id, ok)
		}
	}
	if _, ok := AssetIDFromURL("/api/v1/assets", "/api/v1/assets/not-an-id"); ok {
		t.Fatal("AssetIDFromURL accepted invalid id")
	}
}

func TestNewImagePayloadValidation(t *testing.T) {
	raw := testPNG(t, 1, 1)
	if _, err := NewImagePayload(raw, "image/jpeg", int64(len(raw))); err == nil {
		t.Fatal("expected declared media type mismatch to fail")
	}
	if _, err := NewImagePayload(raw, "image/png", int64(len(raw)-1)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("too large error = %v, want ErrTooLarge", err)
	}
	if _, err := NewImagePayload([]byte("not an image"), "image/png", 1024); !errors.Is(err, ErrUnsupportedImage) {
		t.Fatalf("unsupported image error = %v, want ErrUnsupportedImage", err)
	}
}

func TestNormalizeImageResizesToMaxSide(t *testing.T) {
	payload, err := NormalizeImage(testPNG(t, 512, 256), "image/png", 1<<20, 256)
	if err != nil {
		t.Fatalf("NormalizeImage returned error: %v", err)
	}
	if payload.MediaType != "image/png" {
		t.Fatalf("media type = %q, want image/png", payload.MediaType)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(payload.Data))
	if err != nil {
		t.Fatalf("decode normalized image: %v", err)
	}
	if cfg.Width != 256 || cfg.Height != 128 {
		t.Fatalf("normalized size = %dx%d, want 256x128", cfg.Width, cfg.Height)
	}
}

func TestServeImageMetadataHandlesCacheValidationAndHead(t *testing.T) {
	etag := ETag(strings.Repeat("a", 64))
	req := httptest.NewRequest(http.MethodGet, "/asset.png", nil)
	req.Header.Set("If-None-Match", etag)
	rec := httptest.NewRecorder()

	if !ServeImageMetadata(rec, req, "image/png", strings.Repeat("a", 64), 123) {
		t.Fatal("ServeImageMetadata did not handle matching ETag")
	}
	if rec.Result().StatusCode != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", rec.Result().StatusCode)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("body length = %d, want 0", rec.Body.Len())
	}

	headReq := httptest.NewRequest(http.MethodHead, "/asset.png", nil)
	headRec := httptest.NewRecorder()
	if !ServeImageMetadata(headRec, headReq, "image/png", strings.Repeat("b", 64), 456) {
		t.Fatal("ServeImageMetadata did not handle HEAD")
	}
	if headRec.Result().StatusCode != http.StatusOK {
		t.Fatalf("HEAD status = %d, want 200", headRec.Result().StatusCode)
	}
	if headRec.Header().Get("Content-Length") != "456" {
		t.Fatalf("HEAD content length = %q, want 456", headRec.Header().Get("Content-Length"))
	}
}

func BenchmarkParseDataURL(b *testing.B) {
	raw := testPNG(b, 128, 128)
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		payload, err := ParseDataURL(dataURL, 1<<20)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkPayload = payload
	}
}

func BenchmarkNormalizeImage(b *testing.B) {
	raw := testPNG(b, 512, 512)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		payload, err := NormalizeImage(raw, "image/png", 1<<20, 256)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkPayload = payload
	}
}

func BenchmarkServeImageMetadataNotModified(b *testing.B) {
	sha := strings.Repeat("a", 64)
	req := httptest.NewRequest(http.MethodGet, "/asset.png", nil)
	req.Header.Set("If-None-Match", ETag(sha))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		benchmarkMetadataHandled = ServeImageMetadata(rec, req, "image/png", sha, 1024)
	}
}

func testPNG(tb testing.TB, width, height int) []byte {
	tb.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: uint8(x + y), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		tb.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}
