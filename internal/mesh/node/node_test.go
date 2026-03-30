package node

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCapabilityScore(t *testing.T) {
	tests := []struct {
		name    string
		info    NodeInfo
		wantMin int
		wantMax int
	}{
		{
			name:    "max score 16GB+500GB",
			info:    NodeInfo{RAMTotalMB: 16384, StorageTotalGB: 500},
			wantMin: 60,
			wantMax: 60,
		},
		{
			name:    "over max still caps at 60",
			info:    NodeInfo{RAMTotalMB: 32768, StorageTotalGB: 1000},
			wantMin: 60,
			wantMax: 60,
		},
		{
			name:    "low spec 2GB+50GB",
			info:    NodeInfo{RAMTotalMB: 2048, StorageTotalGB: 50},
			wantMin: 1,
			wantMax: 10,
		},
		{
			name:    "zero resources",
			info:    NodeInfo{RAMTotalMB: 0, StorageTotalGB: 0},
			wantMin: 0,
			wantMax: 0,
		},
		{
			name:    "8GB RAM only",
			info:    NodeInfo{RAMTotalMB: 8192, StorageTotalGB: 0},
			wantMin: 20,
			wantMax: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CapabilityScore(tt.info)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("CapabilityScore(%+v) = %d, want [%d, %d]", tt.info, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestLoadOrCreateID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "node-id")

	id1, err := LoadOrCreateID(path)
	if err != nil {
		t.Fatalf("LoadOrCreateID: %v", err)
	}
	if id1 == "" {
		t.Fatal("expected non-empty ID")
	}

	// Reading again should return the same ID.
	id2, err := LoadOrCreateID(path)
	if err != nil {
		t.Fatalf("LoadOrCreateID (second call): %v", err)
	}
	if id1 != id2 {
		t.Errorf("IDs differ: %q vs %q", id1, id2)
	}
}

func TestLoadOrCreateID_Existing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "node-id")
	existing := "pre-existing-uuid-1234"
	if err := os.WriteFile(path, []byte(existing+"\n"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := LoadOrCreateID(path)
	if err != nil {
		t.Fatalf("LoadOrCreateID: %v", err)
	}
	if got != existing {
		t.Errorf("ID = %q, want %q", got, existing)
	}
}

func TestLoadOrCreateID_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "node-id")
	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	id, err := LoadOrCreateID(path)
	if err != nil {
		t.Fatalf("LoadOrCreateID: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID for empty file")
	}
}

func TestLoadOrCreateID_WhitespaceFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "node-id")
	if err := os.WriteFile(path, []byte("  \n  \n"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	id, err := LoadOrCreateID(path)
	if err != nil {
		t.Fatalf("LoadOrCreateID: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID for whitespace-only file")
	}
}

func TestLoadOrCreateID_UnwritablePath(t *testing.T) {
	// Try a path that doesn't exist and can't be created
	_, err := LoadOrCreateID("/proc/nonexistent/node-id")
	if err == nil {
		t.Error("expected error for unwritable path")
	}
}

func TestSystemSnapshot(t *testing.T) {
	info, err := SystemSnapshot()
	if err != nil {
		t.Fatalf("SystemSnapshot: %v", err)
	}

	if info.Platform == "" {
		t.Error("Platform should not be empty")
	}
	if info.Arch == "" {
		t.Error("Arch should not be empty")
	}
	if info.CPUCores <= 0 {
		t.Errorf("CPUCores = %d, should be > 0", info.CPUCores)
	}
	if info.Status != "online" {
		t.Errorf("Status = %q, want online", info.Status)
	}
	// Name should be set from hostname
	if info.Name == "" {
		t.Error("Name (hostname) should not be empty")
	}
}

func TestSystemSnapshot_HasRAM(t *testing.T) {
	if _, err := os.Stat("/proc/meminfo"); os.IsNotExist(err) {
		t.Skip("no /proc/meminfo — not Linux")
	}

	info, err := SystemSnapshot()
	if err != nil {
		t.Fatalf("SystemSnapshot: %v", err)
	}
	if info.RAMTotalMB <= 0 {
		t.Errorf("RAMTotalMB = %d, should be > 0 on Linux", info.RAMTotalMB)
	}
}

func TestSystemSnapshot_HasStorage(t *testing.T) {
	info, err := SystemSnapshot()
	if err != nil {
		t.Fatalf("SystemSnapshot: %v", err)
	}
	// df should work on any Unix system
	if info.StorageTotalGB <= 0 {
		t.Errorf("StorageTotalGB = %d, should be > 0", info.StorageTotalGB)
	}
}

func TestCapabilityScore_NegativeInputs(t *testing.T) {
	// Negative values should clamp to 0
	score := CapabilityScore(NodeInfo{RAMTotalMB: -1000, StorageTotalGB: -100})
	if score != 0 {
		t.Errorf("CapabilityScore with negatives = %d, want 0", score)
	}
}

func TestCapabilityScore_MidRange(t *testing.T) {
	// 4GB RAM + 250GB storage
	score := CapabilityScore(NodeInfo{RAMTotalMB: 4096, StorageTotalGB: 250})
	// RAM: 4096*40/16384 = 10, Storage: 250*20/500 = 10
	if score != 20 {
		t.Errorf("CapabilityScore = %d, want 20", score)
	}
}

func TestNodeInfo_ZeroValue(t *testing.T) {
	var info NodeInfo
	if info.ID != "" || info.Name != "" || info.Host != "" {
		t.Error("zero-value NodeInfo should have empty strings")
	}
	if info.Port != 0 || info.CPUCores != 0 {
		t.Error("zero-value NodeInfo should have zero ints")
	}
	if info.IsHub {
		t.Error("zero-value NodeInfo.IsHub should be false")
	}
}
