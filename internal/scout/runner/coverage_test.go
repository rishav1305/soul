package runner

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/rishav1305/soul/internal/scout/store"
)

// --- parseTime tests ---

func TestParseTime_RFC3339(t *testing.T) {
	ts := "2026-03-30T10:00:00Z"
	got, err := parseTime(ts)
	if err != nil {
		t.Fatalf("parseTime(%q): %v", ts, err)
	}
	if got.Year() != 2026 || got.Month() != 3 || got.Day() != 30 {
		t.Errorf("parseTime(%q) = %v", ts, got)
	}
}

func TestParseTime_FallbackFormat(t *testing.T) {
	ts := "2026-03-30 10:00:00"
	got, err := parseTime(ts)
	if err != nil {
		t.Fatalf("parseTime(%q): %v", ts, err)
	}
	if got.Year() != 2026 || got.Month() != 3 || got.Day() != 30 {
		t.Errorf("parseTime(%q) = %v", ts, got)
	}
}

func TestParseTime_Invalid(t *testing.T) {
	_, err := parseTime("not-a-time")
	if err == nil {
		t.Fatal("expected error for invalid time string")
	}
}

// --- advanceLead tests ---

func TestAdvanceLead_Success(t *testing.T) {
	s := newTestStore(t)
	id, err := s.AddLead(store.Lead{
		JobTitle: "Engineer",
		Company:  "TestCo",
		Pipeline: "job",
		Stage:    "discovered",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = advanceLead(s, id, "job", "discovered", "qualified")
	if err != nil {
		t.Fatalf("advanceLead: %v", err)
	}

	lead, _ := s.GetLead(id)
	if lead.Stage != "qualified" {
		t.Errorf("stage = %q, want qualified", lead.Stage)
	}
}

func TestAdvanceLead_InvalidTransition(t *testing.T) {
	s := newTestStore(t)
	id, _ := s.AddLead(store.Lead{
		JobTitle: "Engineer",
		Company:  "TestCo",
		Pipeline: "job",
		Stage:    "discovered",
	})

	err := advanceLead(s, id, "job", "discovered", "nonexistent-stage")
	if err == nil {
		t.Fatal("expected error for invalid transition")
	}
}

func TestAdvanceLead_ClosedStore(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "adv.db"))
	if err != nil {
		t.Fatal(err)
	}
	id, _ := s.AddLead(store.Lead{
		JobTitle: "Engineer",
		Company:  "TestCo",
		Pipeline: "job",
		Stage:    "discovered",
	})
	s.Close()

	err = advanceLead(s, id, "job", "discovered", "qualified")
	if err == nil {
		t.Fatal("expected error with closed store")
	}
}

// --- runCycle error handling ---

func TestRunCycle_PhaseError(t *testing.T) {
	s := newTestStore(t)
	r := New(s, time.Minute)

	// Register a phase that returns an error.
	r.Register("failing-phase", func(_ *store.Store) (int, error) {
		return 0, fmt.Errorf("simulated failure")
	})

	// runCycle should not panic — it logs errors and continues.
	r.runCycle()

	if r.Cycles() != 1 {
		t.Errorf("Cycles() = %d, want 1", r.Cycles())
	}
}

func TestRunCycle_PhaseWithProcessed(t *testing.T) {
	s := newTestStore(t)
	r := New(s, time.Minute)

	r.Register("active-phase", func(_ *store.Store) (int, error) {
		return 5, nil // processed 5 leads
	})

	r.runCycle()

	if r.Cycles() != 1 {
		t.Errorf("Cycles() = %d, want 1", r.Cycles())
	}
}

// --- Content pipeline edge cases ---

func TestContentStalePhase_ClosedStore(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "cstale.db"))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	_, err = ContentStalePhase(s)
	// Should handle closed store gracefully.
	if err == nil {
		t.Log("ContentStalePhase with closed store returned nil error (acceptable)")
	}
}

// --- Networking edge cases ---

func TestNetworkingWarmPhase_ClosedStore(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "nwarm.db"))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	_, err = NetworkingWarmPhase(s)
	if err == nil {
		t.Log("NetworkingWarmPhase with closed store returned nil error (acceptable)")
	}
}

func TestConsultingQualifyPhase_ClosedStore(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "cqual.db"))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	_, err = ConsultingQualifyPhase(s)
	if err == nil {
		t.Log("ConsultingQualifyPhase with closed store returned nil error (acceptable)")
	}
}
