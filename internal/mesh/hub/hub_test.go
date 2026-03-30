package hub

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul/internal/mesh/store"
)

func newTestHub(t *testing.T) (*Hub, *store.Store) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "mesh.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return New(s), s
}

func TestNew(t *testing.T) {
	h, _ := newTestHub(t)
	if h == nil {
		t.Fatal("expected non-nil Hub")
	}
}

func TestHandleHeartbeat(t *testing.T) {
	h, s := newTestHub(t)

	// Register a node first.
	n := store.Node{
		ID:       "node-1",
		Name:     "alpha",
		Host:     "192.168.1.1",
		Port:     3024,
		Role:     "agent",
		Platform: "linux",
		Arch:     "amd64",
		CPUCores: 4,
		Status:   "online",
	}
	if err := s.RegisterNode(n); err != nil {
		t.Fatalf("RegisterNode: %v", err)
	}

	payload, _ := json.Marshal(HeartbeatPayload{
		CPUUsagePercent: 45.2,
		CPULoad1m:       1.5,
		RAMAvailableMB:  4096,
		RAMUsedPercent:  62.0,
		StorageFreeGB:   100,
	})

	if err := h.HandleHeartbeat("node-1", payload); err != nil {
		t.Fatalf("HandleHeartbeat: %v", err)
	}

	// Verify heartbeat was recorded.
	hbs, err := s.GetRecentHeartbeats("node-1", 10)
	if err != nil {
		t.Fatalf("GetRecentHeartbeats: %v", err)
	}
	if len(hbs) != 1 {
		t.Fatalf("expected 1 heartbeat, got %d", len(hbs))
	}
	if hbs[0].CPUUsagePercent != 45.2 {
		t.Errorf("CPUUsagePercent = %f, want 45.2", hbs[0].CPUUsagePercent)
	}
}

func TestHandleHeartbeat_InvalidPayload(t *testing.T) {
	h, _ := newTestHub(t)
	err := h.HandleHeartbeat("node-1", json.RawMessage(`not-json`))
	if err == nil {
		t.Fatal("expected error for invalid payload")
	}
}

func TestAggregateResources_Empty(t *testing.T) {
	h, _ := newTestHub(t)

	res, err := h.AggregateResources()
	if err != nil {
		t.Fatalf("AggregateResources: %v", err)
	}
	if res.NodeCount != 0 {
		t.Errorf("NodeCount = %d, want 0", res.NodeCount)
	}
	if res.OnlineCount != 0 {
		t.Errorf("OnlineCount = %d, want 0", res.OnlineCount)
	}
}

func TestAggregateResources_SumsCorrectly(t *testing.T) {
	h, s := newTestHub(t)

	nodes := []store.Node{
		{ID: "n1", Name: "a", CPUCores: 4, RAMTotalMB: 8192, StorageTotalGB: 256, Status: "online"},
		{ID: "n2", Name: "b", CPUCores: 8, RAMTotalMB: 16384, StorageTotalGB: 512, Status: "online"},
		{ID: "n3", Name: "c", CPUCores: 2, RAMTotalMB: 4096, StorageTotalGB: 128, Status: "offline"},
	}
	for _, n := range nodes {
		if err := s.RegisterNode(n); err != nil {
			t.Fatalf("RegisterNode(%s): %v", n.ID, err)
		}
	}

	res, err := h.AggregateResources()
	if err != nil {
		t.Fatalf("AggregateResources: %v", err)
	}
	if res.NodeCount != 3 {
		t.Errorf("NodeCount = %d, want 3", res.NodeCount)
	}
	if res.TotalCPUCores != 14 {
		t.Errorf("TotalCPUCores = %d, want 14", res.TotalCPUCores)
	}
	if res.TotalRAMMB != 28672 {
		t.Errorf("TotalRAMMB = %d, want 28672", res.TotalRAMMB)
	}
	if res.TotalStorageGB != 896 {
		t.Errorf("TotalStorageGB = %d, want 896", res.TotalStorageGB)
	}
	if res.OnlineCount != 2 {
		t.Errorf("OnlineCount = %d, want 2", res.OnlineCount)
	}
}
