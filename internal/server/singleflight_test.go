package server

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
)

func TestSharedFirstLoadWaiterCanCancel(t *testing.T) {
	serverCtx, serverCancel := context.WithCancel(t.Context())
	srv := &Server{ctx: serverCtx, cancel: serverCancel}
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	requestCtx, requestCancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		_, err := srv.sharedFirstLoad(requestCtx, "apps", func(context.Context) (any, error) {
			<-release
			return "loaded", nil
		})
		done <- err
	}()
	requestCancel()

	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("sharedFirstLoad() error = %v, want canceled", err)
	}
}

func TestSharedFirstLoadDeduplicatesLiveCallers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		serverCtx, serverCancel := context.WithCancel(t.Context())
		defer serverCancel()
		srv := &Server{ctx: serverCtx, cancel: serverCancel}
		started := make(chan struct{})
		release := make(chan struct{})
		var calls atomic.Int64
		load := func(context.Context) (any, error) {
			if calls.Add(1) == 1 {
				close(started)
			}
			<-release
			return "loaded", nil
		}
		results := make(chan error, 2)
		go func() {
			_, err := srv.sharedFirstLoad(t.Context(), "apps", load)
			results <- err
		}()
		<-started
		go func() {
			_, err := srv.sharedFirstLoad(t.Context(), "apps", load)
			results <- err
		}()
		synctest.Wait()
		close(release)
		if err := <-results; err != nil {
			t.Fatal(err)
		}
		if err := <-results; err != nil {
			t.Fatal(err)
		}
		if calls.Load() != 1 {
			t.Fatalf("load calls = %d, want 1", calls.Load())
		}
	})
}

func TestServerStopCancelsAndTracksSharedLoad(t *testing.T) {
	serverCtx, serverCancel := context.WithCancel(t.Context())
	srv := &Server{ctx: serverCtx, cancel: serverCancel}
	started := make(chan struct{})
	exited := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := srv.sharedFirstLoad(context.Background(), "apps", func(ctx context.Context) (any, error) {
			close(started)
			<-ctx.Done()
			close(exited)
			return nil, ctx.Err()
		})
		done <- err
	}()
	<-started
	if err := srv.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	waited := make(chan struct{})
	go func() {
		srv.backgroundWG.Wait()
		close(waited)
	}()
	select {
	case <-waited:
	case <-t.Context().Done():
		t.Fatal("shared load did not stop")
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("sharedFirstLoad() error = %v, want canceled", err)
	}
	select {
	case <-exited:
	default:
		t.Fatal("load exit was not observed")
	}
}
