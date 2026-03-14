package server

import (
	"sync"
)

// Event is a server-sent event.
type Event struct {
	Type string
	Data string
}

// Broadcaster is an in-memory pub/sub for SSE events.
type Broadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan Event]struct{}
}

// NewBroadcaster creates a new Broadcaster.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subscribers: make(map[chan Event]struct{}),
	}
}

// Subscribe returns a channel that receives broadcast events and a cancel function.
func (b *Broadcaster) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 64)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		delete(b.subscribers, ch)
		b.mu.Unlock()
	}
	return ch, cancel
}

// Broadcast sends an event to all subscribers. Non-blocking — drops if a subscriber's buffer is full.
func (b *Broadcaster) Broadcast(ev Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subscribers {
		select {
		case ch <- ev:
		default:
			// Subscriber too slow — drop event.
		}
	}
}
