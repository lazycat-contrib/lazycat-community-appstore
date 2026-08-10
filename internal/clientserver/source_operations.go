package clientserver

import "sync"

// sourceOperationCoordinator serializes mutations for one source while still
// allowing independent sources to synchronize in parallel.
type sourceOperationCoordinator struct {
	mu    sync.Mutex
	locks map[int]*sourceOperationLock
}

type sourceOperationLock struct {
	mu   sync.Mutex
	refs int
}

func newSourceOperationCoordinator() *sourceOperationCoordinator {
	return &sourceOperationCoordinator{locks: make(map[int]*sourceOperationLock)}
}

func (c *sourceOperationCoordinator) lock(sourceID int) func() {
	c.mu.Lock()
	entry := c.locks[sourceID]
	if entry == nil {
		entry = &sourceOperationLock{}
		c.locks[sourceID] = entry
	}
	entry.refs++
	c.mu.Unlock()

	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		c.mu.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(c.locks, sourceID)
		}
		c.mu.Unlock()
	}
}
