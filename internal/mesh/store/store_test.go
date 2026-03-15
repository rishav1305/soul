package store

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mesh_test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestRegisterNode(t *testing.T) {
	s := newTestStore(t)
	n := Node{
		ID:             "node-1",
		Name:           "titan-pc",
		Host:           "192.168.1.10",
		Port:           9100,
		Role:           "hub",
		Platform:       "linux",
		Arch:           "amd64",
		CPUCores:       8,
		RAMTotalMB:     16384,
		StorageTotalGB: 512,
		Status:         "online",
	}
	if err := s.RegisterNode(n); err != nil {
		t.Fatalf("RegisterNode: %v", err)
	}

	got, err := s.GetNode("node-1")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got == nil {
		t.Fatal("expected node, got nil")
	}
	if got.Name != "titan-pc" {
		t.Errorf("Name = %q, want %q", got.Name, "titan-pc")
	}
	if got.CPUCores != 8 {
		t.Errorf("CPUCores = %d, want 8", got.CPUCores)
	}
}

func TestGetNode(t *testing.T) {
	s := newTestStore(t)

	got, err := s.GetNode("nonexistent")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for nonexistent node, got %+v", got)
	}
}

func TestListNodes(t *testing.T) {
	s := newTestStore(t)
	for _, name := range []string{"alpha", "beta", "gamma"} {
		if err := s.RegisterNode(Node{ID: name, Name: name, Host: "127.0.0.1", Port: 9100}); err != nil {
			t.Fatalf("RegisterNode(%s): %v", name, err)
		}
	}

	nodes, err := s.ListNodes()
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(nodes) != 3 {
		t.Errorf("len(nodes) = %d, want 3", len(nodes))
	}
}

func TestUpdateHeartbeat(t *testing.T) {
	s := newTestStore(t)
	if err := s.RegisterNode(Node{ID: "node-hb", Name: "hb-test", Host: "127.0.0.1", Port: 9100}); err != nil {
		t.Fatalf("RegisterNode: %v", err)
	}

	hb := Heartbeat{
		CPUUsagePercent: 45.2,
		CPULoad1m:       1.5,
		RAMAvailableMB:  8192,
		RAMUsedPercent:  50.0,
		StorageFreeGB:   200,
	}
	if err := s.UpdateHeartbeat("node-hb", hb); err != nil {
		t.Fatalf("UpdateHeartbeat: %v", err)
	}

	node, err := s.GetNode("node-hb")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if node.Status != "online" {
		t.Errorf("Status = %q, want %q", node.Status, "online")
	}
	if node.LastHeartbeat == "" {
		t.Error("expected LastHeartbeat to be set")
	}
}

func TestGetRecentHeartbeats(t *testing.T) {
	s := newTestStore(t)
	if err := s.RegisterNode(Node{ID: "node-rh", Name: "rh-test", Host: "127.0.0.1", Port: 9100}); err != nil {
		t.Fatalf("RegisterNode: %v", err)
	}

	for i := 0; i < 5; i++ {
		hb := Heartbeat{
			CPUUsagePercent: float64(i * 10),
			Timestamp:       time.Now().UTC().Add(time.Duration(i) * time.Minute).Format(time.RFC3339),
		}
		if err := s.UpdateHeartbeat("node-rh", hb); err != nil {
			t.Fatalf("UpdateHeartbeat[%d]: %v", i, err)
		}
	}

	hbs, err := s.GetRecentHeartbeats("node-rh", 3)
	if err != nil {
		t.Fatalf("GetRecentHeartbeats: %v", err)
	}
	if len(hbs) != 3 {
		t.Errorf("len(heartbeats) = %d, want 3", len(hbs))
	}
	// Most recent first.
	if hbs[0].CPUUsagePercent != 40.0 {
		t.Errorf("first heartbeat CPU = %f, want 40.0", hbs[0].CPUUsagePercent)
	}
}

func TestCreateLinkingCode(t *testing.T) {
	s := newTestStore(t)
	expires := time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339)
	if err := s.CreateLinkingCode("ABC123", "node-1", "acct-1", expires); err != nil {
		t.Fatalf("CreateLinkingCode: %v", err)
	}

	lc, err := s.ValidateLinkingCode("ABC123")
	if err != nil {
		t.Fatalf("ValidateLinkingCode: %v", err)
	}
	if lc == nil {
		t.Fatal("expected linking code, got nil")
	}
	if lc.NodeID != "node-1" {
		t.Errorf("NodeID = %q, want %q", lc.NodeID, "node-1")
	}
	if lc.AccountID != "acct-1" {
		t.Errorf("AccountID = %q, want %q", lc.AccountID, "acct-1")
	}
}

func TestValidateLinkingCode_Expired(t *testing.T) {
	s := newTestStore(t)
	expires := time.Now().UTC().Add(-5 * time.Minute).Format(time.RFC3339)
	if err := s.CreateLinkingCode("EXPIRED1", "node-1", "acct-1", expires); err != nil {
		t.Fatalf("CreateLinkingCode: %v", err)
	}

	lc, err := s.ValidateLinkingCode("EXPIRED1")
	if err != nil {
		t.Fatalf("ValidateLinkingCode: %v", err)
	}
	if lc != nil {
		t.Errorf("expected nil for expired code, got %+v", lc)
	}
}
