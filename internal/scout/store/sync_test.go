package store

import "testing"

func TestAddSyncResult(t *testing.T) {
	s := newTestStore(t)
	sr := SyncResult{
		Platform: "linkedin",
		Status:   "ok",
		Issues:   "0",
		Details:  "all synced",
	}
	id, err := s.AddSyncResult(sr)
	if err != nil {
		t.Fatalf("AddSyncResult: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}
}

func TestGetSyncMeta_NotFound(t *testing.T) {
	s := newTestStore(t)
	val, err := s.GetSyncMeta("nonexistent-key")
	if err != nil {
		t.Fatalf("GetSyncMeta: %v", err)
	}
	if val != "" {
		t.Errorf("expected empty string for missing key, got %q", val)
	}
}

func TestSetAndGetSyncMeta(t *testing.T) {
	s := newTestStore(t)
	if err := s.SetSyncMeta("last_sync", "2026-03-30T01:00:00Z"); err != nil {
		t.Fatalf("SetSyncMeta: %v", err)
	}

	val, err := s.GetSyncMeta("last_sync")
	if err != nil {
		t.Fatalf("GetSyncMeta: %v", err)
	}
	if val != "2026-03-30T01:00:00Z" {
		t.Errorf("val = %q, want 2026-03-30T01:00:00Z", val)
	}
}

func TestSetSyncMeta_Upsert(t *testing.T) {
	s := newTestStore(t)
	s.SetSyncMeta("cursor", "v1")
	s.SetSyncMeta("cursor", "v2")

	val, err := s.GetSyncMeta("cursor")
	if err != nil {
		t.Fatalf("GetSyncMeta: %v", err)
	}
	if val != "v2" {
		t.Errorf("val = %q, want v2 after upsert", val)
	}
}
