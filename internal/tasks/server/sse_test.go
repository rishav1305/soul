package server

import (
	"testing"
	"time"
)

func TestBroadcaster_SubscribeReceivesEvents(t *testing.T) {
	b := NewBroadcaster()

	ch, cancel := b.Subscribe()
	defer cancel()

	go b.Broadcast(Event{Type: "task.created", Data: `{"id":1}`})

	select {
	case ev := <-ch:
		if ev.Type != "task.created" {
			t.Errorf("Type = %q, want %q", ev.Type, "task.created")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestBroadcaster_CancelRemovesSubscriber(t *testing.T) {
	b := NewBroadcaster()

	_, cancel := b.Subscribe()
	cancel()

	b.mu.RLock()
	count := len(b.subscribers)
	b.mu.RUnlock()

	if count != 0 {
		t.Errorf("subscribers = %d, want 0 after cancel", count)
	}
}

func TestBroadcaster_MultipleSubscribers(t *testing.T) {
	b := NewBroadcaster()

	ch1, cancel1 := b.Subscribe()
	defer cancel1()
	ch2, cancel2 := b.Subscribe()
	defer cancel2()

	go b.Broadcast(Event{Type: "test", Data: "{}"})

	for _, ch := range []<-chan Event{ch1, ch2} {
		select {
		case ev := <-ch:
			if ev.Type != "test" {
				t.Errorf("Type = %q, want %q", ev.Type, "test")
			}
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for event")
		}
	}
}

func TestBroadcaster_SlowSubscriberDoesNotBlock(t *testing.T) {
	b := NewBroadcaster()

	// Subscribe but never read — channel should fill and broadcast should not block.
	_, cancel := b.Subscribe()
	defer cancel()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 200; i++ {
			b.Broadcast(Event{Type: "flood", Data: "{}"})
		}
		close(done)
	}()

	select {
	case <-done:
		// Broadcast completed without blocking — good.
	case <-time.After(2 * time.Second):
		t.Fatal("broadcast blocked on slow subscriber")
	}
}
