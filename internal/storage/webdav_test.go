package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebDAVBackendSaveUsesPUTWithBasicAuth(t *testing.T) {
	var sawPut bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "MKCOL" {
			w.WriteHeader(http.StatusCreated)
			return
		}
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "user" || pass != "pass" {
			t.Fatalf("missing basic auth")
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "content" {
			t.Fatalf("body = %q, want content", string(body))
		}
		if !strings.HasSuffix(r.URL.Path, ".lpk") {
			t.Fatalf("path = %s, want .lpk suffix", r.URL.Path)
		}
		sawPut = true
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	backend := NewWebDAVBackend(server.URL, "user", "pass", server.URL)
	obj, err := backend.Save(context.Background(), "demo.lpk", strings.NewReader("content"))
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if !sawPut {
		t.Fatal("server did not receive PUT")
	}
	if !strings.HasPrefix(obj.DownloadURL, server.URL) {
		t.Fatalf("DownloadURL = %q, want server URL prefix", obj.DownloadURL)
	}
	if obj.Size != int64(len("content")) {
		t.Fatalf("Size = %d, want %d", obj.Size, len("content"))
	}
}
