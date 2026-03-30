package ws

import "testing"

func TestReplayBuffer_StoreAndReplay(t *testing.T) {
	rb := NewReplayBuffer(100, 50)

	rb.Store("session-1", "msg-1", []byte(`{"type":"chat.token","text":"hello"}`))
	rb.Store("session-1", "msg-2", []byte(`{"type":"chat.token","text":"world"}`))

	msgs, found := rb.Replay("session-1", "msg-1")
	if !found {
		t.Fatal("expected found=true")
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message after msg-1, got %d", len(msgs))
	}
	if string(msgs[0]) != `{"type":"chat.token","text":"world"}` {
		t.Errorf("unexpected: %s", msgs[0])
	}
}

func TestReplayBuffer_EmptyReplay(t *testing.T) {
	rb := NewReplayBuffer(100, 50)
	msgs, found := rb.Replay("session-1", "msg-nonexistent")
	if found {
		t.Fatal("expected found=false for unknown session")
	}
	if len(msgs) != 0 {
		t.Fatalf("expected 0 for unknown ID, got %d", len(msgs))
	}
}

func TestReplayBuffer_CaughtUp(t *testing.T) {
	rb := NewReplayBuffer(100, 50)
	rb.Store("s1", "m1", []byte("1"))

	msgs, found := rb.Replay("s1", "m1")
	if !found {
		t.Fatal("expected found=true (anchor exists)")
	}
	if len(msgs) != 0 {
		t.Fatalf("expected 0 msgs (caught up), got %d", len(msgs))
	}
}

func TestReplayBuffer_CapacityEviction(t *testing.T) {
	rb := NewReplayBuffer(3, 2)

	rb.Store("s1", "m1", []byte("1"))
	rb.Store("s1", "m2", []byte("2"))
	rb.Store("s1", "m3", []byte("3"))
	rb.Store("s1", "m4", []byte("4")) // evicts m1

	msgs, found := rb.Replay("s1", "m1")
	if found {
		t.Fatal("expected found=false (anchor evicted)")
	}
	if len(msgs) != 0 {
		t.Fatalf("expected 0 (anchor evicted), got %d", len(msgs))
	}

	msgs, found = rb.Replay("s1", "m2")
	if !found {
		t.Fatal("expected found=true for m2")
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 after m2, got %d", len(msgs))
	}
}

func TestReplayBuffer_SessionEviction(t *testing.T) {
	rb := NewReplayBuffer(10, 2)

	rb.Store("s1", "m1", []byte("1"))
	rb.Store("s2", "m1", []byte("2"))
	rb.Store("s3", "m1", []byte("3")) // evicts s1

	_, found := rb.Replay("s1", "m1")
	if found {
		t.Fatal("expected found=false (session evicted)")
	}
}

func TestReplayBuffer_Clear(t *testing.T) {
	rb := NewReplayBuffer(100, 50)

	rb.Store("s1", "m1", []byte("data1"))
	rb.Store("s1", "m2", []byte("data2"))
	rb.Store("s2", "m1", []byte("other"))

	// Verify s1 exists before clear.
	msgs, found := rb.Replay("s1", "m1")
	if !found {
		t.Fatal("expected s1 to exist before Clear")
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(msgs))
	}

	// Clear s1.
	rb.Clear("s1")

	// Verify s1 is gone.
	_, found = rb.Replay("s1", "m1")
	if found {
		t.Fatal("expected s1 to be gone after Clear")
	}

	// Verify s2 still exists.
	_, found = rb.Replay("s2", "m1")
	if !found {
		t.Fatal("expected s2 to still exist after clearing s1")
	}
}

func TestReplayBuffer_ClearNonexistent(t *testing.T) {
	rb := NewReplayBuffer(100, 50)
	// Clearing a session that doesn't exist should not panic.
	rb.Clear("nonexistent")
}

func TestReplayBuffer_ReplayEmptyAnchor(t *testing.T) {
	rb := NewReplayBuffer(100, 50)
	rb.Store("s1", "m1", []byte("data"))

	// Empty afterMessageID should return found=false.
	_, found := rb.Replay("s1", "")
	if found {
		t.Fatal("expected found=false for empty afterMessageID")
	}
}

func TestReplayBuffer_StoreUpdatesLRU(t *testing.T) {
	rb := NewReplayBuffer(10, 2)

	rb.Store("s1", "m1", []byte("1"))
	rb.Store("s2", "m1", []byte("2"))

	// Touch s1 again — should move it to end of LRU.
	rb.Store("s1", "m2", []byte("3"))

	// Adding s3 should now evict s2 (oldest), not s1.
	rb.Store("s3", "m1", []byte("4"))

	// s1 should still exist.
	_, found := rb.Replay("s1", "m1")
	if !found {
		t.Fatal("expected s1 to survive LRU (was recently touched)")
	}

	// s2 should be evicted.
	_, found = rb.Replay("s2", "m1")
	if found {
		t.Fatal("expected s2 to be evicted (oldest in LRU)")
	}
}
