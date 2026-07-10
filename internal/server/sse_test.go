package server

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type failingResponseWriter struct {
	err error
}

func (w *failingResponseWriter) Header() http.Header       { return make(http.Header) }
func (w *failingResponseWriter) WriteHeader(int)           {}
func (w *failingResponseWriter) Write([]byte) (int, error) { return 0, w.err }

func TestWriteSSEPropagatesWriteFailure(t *testing.T) {
	writer := &failingResponseWriter{err: io.ErrClosedPipe}
	err := writeSSE(writer, time.Second, ": heartbeat\n\n")
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("writeSSE() error = %v, want closed pipe", err)
	}
}

func TestWriteSSERenewsServerWriteDeadline(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		ticker := time.NewTicker(15 * time.Millisecond)
		defer ticker.Stop()
		for range 3 {
			<-ticker.C
			if err := writeSSE(w, 25*time.Millisecond, ": heartbeat\n\n"); err != nil {
				return
			}
		}
	})
	server := httptest.NewUnstartedServer(handler)
	server.Config.WriteTimeout = 20 * time.Millisecond
	server.Start()
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("close response body: %v", err)
		}
	}()

	seen := 0
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if scanner.Text() == ": heartbeat" {
			seen++
			if seen == 3 {
				return
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan SSE: %v", err)
	}
	t.Fatalf("received %d heartbeats, want 3", seen)
}
