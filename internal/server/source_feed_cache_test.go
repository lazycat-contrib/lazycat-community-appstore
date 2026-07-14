package server

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSourceFeedCacheCachesAndInvalidates(t *testing.T) {
	var builds atomic.Int32
	cache, err := newSourceFeedCache(t.Context(), "", func(context.Context, int, sourceFeedAccessScope) ([]byte, error) {
		builds.Add(1)
		return []byte(`{"apps":[]}`), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cache.Close() })

	first, err := cache.GetOrBuild(t.Context(), 2, sourceFeedAccessScope{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.GetOrBuild(t.Context(), 2, sourceFeedAccessScope{})
	if err != nil {
		t.Fatal(err)
	}
	if builds.Load() != 1 {
		t.Fatalf("builds = %d, want 1", builds.Load())
	}
	if first.ETag == "" || first.ETag != second.ETag {
		t.Fatalf("ETags = %q and %q", first.ETag, second.ETag)
	}
	if len(first.Brotli) == 0 || len(first.Gzip) == 0 {
		t.Fatal("compressed representations are empty")
	}

	cache.InvalidateAndWarm()
	third, err := cache.GetOrBuild(t.Context(), 2, sourceFeedAccessScope{})
	if err != nil {
		t.Fatal(err)
	}
	if builds.Load() < 2 {
		t.Fatalf("builds after invalidation = %d, want at least 2", builds.Load())
	}
	if third.ETag != first.ETag {
		t.Fatalf("stable content ETag changed: %q != %q", third.ETag, first.ETag)
	}
}

func TestSourceFeedCacheCollapsesConcurrentMisses(t *testing.T) {
	var builds atomic.Int32
	entered := make(chan struct{})
	release := make(chan struct{})
	cache, err := newSourceFeedCache(t.Context(), "", func(context.Context, int, sourceFeedAccessScope) ([]byte, error) {
		if builds.Add(1) == 1 {
			close(entered)
		}
		<-release
		return []byte(`{"apps":[{"id":1}]}`), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cache.Close() })

	const callers = 12
	var wg sync.WaitGroup
	errs := make(chan error, callers)
	for range callers {
		wg.Go(func() {
			_, err := cache.GetOrBuild(t.Context(), 2, sourceFeedAccessScope{})
			errs <- err
		})
	}
	<-entered
	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if builds.Load() != 1 {
		t.Fatalf("builds = %d, want 1", builds.Load())
	}
}

func TestSourceFeedCacheDoesNotReusePreviousBoot(t *testing.T) {
	path := t.TempDir()
	first, err := newSourceFeedCache(t.Context(), path, func(context.Context, int, sourceFeedAccessScope) ([]byte, error) {
		return []byte(`{"boot":1}`), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.GetOrBuild(t.Context(), 2, sourceFeedAccessScope{}); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	var builds atomic.Int32
	second, err := newSourceFeedCache(t.Context(), path, func(context.Context, int, sourceFeedAccessScope) ([]byte, error) {
		builds.Add(1)
		return []byte(`{"boot":2}`), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })
	snapshot, err := second.GetOrBuild(t.Context(), 2, sourceFeedAccessScope{})
	if err != nil {
		t.Fatal(err)
	}
	if builds.Load() != 1 || string(snapshot.Identity) != `{"boot":2}` {
		t.Fatalf("reused previous boot: builds=%d body=%s", builds.Load(), snapshot.Identity)
	}
}

func TestSourceFeedCacheCloseWaitsForCanceledWaiterBuild(t *testing.T) {
	entered := make(chan struct{})
	cacheCanceled := make(chan struct{})
	release := make(chan struct{})
	exited := make(chan struct{})
	cache, err := newSourceFeedCache(t.Context(), "", func(ctx context.Context, _ int, _ sourceFeedAccessScope) ([]byte, error) {
		close(entered)
		<-ctx.Done()
		close(cacheCanceled)
		<-release
		close(exited)
		return nil, ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}

	callerCtx, cancel := context.WithCancel(t.Context())
	requestDone := make(chan error, 1)
	go func() {
		_, err := cache.GetOrBuild(callerCtx, 2, sourceFeedAccessScope{})
		requestDone <- err
	}()
	<-entered
	cancel()
	if err := <-requestDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("request error = %v, want canceled", err)
	}
	closeDone := make(chan error, 1)
	go func() { closeDone <- cache.Close() }()
	<-cacheCanceled
	select {
	case err := <-closeDone:
		t.Fatalf("Close returned before the shared build was released: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	if err := <-closeDone; err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
	default:
		t.Fatal("Close returned before the shared build exited")
	}
}
