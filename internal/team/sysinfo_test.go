package team

import (
	"testing"
)

func TestParseMeminfo(t *testing.T) {
	content := `MemTotal:       16264860 kB
MemFree:         1234560 kB
MemAvailable:    8132430 kB
Buffers:          456789 kB
Cached:          6789012 kB
SwapTotal:       2097148 kB
`
	totalMB, usedMB := parseMeminfo(content)
	if totalMB == 0 {
		t.Error("expected non-zero totalMB")
	}
	if usedMB == 0 {
		t.Error("expected non-zero usedMB")
	}

	// MemTotal = 16264860 kB = ~15883 MB
	expectedTotal := 16264860 / 1024
	if totalMB != expectedTotal {
		t.Errorf("totalMB = %d, want %d", totalMB, expectedTotal)
	}

	// MemUsed = MemTotal - MemAvailable = (16264860 - 8132430) / 1024
	expectedUsed := (16264860 - 8132430) / 1024
	if usedMB != expectedUsed {
		t.Errorf("usedMB = %d, want %d", usedMB, expectedUsed)
	}
}

func TestParseMeminfo_Empty(t *testing.T) {
	totalMB, usedMB := parseMeminfo("")
	if totalMB != 0 || usedMB != 0 {
		t.Errorf("expected 0,0 for empty input, got %d,%d", totalMB, usedMB)
	}
}

func TestParseLoadAvg(t *testing.T) {
	content := "2.34 1.56 0.87 4/1234 56789"
	avg1, avg5, avg15 := parseLoadAvg(content)

	if avg1 != 2.34 {
		t.Errorf("avg1 = %f, want 2.34", avg1)
	}
	if avg5 != 1.56 {
		t.Errorf("avg5 = %f, want 1.56", avg5)
	}
	if avg15 != 0.87 {
		t.Errorf("avg15 = %f, want 0.87", avg15)
	}
}

func TestParseLoadAvg_Empty(t *testing.T) {
	avg1, avg5, avg15 := parseLoadAvg("")
	if avg1 != 0 || avg5 != 0 || avg15 != 0 {
		t.Errorf("expected 0,0,0 for empty input, got %f,%f,%f", avg1, avg5, avg15)
	}
}

func TestReadSystemResources(t *testing.T) {
	// This test runs on real Linux -- it should return non-zero values for
	// at least hostname and memory (loadavg and temp may vary).
	res := ReadSystemResources()

	if res.Hostname == "" {
		t.Error("expected non-empty hostname")
	}

	// On any Linux system, /proc/meminfo should be readable.
	if res.MemoryTotalMB == 0 {
		t.Error("expected non-zero MemoryTotalMB (are we on Linux?)")
	}
	if res.MemoryUsedMB == 0 {
		t.Error("expected non-zero MemoryUsedMB")
	}

	// Load average should be readable on Linux.
	// LoadAvg1 can be 0.00 on an idle system, so we just check it doesn't error.
	// (no assertion on value)
}

func TestListTmuxPanes_EmptySession(t *testing.T) {
	result := ListTmuxPanes("")
	if len(result) != 0 {
		t.Errorf("expected empty map for empty session, got %d entries", len(result))
	}
}

func TestReadTokenUsage(t *testing.T) {
	// This test may return nil if guardian.db doesn't exist -- that's fine.
	// We're just verifying it doesn't panic.
	result := ReadTokenUsage()
	_ = result // nil is acceptable
}
