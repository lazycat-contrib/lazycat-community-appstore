package clientserver

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type failingSSEWriter struct {
	err error
}

func (w *failingSSEWriter) Header() http.Header       { return make(http.Header) }
func (w *failingSSEWriter) WriteHeader(int)           {}
func (w *failingSSEWriter) Write([]byte) (int, error) { return 0, w.err }

func TestCopySSECopiesAndFlushesFrames(t *testing.T) {
	const frames = "event: chat\ndata: one\n\nevent: chat\ndata: two\n\n"
	recorder := httptest.NewRecorder()
	if err := copySSE(t.Context(), recorder, strings.NewReader(frames), time.Second); err != nil {
		t.Fatalf("copySSE() error = %v", err)
	}
	if got := recorder.Body.String(); got != frames {
		t.Fatalf("copySSE() body = %q, want %q", got, frames)
	}
}

func TestCopySSEReturnsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	reader, writer := io.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- copySSE(ctx, httptest.NewRecorder(), reader, time.Second)
	}()
	cancel()
	if err := writer.CloseWithError(context.Canceled); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("copySSE() error = %v, want canceled", err)
	}
}

func TestCopySSEPropagatesWriteFailure(t *testing.T) {
	writer := &failingSSEWriter{err: io.ErrClosedPipe}
	err := copySSE(t.Context(), writer, strings.NewReader("data: one\n\n"), time.Second)
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("copySSE() error = %v, want closed pipe", err)
	}
}
