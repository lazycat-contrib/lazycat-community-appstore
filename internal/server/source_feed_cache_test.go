package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lazycat.community/appstore/ent/announcement"
)

func TestSourceFeedCacheCachesAndInvalidates(t *testing.T) {
	var builds atomic.Int32
	cache, err := newSourceFeedCache(t.Context(), "", func(context.Context, int, sourceFeedAccessScope, []string) (sourceFeedBuild, error) {
		builds.Add(1)
		return sourceFeedBuild{Identity: []byte(`{"apps":[]}`)}, nil
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

func TestSourceFeedCacheBoundsPersistedSnapshots(t *testing.T) {
	cache, err := newSourceFeedCache(t.Context(), "", func(context.Context, int, sourceFeedAccessScope, []string) (sourceFeedBuild, error) {
		return sourceFeedBuild{Identity: []byte(`{"apps":[]}`)}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cache.Close() })
	oversizedPrefix := cache.snapshotPrefix(cache.generation.Load(), 2, "oversized")
	if cache.admit(oversizedPrefix, sourceFeedCacheMaxBytes+1) {
		t.Fatal("oversized source snapshot was admitted")
	}

	for id := 1; id <= sourceFeedCacheMaxSnapshots+32; id++ {
		if _, err := cache.GetOrBuild(t.Context(), 2, sourceFeedAccessScope{GroupIDs: []int{id}}); err != nil {
			t.Fatal(err)
		}
	}
	cache.mu.Lock()
	admitted := len(cache.admitted)
	cache.mu.Unlock()
	if admitted != sourceFeedCacheMaxSnapshots {
		t.Fatalf("persisted snapshot admissions = %d, want %d", admitted, sourceFeedCacheMaxSnapshots)
	}
}

func TestSourceFeedCacheRejectsStaleScopeAfterInvalidation(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	cache, err := newSourceFeedCache(t.Context(), "", func(_ context.Context, _ int, scope sourceFeedAccessScope, _ []string) (sourceFeedBuild, error) {
		if len(scope.Groups) > 0 && scope.Groups[0].Name == "Old" {
			close(entered)
			<-release
		}
		name := "public"
		if len(scope.Groups) > 0 {
			name = scope.Groups[0].Name
		}
		return sourceFeedBuild{Identity: []byte(name)}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cache.Close() })

	requestDone := make(chan error, 1)
	go func() {
		_, err := cache.GetOrBuild(t.Context(), 2, sourceFeedAccessScope{
			GroupIDs: []int{1},
			Groups:   []sourceGroupDTO{{ID: 1, Name: "Old", Code: "OLD111"}},
		})
		requestDone <- err
	}()
	<-entered
	cache.InvalidateAndWarm()
	close(release)
	if err := <-requestDone; !errors.Is(err, errSourceFeedGenerationChanged) {
		t.Fatalf("stale scope error = %v, want generation change", err)
	}

	snapshot, err := cache.GetOrBuild(t.Context(), 2, sourceFeedAccessScope{
		GroupIDs: []int{1},
		Groups:   []sourceGroupDTO{{ID: 1, Name: "New", Code: "NEW111"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(snapshot.Identity) != "New" {
		t.Fatalf("stale group scope cached in new generation: %q", snapshot.Identity)
	}
}

func TestSourceFeedCacheExpiresAtAnnouncementBoundary(t *testing.T) {
	app := newTestApp(t)
	startsAt := time.Now().UTC().Add(time.Second)
	app.server.db.Announcement.Create().
		SetEnabled(true).
		SetLevel(announcement.LevelInfo).
		SetTitle("Scheduled notice").
		SetStartsAt(startsAt).
		SaveX(t.Context())

	before := app.do(http.MethodGet, "/source/v2/index.json", nil)
	if before.Code != http.StatusOK || strings.Contains(before.Body.String(), "Scheduled notice") {
		t.Fatalf("feed before announcement boundary = %d %s", before.Code, before.Body.String())
	}
	timer := time.NewTimer(time.Until(startsAt) + 20*time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}
	after := app.do(http.MethodGet, "/source/v2/index.json", nil)
	if after.Code != http.StatusOK || !strings.Contains(after.Body.String(), "Scheduled notice") {
		t.Fatalf("feed after announcement boundary = %d %s", after.Code, after.Body.String())
	}
	if before.Header().Get("ETag") == after.Header().Get("ETag") {
		t.Fatalf("scheduled announcement did not change ETag: %q", after.Header().Get("ETag"))
	}
}

func TestSourceFeedCacheCollapsesConcurrentMisses(t *testing.T) {
	var builds atomic.Int32
	entered := make(chan struct{})
	release := make(chan struct{})
	cache, err := newSourceFeedCache(t.Context(), "", func(context.Context, int, sourceFeedAccessScope, []string) (sourceFeedBuild, error) {
		if builds.Add(1) == 1 {
			close(entered)
		}
		<-release
		return sourceFeedBuild{Identity: []byte(`{"apps":[{"id":1}]}`)}, nil
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
	first, err := newSourceFeedCache(t.Context(), path, func(context.Context, int, sourceFeedAccessScope, []string) (sourceFeedBuild, error) {
		return sourceFeedBuild{Identity: []byte(`{"boot":1}`)}, nil
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
	second, err := newSourceFeedCache(t.Context(), path, func(context.Context, int, sourceFeedAccessScope, []string) (sourceFeedBuild, error) {
		builds.Add(1)
		return sourceFeedBuild{Identity: []byte(`{"boot":2}`)}, nil
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
	cache, err := newSourceFeedCache(t.Context(), "", func(ctx context.Context, _ int, _ sourceFeedAccessScope, _ []string) (sourceFeedBuild, error) {
		close(entered)
		<-ctx.Done()
		close(cacheCanceled)
		<-release
		close(exited)
		return sourceFeedBuild{}, ctx.Err()
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
