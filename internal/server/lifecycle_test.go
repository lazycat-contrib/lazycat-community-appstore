package server

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"lazycat.community/appstore/internal/httpserve"

	"go.uber.org/goleak"
)

func TestRequestRestartClosesChannelOnce(t *testing.T) {
	srv := &Server{restartRequested: make(chan struct{})}
	srv.requestRestart()
	srv.requestRestart()

	select {
	case <-srv.RestartRequested():
	default:
		t.Fatal("restart channel is still open")
	}
}

func TestShutdownDrainsActiveHandlerBeforeClosingDatabase(t *testing.T) {
	app := newTestApp(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	app.server.mux.HandleFunc("GET /shutdown-db-regression", func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		if _, err := app.server.db.SiteSetting.Query().Count(r.Context()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = io.WriteString(w, "db-ok")
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	stopped := make(chan struct{})
	runDone := make(chan error, 1)
	go func() {
		runDone <- httpserve.RunListener(ctx, &http.Server{Handler: app.server.Handler()}, listener, httpserve.Options{
			ShutdownTimeout: 2 * time.Second,
			Stop: func(ctx context.Context) error {
				err := app.server.Stop(ctx)
				close(stopped)
				return err
			},
			Close: app.server.CloseContext,
		})
	}()
	responseDone := make(chan *http.Response, 1)
	requestErr := make(chan error, 1)
	go func() {
		//nolint:bodyclose // Ownership of the response body transfers through responseDone.
		resp, err := http.Get("http://" + listener.Addr().String() + "/shutdown-db-regression")
		if err != nil {
			requestErr <- err
			return
		}
		responseDone <- resp
	}()
	<-entered
	cancel()
	<-stopped
	close(release)
	select {
	case err := <-requestErr:
		t.Fatal(err)
	case resp := <-responseDone:
		defer func() { _ = resp.Body.Close() }()
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK || string(raw) != "db-ok" {
			t.Fatalf("active handler response = %d %q", resp.StatusCode, raw)
		}
	case <-t.Context().Done():
		t.Fatal("active handler did not finish")
	}
	if err := <-runDone; err != nil {
		t.Fatalf("RunListener() error = %v", err)
	}
}

func TestServerCloseTimeoutDoesNotCreateWaiterPerCall(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	serverCtx, cancel := context.WithCancel(t.Context())
	srv := &Server{ctx: serverCtx, cancel: cancel, closeDone: make(chan struct{})}
	release := make(chan struct{})
	srv.backgroundWG.Add(1)
	go func() {
		defer srv.backgroundWG.Done()
		<-release
	}()

	for range 20 {
		ctx, stop := context.WithTimeout(t.Context(), time.Nanosecond)
		err := srv.CloseContext(ctx)
		stop()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("CloseContext() error = %v, want deadline exceeded", err)
		}
	}
	close(release)
	if err := srv.CloseContext(t.Context()); err != nil {
		t.Fatalf("CloseContext() after release error = %v", err)
	}
}

func TestServerStopTimeoutDoesNotCreateWaiterPerCall(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	serverCtx, cancel := context.WithCancel(t.Context())
	srv := &Server{ctx: serverCtx, cancel: cancel, stopDone: make(chan struct{}), closeDone: make(chan struct{})}
	release := make(chan struct{})
	srv.backgroundWG.Add(1)
	go func() {
		defer srv.backgroundWG.Done()
		<-release
	}()
	for range 20 {
		ctx, stop := context.WithTimeout(t.Context(), time.Nanosecond)
		err := srv.Stop(ctx)
		stop()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Stop() error = %v, want deadline exceeded", err)
		}
	}
	close(release)
	if err := srv.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() after release error = %v", err)
	}
	if err := srv.CloseContext(t.Context()); err != nil {
		t.Fatalf("CloseContext() error = %v", err)
	}
}
