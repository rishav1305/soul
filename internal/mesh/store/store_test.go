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

func TestValidateLinkingCode_NotFound(t *testing.T) {
	s := newTestStore(t)
	lc, err := s.ValidateLinkingCode("NONEXISTENT")
	if err != nil {
		t.Fatalf("ValidateLinkingCode: %v", err)
	}
	if lc != nil {
		t.Errorf("expected nil for nonexistent code, got %+v", lc)
	}
}

// --- Peer CRUD tests ---

func TestUpsertPeer(t *testing.T) {
	s := newTestStore(t)

	p := Peer{
		PeerID:   "peer-001",
		LastSeen: time.Now().UTC().Format(time.RFC3339),
		Host:     "192.168.1.10",
		Port:     9100,
		IsHub:    true,
	}
	if err := s.UpsertPeer(p); err != nil {
		t.Fatalf("UpsertPeer: %v", err)
	}

	peers, err := s.ListPeers()
	if err != nil {
		t.Fatalf("ListPeers: %v", err)
	}
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers))
	}
	if peers[0].PeerID != "peer-001" {
		t.Errorf("PeerID = %q, want %q", peers[0].PeerID, "peer-001")
	}
	if peers[0].Host != "192.168.1.10" {
		t.Errorf("Host = %q, want %q", peers[0].Host, "192.168.1.10")
	}
	if peers[0].Port != 9100 {
		t.Errorf("Port = %d, want 9100", peers[0].Port)
	}
	if !peers[0].IsHub {
		t.Error("expected IsHub to be true")
	}
}

func TestUpsertPeer_Update(t *testing.T) {
	s := newTestStore(t)

	p := Peer{
		PeerID: "peer-002",
		Host:   "192.168.1.20",
		Port:   9100,
		IsHub:  false,
	}
	if err := s.UpsertPeer(p); err != nil {
		t.Fatalf("UpsertPeer (create): %v", err)
	}

	// Update the same peer
	p.Host = "192.168.1.30"
	p.Port = 9200
	p.IsHub = true
	if err := s.UpsertPeer(p); err != nil {
		t.Fatalf("UpsertPeer (update): %v", err)
	}

	peers, err := s.ListPeers()
	if err != nil {
		t.Fatalf("ListPeers: %v", err)
	}
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer after upsert, got %d", len(peers))
	}
	if peers[0].Host != "192.168.1.30" {
		t.Errorf("Host after update = %q, want %q", peers[0].Host, "192.168.1.30")
	}
	if peers[0].Port != 9200 {
		t.Errorf("Port after update = %d, want 9200", peers[0].Port)
	}
	if !peers[0].IsHub {
		t.Error("expected IsHub to be true after update")
	}
}

func TestListPeers_Empty(t *testing.T) {
	s := newTestStore(t)
	peers, err := s.ListPeers()
	if err != nil {
		t.Fatalf("ListPeers: %v", err)
	}
	if peers != nil && len(peers) != 0 {
		t.Errorf("expected empty peers list, got %d", len(peers))
	}
}

func TestListPeers_Multiple(t *testing.T) {
	s := newTestStore(t)
	for i, id := range []string{"alpha", "beta", "gamma"} {
		p := Peer{
			PeerID: id,
			Host:   "192.168.1." + string(rune('1'+i)),
			Port:   9100 + i,
			IsHub:  i == 0,
		}
		if err := s.UpsertPeer(p); err != nil {
			t.Fatalf("UpsertPeer(%s): %v", id, err)
		}
	}

	peers, err := s.ListPeers()
	if err != nil {
		t.Fatalf("ListPeers: %v", err)
	}
	if len(peers) != 3 {
		t.Errorf("expected 3 peers, got %d", len(peers))
	}
	// Should be ordered by peer_id
	if peers[0].PeerID != "alpha" {
		t.Errorf("first peer = %q, want alpha", peers[0].PeerID)
	}
}

// --- Additional heartbeat edge cases ---

func TestUpdateHeartbeat_NodeNotRegistered(t *testing.T) {
	s := newTestStore(t)
	hb := Heartbeat{CPUUsagePercent: 50.0}
	// UpdateHeartbeat for a node that doesn't exist — should fail due to FK constraint.
	err := s.UpdateHeartbeat("nonexistent-node", hb)
	if err == nil {
		t.Error("expected error for heartbeat on nonexistent node (FK constraint)")
	}
}

func TestGetRecentHeartbeats_NoHeartbeats(t *testing.T) {
	s := newTestStore(t)
	hbs, err := s.GetRecentHeartbeats("no-heartbeats-node", 10)
	if err != nil {
		t.Fatalf("GetRecentHeartbeats: %v", err)
	}
	if hbs != nil && len(hbs) != 0 {
		t.Errorf("expected empty heartbeats, got %d", len(hbs))
	}
}

func TestRegisterNode_Upsert(t *testing.T) {
	s := newTestStore(t)
	n := Node{ID: "node-upsert", Name: "original", Host: "10.0.0.1", Port: 9100}
	if err := s.RegisterNode(n); err != nil {
		t.Fatalf("RegisterNode: %v", err)
	}

	// Re-register with different data
	n.Name = "updated"
	n.Host = "10.0.0.2"
	if err := s.RegisterNode(n); err != nil {
		t.Fatalf("RegisterNode (upsert): %v", err)
	}

	got, err := s.GetNode("node-upsert")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got.Name != "updated" {
		t.Errorf("Name = %q, want updated", got.Name)
	}
	if got.Host != "10.0.0.2" {
		t.Errorf("Host = %q, want 10.0.0.2", got.Host)
	}
}

func TestCreateLinkingCode_Duplicate(t *testing.T) {
	s := newTestStore(t)
	expires := time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339)
	if err := s.CreateLinkingCode("DUP1", "node-1", "acct-1", expires); err != nil {
		t.Fatalf("CreateLinkingCode: %v", err)
	}
	// Duplicate code should fail (UNIQUE constraint)
	err := s.CreateLinkingCode("DUP1", "node-2", "acct-2", expires)
	if err == nil {
		t.Error("expected error for duplicate linking code")
	}
}

func TestClose_DoubleClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mesh_close.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Second close should not panic (may return error)
	_ = s.Close()
}
