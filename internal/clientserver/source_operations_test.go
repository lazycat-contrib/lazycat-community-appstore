package clientserver

import (
	"testing"
	"time"
)

func TestSourceOperationCoordinatorSerializesOneSource(t *testing.T) {
	coordinator := newSourceOperationCoordinator()
	releaseFirst := coordinator.lock(1)

	sameSourceAcquired := make(chan struct{})
	go func() {
		release := coordinator.lock(1)
		close(sameSourceAcquired)
		release()
	}()

	otherSourceAcquired := make(chan struct{})
	go func() {
		release := coordinator.lock(2)
		close(otherSourceAcquired)
		release()
	}()

	select {
	case <-otherSourceAcquired:
	case <-time.After(time.Second):
		t.Fatal("independent source was blocked")
	}
	select {
	case <-sameSourceAcquired:
		t.Fatal("same source acquired before the first operation released")
	case <-time.After(50 * time.Millisecond):
	}

	releaseFirst()
	select {
	case <-sameSourceAcquired:
	case <-time.After(time.Second):
		t.Fatal("same source did not resume after release")
	}
}
