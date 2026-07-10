package server

import "testing"

func TestChatHubCountsDroppedInvalidationHints(t *testing.T) {
	hub := newChatHub()
	events, unsubscribe := hub.subscribe()
	defer unsubscribe()
	for range cap(events) {
		hub.broadcast(chatEvent{Type: "message"})
	}
	hub.broadcast(chatEvent{Type: "message"})
	if got := hub.droppedEvents(); got != 1 {
		t.Fatalf("droppedEvents() = %d, want 1", got)
	}
}
