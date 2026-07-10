package httpserve

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestRunListenerStopsWorkBeforeClosingDependencies(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	stopped := make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	})}
	done := make(chan error, 1)
	go func() {
		done <- RunListener(ctx, server, listener, Options{
			ShutdownTimeout: time.Second,
			Stop: func(context.Context) error {
				close(stopped)
				return nil
			},
			Close: func(context.Context) error {
				select {
				case <-stopped:
					return nil
				default:
					t.Error("Close called before Stop")
					return nil
				}
			},
		})
	}()

	resp, err := http.Get("http://" + listener.Addr().String())
	if err != nil {
		t.Fatalf("GET server: %v", err)
	}
	_ = resp.Body.Close()
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("RunListener() error = %v", err)
	}
}

func TestRunListenerForcesCloseAfterShutdownTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	server := &http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(started)
		<-release
	})}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- RunListener(ctx, server, listener, Options{ShutdownTimeout: 20 * time.Millisecond})
	}()
	go func() {
		resp, getErr := http.Get("http://" + listener.Addr().String())
		if getErr == nil {
			_ = resp.Body.Close()
		}
	}()
	<-started
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("RunListener() error = %v, want deadline exceeded", err)
		}
	case <-time.After(time.Second):
		t.Fatal("RunListener hung after shutdown timeout")
	}
}

func TestRunCleansUpWhenListenFails(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := occupied.Close(); err != nil {
			t.Errorf("close listener: %v", err)
		}
	}()

	stopped := false
	closed := false
	err = Run(t.Context(), &http.Server{Addr: occupied.Addr().String()}, Options{
		ShutdownTimeout: time.Second,
		Stop: func(context.Context) error {
			stopped = true
			return nil
		},
		Close: func(context.Context) error {
			closed = true
			return nil
		},
	})
	if err == nil {
		t.Fatal("Run() error = nil, want listen failure")
	}
	if !stopped || !closed {
		t.Fatalf("cleanup called = stop:%v close:%v, want both", stopped, closed)
	}
}

func TestRunListenerNormalCancellationAlwaysReturnsNil(t *testing.T) {
	for range 20 {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "ok")
		})}
		done := make(chan error, 1)
		go func() {
			done <- RunListener(ctx, server, listener, Options{ShutdownTimeout: time.Second})
		}()
		resp, err := http.Get("http://" + listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		cancel()
		if err := <-done; err != nil {
			t.Fatalf("RunListener() cancellation error = %v", err)
		}
	}
}

func TestRunListenerRestartUsesSameShutdownPath(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	restart := make(chan struct{})
	var mu sync.Mutex
	phases := make([]string, 0, 2)
	record := func(value string) {
		mu.Lock()
		defer mu.Unlock()
		phases = append(phases, value)
	}
	done := make(chan error, 1)
	go func() {
		done <- RunListener(t.Context(), &http.Server{}, listener, Options{
			ShutdownTimeout: time.Second,
			Restart:         restart,
			Stop: func(context.Context) error {
				record("stop")
				return nil
			},
			Close: func(context.Context) error {
				record("close")
				return nil
			},
		})
	}()
	close(restart)
	if err := <-done; err != nil {
		t.Fatalf("RunListener() restart error = %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(phases) != 2 || phases[0] != "stop" || phases[1] != "close" {
		t.Fatalf("phases = %v, want [stop close]", phases)
	}
}

func TestRunListenerRejectsMissingTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	err = RunListener(t.Context(), &http.Server{}, listener, Options{})
	if err == nil || err.Error() != "ShutdownTimeout must be positive" {
		t.Fatalf("RunListener() error = %v", err)
	}
}

type failingListener struct {
	err error
}

func (l failingListener) Accept() (net.Conn, error) { return nil, l.err }
func (failingListener) Close() error                { return nil }
func (failingListener) Addr() net.Addr              { return &net.TCPAddr{} }

func TestRunListenerClosesDependenciesAfterServeError(t *testing.T) {
	errServe := errors.New("forced serve failure")
	closed := false
	err := RunListener(t.Context(), &http.Server{}, failingListener{err: errServe}, Options{
		ShutdownTimeout: time.Second,
		Close: func(ctx context.Context) error {
			if _, ok := ctx.Deadline(); !ok {
				return errors.New("close context has no deadline")
			}
			closed = true
			return nil
		},
	})
	if !errors.Is(err, errServe) {
		t.Fatalf("RunListener() error = %v, want serve error", err)
	}
	if !closed {
		t.Fatal("Close was not called after serve failure")
	}
}

func TestRunListenerJoinsStopAndCloseErrors(t *testing.T) {
	errServe := errors.New("forced serve failure")
	errStop := errors.New("forced stop failure")
	errClose := errors.New("forced dependency close failure")
	err := RunListener(t.Context(), &http.Server{}, failingListener{err: errServe}, Options{
		ShutdownTimeout: time.Second,
		Stop: func(context.Context) error {
			return errStop
		},
		Close: func(context.Context) error {
			return errClose
		},
	})
	for _, want := range []error{errServe, errStop, errClose} {
		if !errors.Is(err, want) {
			t.Fatalf("RunListener() error = %v, missing %v", err, want)
		}
	}
}
