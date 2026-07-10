package server

import (
	"sync"
	"sync/atomic"
)

type chatEvent struct {
	Type           string `json:"type"`
	ConversationID int    `json:"conversationId"`
}

type chatHub struct {
	mu      sync.Mutex
	clients map[chan chatEvent]struct{}
	dropped atomic.Uint64
}

func (h *chatHub) droppedEvents() uint64 {
	return h.dropped.Load()
}

func newChatHub() *chatHub {
	return &chatHub{clients: map[chan chatEvent]struct{}{}}
}

func (h *chatHub) subscribe() (chan chatEvent, func()) {
	ch := make(chan chatEvent, 8)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		if _, ok := h.clients[ch]; ok {
			delete(h.clients, ch)
			close(ch)
		}
		h.mu.Unlock()
	}
}

// broadcast sends invalidation hints only. A slow subscriber may lose a hint;
// every received hint causes the client to reload authoritative state.
func (h *chatHub) broadcast(event chatEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- event:
		default:
			h.dropped.Add(1)
		}
	}
}
