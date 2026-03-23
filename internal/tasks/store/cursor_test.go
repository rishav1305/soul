package store

import (
	"testing"
	"time"
)

func TestCursorRoundTrip(t *testing.T) {
	now := time.Now().Unix()
	token := EncodeCursor(42, now)
	seq, ts, err := DecodeCursor(token)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if seq != 42 {
		t.Errorf("seq = %d, want 42", seq)
	}
	if ts != now {
		t.Errorf("ts = %d, want %d", ts, now)
	}
}

func TestDecodeCursorEmpty(t *testing.T) {
	seq, ts, err := DecodeCursor("")
	if err != nil {
		t.Fatalf("decode empty: %v", err)
	}
	if seq != 0 || ts != 0 {
		t.Errorf("expected zero values for empty cursor, got seq=%d ts=%d", seq, ts)
	}
}

func TestDecodeCursorInvalid(t *testing.T) {
	_, _, err := DecodeCursor("not-base64-json")
	if err == nil {
		t.Error("expected error for invalid cursor")
	}
}
