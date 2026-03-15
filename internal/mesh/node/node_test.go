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
