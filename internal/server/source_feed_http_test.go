package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
)

func TestSourceFeedNegotiatesCompressionAndConditionalRequests(t *testing.T) {
	app := newTestApp(t)

	tests := []struct {
		name           string
		acceptEncoding string
		wantEncoding   string
		decode         func(io.Reader) io.Reader
	}{
		{name: "identity"},
		{name: "gzip old client", acceptEncoding: "gzip", wantEncoding: "gzip", decode: func(reader io.Reader) io.Reader {
			decoded, err := gzip.NewReader(reader)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = decoded.Close() })
			return decoded
		}},
		{name: "brotli preferred", acceptEncoding: "gzip, br", wantEncoding: "br", decode: func(reader io.Reader) io.Reader { return brotli.NewReader(reader) }},
		{name: "quality values", acceptEncoding: "br;q=0.2, gzip;q=0.8", wantEncoding: "gzip", decode: func(reader io.Reader) io.Reader {
			decoded, err := gzip.NewReader(reader)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = decoded.Close() })
			return decoded
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/source/v2/index.json", nil)
			if tc.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tc.acceptEncoding)
			}
			rec := httptest.NewRecorder()
			app.handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Encoding"); got != tc.wantEncoding {
				t.Fatalf("Content-Encoding = %q, want %q", got, tc.wantEncoding)
			}
			if rec.Header().Get("ETag") == "" {
				t.Fatal("missing ETag")
			}
			vary := rec.Header().Get("Vary")
			for _, expected := range []string{"Accept-Encoding", "X-Group-Codes", "X-Source-Password"} {
				if !strings.Contains(vary, expected) {
					t.Fatalf("Vary = %q, missing %s", vary, expected)
				}
			}
			reader := io.Reader(rec.Body)
			if tc.decode != nil {
				reader = tc.decode(reader)
			}
			body, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), `"apps"`) {
				t.Fatalf("decoded body = %s", body)
			}

			conditional := httptest.NewRequest(http.MethodGet, "/source/v2/index.json", nil)
			conditional.Header.Set("If-None-Match", rec.Header().Get("ETag"))
			conditionalRec := httptest.NewRecorder()
			app.handler.ServeHTTP(conditionalRec, conditional)
			if conditionalRec.Code != http.StatusNotModified || conditionalRec.Body.Len() != 0 {
				t.Fatalf("conditional response = %d bodyLen=%d", conditionalRec.Code, conditionalRec.Body.Len())
			}
		})
	}
}

func TestSourceFeedRejectsExcessiveGroupCodes(t *testing.T) {
	app := newTestApp(t)
	codes := make([]string, 65)
	for index := range codes {
		codes[index] = "AA" + strings.Repeat("A", 2) + string(rune('A'+index/26)) + string(rune('A'+index%26))
	}
	req := httptest.NewRequest(http.MethodGet, "/source/v2/index.json", nil)
	req.Header.Set("X-Group-Codes", strings.Join(codes, ","))
	rec := httptest.NewRecorder()
	app.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
}
