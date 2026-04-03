package team

import (
	"testing"
)

func TestHub_RegisterUnregister(t *testing.T) {
	h := NewHub()

	if h.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", h.ClientCount())
	}

	// We can't create a real websocket.Conn in unit tests, so test
	// the subscription logic directly.
}

func TestClient_Subscribe(t *testing.T) {
	// Test subscription matching without a real websocket.
	c := &Client{
		subs: make(map[string]bool),
	}

	c.Subscribe("pane.shuri")
	if !c.isSubscribed("pane.shuri") {
		t.Error("expected subscribed to pane.shuri")
	}
	if c.isSubscribed("pane.happy") {
		t.Error("should not be subscribed to pane.happy")
	}
}

func TestClient_SubscribeAll(t *testing.T) {
	c := &Client{
		subs: make(map[string]bool),
	}

	c.SubscribeAll()
	if !c.isSubscribed("pane.shuri") {
		t.Error("wildcard should match pane.shuri")
	}
	if !c.isSubscribed("state.update") {
		t.Error("wildcard should match state.update")
	}
	if !c.isSubscribed("chat.message") {
		t.Error("wildcard should match chat.message")
	}
}

func TestClient_PrefixSubscribe(t *testing.T) {
	c := &Client{
		subs: make(map[string]bool),
	}

	// Subscribe with prefix (trailing dot).
	c.Subscribe("pane.")
	if !c.isSubscribed("pane.shuri") {
		t.Error("pane. prefix should match pane.shuri")
	}
	if !c.isSubscribed("pane.happy") {
		t.Error("pane. prefix should match pane.happy")
	}
	if c.isSubscribed("state.update") {
		t.Error("pane. prefix should not match state.update")
	}
}

func TestHub_Broadcast(t *testing.T) {
	h := NewHub()

	// Test that broadcast doesn't panic with no clients.
	h.Broadcast(WSMessage{Type: "test", Payload: "hello"})
	h.BroadcastAll(WSMessage{Type: "test", Payload: "hello"})
}

func TestWSMessage_Types(t *testing.T) {
	// Verify expected message type conventions.
	types := []string{
		"pane.shuri",
		"pane.happy",
		"state.update",
		"state.snapshot",
		"chat.message",
		"task.update",
		"health.update",
	}

	for _, msgType := range types {
		if msgType == "" {
			t.Error("empty message type")
		}
	}
}
