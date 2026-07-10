package clientserver

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestSchedulerCloseWaitsForStartupSync(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	started := make(chan struct{})
	exited := make(chan struct{})
	scheduler, err := newSourceSyncSchedulerWithStartup(&Server{}, func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		close(exited)
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started

	if err := scheduler.CloseContext(t.Context()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	select {
	case <-exited:
	default:
		t.Fatal("Close returned before startup sync exited")
	}
}

func TestSchedulerCloseTimeoutDoesNotCreateWaiterPerCall(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	started := make(chan struct{})
	release := make(chan struct{})
	scheduler, err := newSourceSyncSchedulerWithStartup(&Server{}, func(context.Context) {
		close(started)
		<-release
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	for range 20 {
		ctx, cancel := context.WithTimeout(t.Context(), time.Nanosecond)
		err := scheduler.CloseContext(ctx)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("CloseContext() error = %v, want deadline exceeded", err)
		}
	}
	close(release)
	if err := scheduler.CloseContext(t.Context()); err != nil {
		t.Fatalf("CloseContext() after release error = %v", err)
	}
}
