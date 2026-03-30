package metrics

import (
	"testing"
	"time"
)

func TestConnectionHealth_DropRate(t *testing.T) {
	ch := NewConnectionHealth(1 * time.Hour)

	for i := 0; i < 100; i++ {
		ch.RecordConnect()
	}
	for i := 0; i < 99; i++ {
		ch.RecordDisconnect("normal")
	}
	ch.RecordDisconnect("network")

	rate := ch.DropRate()
	if rate < 0.009 || rate > 0.011 {
		t.Errorf("expected drop rate ~0.01, got %f", rate)
	}
}

func TestConnectionHealth_ReconnectLatency(t *testing.T) {
	ch := NewConnectionHealth(1 * time.Hour)

	ch.RecordReconnectLatency(100 * time.Millisecond)
	ch.RecordReconnectLatency(200 * time.Millisecond)
	ch.RecordReconnectLatency(3000 * time.Millisecond)

	p50 := ch.ReconnectP50()
	p95 := ch.ReconnectP95()

	if p50 < 100*time.Millisecond || p50 > 300*time.Millisecond {
		t.Errorf("expected p50 ~200ms, got %v", p50)
	}
	if p95 < 2*time.Second {
		t.Errorf("expected p95 >= 2s, got %v", p95)
	}
}

func TestConnectionHealth_ReconnectSuccessRate(t *testing.T) {
	ch := NewConnectionHealth(1 * time.Hour)

	ch.RecordReconnectAttempt(true)
	ch.RecordReconnectAttempt(true)
	ch.RecordReconnectAttempt(false)

	rate := ch.ReconnectSuccessRate()
	if rate < 0.65 || rate > 0.68 {
		t.Errorf("expected success rate ~0.667, got %f", rate)
	}
}

func TestConnectionHealth_WindowExpiry(t *testing.T) {
	ch := NewConnectionHealth(100 * time.Millisecond)

	ch.RecordConnect()
	ch.RecordDisconnect("network")

	time.Sleep(150 * time.Millisecond)

	rate := ch.DropRate()
	if rate != 0 {
		t.Errorf("expected 0 after window expiry, got %f", rate)
	}
}

func TestConnectionHealth_Prune(t *testing.T) {
	ch := NewConnectionHealth(50 * time.Millisecond)

	// Record some events.
	ch.RecordConnect()
	ch.RecordDisconnect("network")
	ch.RecordReconnectAttempt(true)
	ch.RecordReconnectLatency(100 * time.Millisecond)

	// Wait past 2x window (100ms).
	time.Sleep(120 * time.Millisecond)

	// Add fresh events.
	ch.RecordConnect()

	ch.Prune()

	// After pruning, old events should be gone. Only the fresh connect remains.
	ch.mu.Lock()
	connectCount := len(ch.connects)
	disconnectCount := len(ch.disconnects)
	reconnectCount := len(ch.reconnects)
	ch.mu.Unlock()

	if connectCount != 1 {
		t.Errorf("expected 1 connect after prune, got %d", connectCount)
	}
	if disconnectCount != 0 {
		t.Errorf("expected 0 disconnects after prune, got %d", disconnectCount)
	}
	if reconnectCount != 0 {
		t.Errorf("expected 0 reconnects after prune, got %d", reconnectCount)
	}
}
