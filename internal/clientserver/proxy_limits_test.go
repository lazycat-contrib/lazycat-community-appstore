package clientserver

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestChatAndCommentProxyResponseLimits(t *testing.T) {
	var responseBytes atomic.Int64
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("X-Upstream", "must-not-leak-on-overflow")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.CopyN(w, zeroProxyReader{}, responseBytes.Load())
	}))
	t.Cleanup(sourceServer.Close)
	app := testServer(t)
	source := app.server.db.ClientSource.Create().
		SetUserID("alice").
		SetName("Proxy Limits").
		SetURL(sourceServer.URL + "/source/v1/index.json").
		SetChatAvailable(true).
		SetChatEnabled(true).
		SaveX(t.Context())
	sourceApp := app.server.db.ClientSourceApp.Create().
		SetSourceID(source.ID).
		SetExternalID("external-app").
		SetPackageID("cloud.lazycat.proxy-limits").
		SetName("Proxy Limits").
		SetSlug("proxy-limits").
		SetCommentsEnabled(true).
		SaveX(t.Context())

	for _, endpoint := range []struct {
		name string
		path string
		body string
	}{
		{name: "chat", path: fmt.Sprintf("/api/client/v1/apps/%d/chat", sourceApp.ID), body: `{}`},
		{name: "comment", path: fmt.Sprintf("/api/client/v1/apps/%d/comments", sourceApp.ID), body: `{"body":"hello"}`},
	} {
		t.Run(endpoint.name+" exact", func(t *testing.T) {
			responseBytes.Store(maxSourceProxyResponseBytes)
			rec := proxyLimitRequest(app, endpoint.path, endpoint.body)
			if rec.Code != http.StatusCreated || int64(rec.Body.Len()) != maxSourceProxyResponseBytes {
				t.Fatalf("exact response code=%d bytes=%d body-prefix=%q", rec.Code, rec.Body.Len(), rec.Body.String()[:min(rec.Body.Len(), 64)])
			}
			if rec.Header().Get("Content-Type") != "application/octet-stream" {
				t.Fatalf("exact Content-Type = %q", rec.Header().Get("Content-Type"))
			}
		})

		t.Run(endpoint.name+" overflow", func(t *testing.T) {
			responseBytes.Store(maxSourceProxyResponseBytes + 1)
			rec := proxyLimitRequest(app, endpoint.path, endpoint.body)
			if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "SOURCE_RESPONSE_TOO_LARGE") {
				t.Fatalf("overflow response code=%d body=%s", rec.Code, rec.Body.String())
			}
			if rec.Header().Get("X-Upstream") != "" || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
				t.Fatalf("overflow headers leaked upstream metadata: %v", rec.Header())
			}
		})
	}
}

func proxyLimitRequest(app *clientTestServer, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-hc-user-id", "alice")
	req.Header.Set("x-hc-device-id", "device-1")
	rec := httptest.NewRecorder()
	app.handler.ServeHTTP(rec, req)
	return rec
}

type zeroProxyReader struct{}

func (zeroProxyReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
