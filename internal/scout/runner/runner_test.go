package runner

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/rishav1305/soul-v2/internal/scout/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestRunnerStartStop(t *testing.T) {
	s := newTestStore(t)
	r := New(s, 50*time.Millisecond)

	called := make(chan struct{}, 10)
	r.Register("test", func(_ *store.Store) (int, error) {
		select {
		case called <- struct{}{}:
		default:
		}
		return 0, nil
	})

	ctx := context.Background()
	r.Start(ctx)

	// Wait for at least one cycle to complete.
	select {
	case <-called:
		// Phase was called at least once.
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first cycle")
	}

	r.Stop()

	if r.Cycles() < 1 {
		t.Errorf("Cycles() = %d, want >= 1", r.Cycles())
	}

	// Verify Stop is idempotent.
	r.Stop()
}

func TestRunnerStartIdempotent(t *testing.T) {
	s := newTestStore(t)
	r := New(s, 50*time.Millisecond)
	r.Register("noop", func(_ *store.Store) (int, error) { return 0, nil })

	ctx := context.Background()
	r.Start(ctx)
	r.Start(ctx) // second Start should be a no-op

	time.Sleep(100 * time.Millisecond)
	r.Stop()

	if r.Cycles() < 1 {
		t.Errorf("Cycles() = %d, want >= 1", r.Cycles())
	}
}

func TestQualifyPhase(t *testing.T) {
	s := newTestStore(t)

	// Add a discovered lead with tier=1 and match_score=80 (above threshold).
	id, err := s.AddLead(store.Lead{
		JobTitle:   "Senior Go Developer",
		Company:    "Acme Corp",
		Pipeline:   "job",
		Stage:      "discovered",
		Tier:       1,
		MatchScore: 80,
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	n, err := QualifyPhase(s)
	if err != nil {
		t.Fatalf("QualifyPhase: %v", err)
	}
	if n != 1 {
		t.Errorf("processed = %d, want 1", n)
	}

	lead, err := s.GetLead(id)
	if err != nil {
		t.Fatalf("GetLead: %v", err)
	}
	if lead.Stage != "qualified" {
		t.Errorf("Stage = %q, want %q", lead.Stage, "qualified")
	}

	// Verify stage history was recorded.
	history, err := s.GetStageHistory(id)
	if err != nil {
		t.Fatalf("GetStageHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("history len = %d, want 1", len(history))
	}
	if history[0].FromStage != "discovered" || history[0].ToStage != "qualified" {
		t.Errorf("history = %s -> %s, want discovered -> qualified", history[0].FromStage, history[0].ToStage)
	}
}

func TestQualifySkip(t *testing.T) {
	s := newTestStore(t)

	// Add a discovered lead with tier=2 and match_score=50 (below threshold).
	id, err := s.AddLead(store.Lead{
		JobTitle:   "Junior Python Developer",
		Company:    "SmallCo",
		Pipeline:   "job",
		Stage:      "discovered",
		Tier:       2,
		MatchScore: 50,
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	n, err := QualifyPhase(s)
	if err != nil {
		t.Fatalf("QualifyPhase: %v", err)
	}
	if n != 1 {
		t.Errorf("processed = %d, want 1", n)
	}

	lead, err := s.GetLead(id)
	if err != nil {
		t.Fatalf("GetLead: %v", err)
	}
	if lead.Stage != "skipped" {
		t.Errorf("Stage = %q, want %q", lead.Stage, "skipped")
	}
}

func TestQualifyUnscoredSkipped(t *testing.T) {
	s := newTestStore(t)

	// Add a discovered lead with no score (match_score=0).
	id, err := s.AddLead(store.Lead{
		JobTitle:   "ML Engineer",
		Company:    "DataCorp",
		Pipeline:   "job",
		Stage:      "discovered",
		Tier:       1,
		MatchScore: 0,
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	n, err := QualifyPhase(s)
	if err != nil {
		t.Fatalf("QualifyPhase: %v", err)
	}
	if n != 0 {
		t.Errorf("processed = %d, want 0 (unscored should be skipped)", n)
	}

	lead, err := s.GetLead(id)
	if err != nil {
		t.Fatalf("GetLead: %v", err)
	}
	if lead.Stage != "discovered" {
		t.Errorf("Stage = %q, want %q (should remain discovered)", lead.Stage, "discovered")
	}
}

func TestQualifyTier3Excluded(t *testing.T) {
	s := newTestStore(t)

	// Add a discovered lead with tier=3 -- should not be picked up.
	id, err := s.AddLead(store.Lead{
		JobTitle:   "Support Engineer",
		Company:    "BigCo",
		Pipeline:   "job",
		Stage:      "discovered",
		Tier:       3,
		MatchScore: 90,
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	n, err := QualifyPhase(s)
	if err != nil {
		t.Fatalf("QualifyPhase: %v", err)
	}
	if n != 0 {
		t.Errorf("processed = %d, want 0 (tier 3 excluded)", n)
	}

	lead, err := s.GetLead(id)
	if err != nil {
		t.Fatalf("GetLead: %v", err)
	}
	if lead.Stage != "discovered" {
		t.Errorf("Stage = %q, want %q", lead.Stage, "discovered")
	}
}

func TestStalePhase(t *testing.T) {
	s := newTestStore(t)

	// Add a lead at 'preparing' with updated_at = 15 days ago (> 14 day auto-skip).
	staleTime := time.Now().UTC().Add(-15 * 24 * time.Hour).Format(time.RFC3339)
	id, err := s.AddLead(store.Lead{
		JobTitle:  "Backend Engineer",
		Company:   "OldCo",
		Pipeline:  "job",
		Stage:     "preparing",
		UpdatedAt: staleTime,
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}
	// Force updated_at back to stale time (AddLead may have set it to now).
	if err := s.UpdateLead(id, map[string]interface{}{"stage": "preparing"}); err != nil {
		t.Fatalf("UpdateLead: %v", err)
	}
	// Now manually set updated_at via raw SQL to ensure it's old enough.
	if _, err := s.DB().Exec("UPDATE leads SET updated_at = ? WHERE id = ?", staleTime, id); err != nil {
		t.Fatalf("set updated_at: %v", err)
	}

	n, err := StalePhase(s)
	if err != nil {
		t.Fatalf("StalePhase: %v", err)
	}
	if n != 1 {
		t.Errorf("processed = %d, want 1", n)
	}

	lead, err := s.GetLead(id)
	if err != nil {
		t.Fatalf("GetLead: %v", err)
	}
	if lead.Stage != "skipped" {
		t.Errorf("Stage = %q, want %q", lead.Stage, "skipped")
	}
}

func TestStalePhaseWarningOnly(t *testing.T) {
	s := newTestStore(t)

	// Add a lead at 'preparing' with updated_at = 10 days ago (> 7 but < 14).
	staleTime := time.Now().UTC().Add(-10 * 24 * time.Hour).Format(time.RFC3339)
	id, err := s.AddLead(store.Lead{
		JobTitle:  "Frontend Dev",
		Company:   "MidCo",
		Pipeline:  "job",
		Stage:     "preparing",
		UpdatedAt: staleTime,
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}
	// Force updated_at to the stale time.
	if _, err := s.DB().Exec("UPDATE leads SET updated_at = ? WHERE id = ?", staleTime, id); err != nil {
		t.Fatalf("set updated_at: %v", err)
	}

	n, err := StalePhase(s)
	if err != nil {
		t.Fatalf("StalePhase: %v", err)
	}
	if n != 1 {
		t.Errorf("processed = %d, want 1 (logged as stale)", n)
	}

	lead, err := s.GetLead(id)
	if err != nil {
		t.Fatalf("GetLead: %v", err)
	}
	// Should remain in preparing -- only warned, not auto-skipped.
	if lead.Stage != "preparing" {
		t.Errorf("Stage = %q, want %q (should still be preparing)", lead.Stage, "preparing")
	}
}

func TestCadencePhase(t *testing.T) {
	s := newTestStore(t)

	// Add a lead in outreach-sent with a past next_date.
	pastDate := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	_, err := s.AddLead(store.Lead{
		JobTitle: "DevOps Engineer",
		Company:  "CloudCo",
		Pipeline: "job",
		Stage:    "outreach-sent",
		NextDate: pastDate,
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	n, err := CadencePhase(s)
	if err != nil {
		t.Fatalf("CadencePhase: %v", err)
	}
	if n != 1 {
		t.Errorf("processed = %d, want 1", n)
	}
}

func TestPreparePhase(t *testing.T) {
	s := newTestStore(t)

	_, err := s.AddLead(store.Lead{
		JobTitle: "Platform Engineer",
		Company:  "InfraCo",
		Pipeline: "job",
		Stage:    "qualified",
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	n, err := PreparePhase(s)
	if err != nil {
		t.Fatalf("PreparePhase: %v", err)
	}
	if n != 1 {
		t.Errorf("processed = %d, want 1", n)
	}
}
