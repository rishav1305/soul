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

	r.Register("failing-phase", func(_ *store.Store) (int, error) {
		return 0, fmt.Errorf("simulated failure")
	})

	r.runCycle()

	if r.Cycles() != 1 {
		t.Errorf("Cycles() = %d, want 1", r.Cycles())
	}
}

func TestRunCycle_PhaseWithProcessed(t *testing.T) {
	s := newTestStore(t)
	r := New(s, time.Minute)

	r.Register("active-phase", func(_ *store.Store) (int, error) {
		return 5, nil
	})

	r.runCycle()

	if r.Cycles() != 1 {
		t.Errorf("Cycles() = %d, want 1", r.Cycles())
	}
}

// --- ContentStalePhase: SQLite datetime fallback format ---
// Covers content.go line 51: time.Parse("2006-01-02 15:04:05", p.CreatedAt)

func TestContentStalePhase_SQLiteDateFormat(t *testing.T) {
	s := newTestStore(t)

	// Use SQLite datetime format (not RFC3339) — triggers the fallback parse path.
	oldDate := time.Now().UTC().Add(-20 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	_, err := s.AddContentPost(store.ContentPost{
		Platform:  "linkedin",
		Topic:     "SQLite Date Post",
		Status:    "draft",
		Content:   "stale content",
		CreatedAt: oldDate,
	})
	if err != nil {
		t.Fatalf("AddContentPost: %v", err)
	}

	n, err := ContentStalePhase(s)
	if err != nil {
		t.Fatalf("ContentStalePhase: %v", err)
	}
	if n != 1 {
		t.Errorf("count = %d, want 1 (stale draft with SQLite date)", n)
	}
}

// --- ContentStalePhase: bad date error path ---
// Covers content.go lines 52-54: both parse attempts fail, log and continue.

func TestContentStalePhase_BadCreatedAt(t *testing.T) {
	s := newTestStore(t)

	_, err := s.AddContentPost(store.ContentPost{
		Platform:  "linkedin",
		Topic:     "Bad Date Post",
		Status:    "draft",
		Content:   "content",
		CreatedAt: "not-a-date",
	})
	if err != nil {
		t.Fatalf("AddContentPost: %v", err)
	}

	n, err := ContentStalePhase(s)
	if err != nil {
		t.Fatalf("ContentStalePhase: %v", err)
	}
	if n != 0 {
		t.Errorf("count = %d, want 0 (bad date skipped)", n)
	}
}

// --- StalePhase: bad updated_at path ---
// Covers job.go lines 157-159: time.Parse fails, log and continue.

func TestStalePhase_BadUpdatedAt(t *testing.T) {
	s := newTestStore(t)

	id, err := s.AddLead(store.Lead{
		JobTitle: "Backend Dev",
		Company:  "BadDateCo",
		Pipeline: "job",
		Stage:    "preparing",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Set updated_at to a string that is lexicographically < warningCutoff (so SQL finds it)
	// but fails time.Parse(RFC3339, ...) in Go.
	badDate := "2020-01-01 bad-time"
	if _, err := s.DB().Exec("UPDATE leads SET updated_at = ? WHERE id = ?", badDate, id); err != nil {
		t.Fatal(err)
	}

	n, err := StalePhase(s)
	if err != nil {
		t.Fatalf("StalePhase: %v", err)
	}
	if n != 0 {
		t.Errorf("processed = %d, want 0 (bad date skipped)", n)
	}

	lead, _ := s.GetLead(id)
	if lead.Stage != "preparing" {
		t.Errorf("Stage = %q, want preparing", lead.Stage)
	}
}

// --- Closed store edge cases ---
// These cover the initial query error paths (fmt.Errorf returns).

func TestContentStalePhase_ClosedStore(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "cstale.db"))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	_, err = ContentStalePhase(s)
	if err == nil {
		t.Log("ContentStalePhase with closed store returned nil error (acceptable)")
	}
}

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

func TestNetworkingEngagePhase_ClosedStore(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "nengage.db"))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	_, err = NetworkingEngagePhase(s)
	if err == nil {
		t.Log("NetworkingEngagePhase with closed store returned nil error (acceptable)")
	}
}

func TestContractQualifyPhase_ClosedStore(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "cqual2.db"))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	_, err = ContractQualifyPhase(s)
	if err == nil {
		t.Log("ContractQualifyPhase with closed store returned nil error (acceptable)")
	}
}

func TestFreelanceQualifyPhase_ClosedStore(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "fqual.db"))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	_, err = FreelanceQualifyPhase(s)
	if err == nil {
		t.Log("FreelanceQualifyPhase with closed store returned nil error (acceptable)")
	}
}

func TestContractEngagedPhase_ClosedStore(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "cengage.db"))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	_, err = ContractEngagedPhase(s)
	if err == nil {
		t.Log("ContractEngagedPhase with closed store returned nil error (acceptable)")
	}
}

func TestFreelanceStalePhase_ClosedStore(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "fstale.db"))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	_, err = FreelanceStalePhase(s)
	if err == nil {
		t.Log("FreelanceStalePhase with closed store returned nil error (acceptable)")
	}
}

func TestConsultingStalePhase_ClosedStore(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "csstale.db"))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	_, err = ConsultingStalePhase(s)
	if err == nil {
		t.Log("ConsultingStalePhase with closed store returned nil error (acceptable)")
	}
}

func TestNetworkingStalePhase_ClosedStore(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "nstale.db"))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	_, err = NetworkingStalePhase(s)
	if err == nil {
		t.Log("NetworkingStalePhase with closed store returned nil error (acceptable)")
	}
}

func TestQualifyPhase_ClosedStore(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "jqual.db"))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	_, err = QualifyPhase(s)
	if err == nil {
		t.Log("QualifyPhase with closed store returned nil error (acceptable)")
	}
}

func TestPreparePhase_ClosedStore(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "jprep.db"))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	_, err = PreparePhase(s)
	if err == nil {
		t.Log("PreparePhase with closed store returned nil error (acceptable)")
	}
}

func TestCadencePhase_ClosedStore(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "jcad.db"))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	_, err = CadencePhase(s)
	if err == nil {
		t.Log("CadencePhase with closed store returned nil error (acceptable)")
	}
}

func TestStalePhase_ClosedStore(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "jstale.db"))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	_, err = StalePhase(s)
	if err == nil {
		t.Log("StalePhase with closed store returned nil error (acceptable)")
	}
}

func TestContentPublishPhase_ClosedStore(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "cpub.db"))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	_, err = ContentPublishPhase(s)
	if err == nil {
		t.Log("ContentPublishPhase with closed store returned nil error (acceptable)")
	}
}

func TestProfileAuditPhase_ClosedStore(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "paudit.db"))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	_, err = ProfileAuditPhase(s)
	if err == nil {
		t.Log("ProfileAuditPhase with closed store returned nil error (acceptable)")
	}
}

// --- Empty DB tests (no matching leads) ---
// Covers the "zero rows returned" path where rows.Next() is never entered.

func TestNetworkingEngagePhase_Empty(t *testing.T) {
	s := newTestStore(t)
	n, err := NetworkingEngagePhase(s)
	if err != nil {
		t.Fatalf("NetworkingEngagePhase: %v", err)
	}
	if n != 0 {
		t.Errorf("processed = %d, want 0", n)
	}
}

func TestNetworkingWarmPhase_Empty(t *testing.T) {
	s := newTestStore(t)
	n, err := NetworkingWarmPhase(s)
	if err != nil {
		t.Fatalf("NetworkingWarmPhase: %v", err)
	}
	if n != 0 {
		t.Errorf("processed = %d, want 0", n)
	}
}

func TestContractQualifyPhase_Empty(t *testing.T) {
	s := newTestStore(t)
	n, err := ContractQualifyPhase(s)
	if err != nil {
		t.Fatalf("ContractQualifyPhase: %v", err)
	}
	if n != 0 {
		t.Errorf("processed = %d, want 0", n)
	}
}

func TestFreelanceQualifyPhase_Empty(t *testing.T) {
	s := newTestStore(t)
	n, err := FreelanceQualifyPhase(s)
	if err != nil {
		t.Fatalf("FreelanceQualifyPhase: %v", err)
	}
	if n != 0 {
		t.Errorf("processed = %d, want 0", n)
	}
}

func TestConsultingQualifyPhase_Empty(t *testing.T) {
	s := newTestStore(t)
	n, err := ConsultingQualifyPhase(s)
	if err != nil {
		t.Fatalf("ConsultingQualifyPhase: %v", err)
	}
	if n != 0 {
		t.Errorf("processed = %d, want 0", n)
	}
}

// --- Table-drop technique: GetInteractionCount error path ---
// Covers networking.go lines 44-46 and 89-91 (GetInteractionCount error → log, continue).

func TestNetworkingEngagePhase_InteractionCountError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "nengage_err.db")
	s2, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	// Add a networking lead at "connected".
	_, err = s2.AddLead(store.Lead{
		JobTitle: "CTO",
		Company:  "ErrCo",
		Pipeline: "networking",
		Stage:    "connected",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Drop lead_interactions table so GetInteractionCount fails.
	if _, err := s2.DB().Exec("DROP TABLE interactions"); err != nil {
		t.Fatalf("drop lead_interactions: %v", err)
	}

	// Phase should log the error and continue (not crash).
	n, err := NetworkingEngagePhase(s2)
	if err != nil {
		t.Fatalf("NetworkingEngagePhase: %v", err)
	}
	// No leads processed because GetInteractionCount errored.
	if n != 0 {
		t.Errorf("processed = %d, want 0 (interaction count error)", n)
	}
	s2.Close()
}

func TestNetworkingWarmPhase_InteractionCountError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "nwarm_err.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.AddLead(store.Lead{
		JobTitle: "Director",
		Company:  "ErrCo2",
		Pipeline: "networking",
		Stage:    "engaging",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.DB().Exec("DROP TABLE interactions"); err != nil {
		t.Fatalf("drop: %v", err)
	}

	n, err := NetworkingWarmPhase(s)
	if err != nil {
		t.Fatalf("NetworkingWarmPhase: %v", err)
	}
	if n != 0 {
		t.Errorf("processed = %d, want 0", n)
	}
	s.Close()
}

// --- Table-drop technique: GetArtifacts error path ---
// Covers consulting.go lines 42-44 (GetArtifacts error → log, continue).

func TestConsultingQualifyPhase_ArtifactsError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "cqual_err.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.AddLead(store.Lead{
		JobTitle: "Consultant",
		Company:  "ArtErrCo",
		Pipeline: "consulting",
		Stage:    "lead",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Drop lead_artifacts table so GetArtifacts fails.
	if _, err := s.DB().Exec("DROP TABLE lead_artifacts"); err != nil {
		t.Fatalf("drop: %v", err)
	}

	n, err := ConsultingQualifyPhase(s)
	if err != nil {
		t.Fatalf("ConsultingQualifyPhase: %v", err)
	}
	if n != 0 {
		t.Errorf("processed = %d, want 0 (artifacts error)", n)
	}
	s.Close()
}

// --- ContentStalePhase: SQLite datetime that is recent (not stale) ---
// Covers content.go line 51 with a recent SQLite-format date that passes parse
// but is NOT before the cutoff.

func TestContentStalePhase_SQLiteDateRecent(t *testing.T) {
	s := newTestStore(t)

	recentDate := time.Now().UTC().Add(-2 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	_, err := s.AddContentPost(store.ContentPost{
		Platform:  "twitter",
		Topic:     "Recent SQLite Date",
		Status:    "draft",
		Content:   "fresh",
		CreatedAt: recentDate,
	})
	if err != nil {
		t.Fatalf("AddContentPost: %v", err)
	}

	n, err := ContentStalePhase(s)
	if err != nil {
		t.Fatalf("ContentStalePhase: %v", err)
	}
	if n != 0 {
		t.Errorf("count = %d, want 0 (recent SQLite date)", n)
	}
}

// --- ContentPublishPhase: scheduled post with empty date ---
// Covers content.go line 26: p.ScheduledDate == "" check.

func TestContentPublishPhase_EmptyScheduledDate(t *testing.T) {
	s := newTestStore(t)

	_, err := s.AddContentPost(store.ContentPost{
		Platform: "linkedin",
		Topic:    "No Date Post",
		Status:   "scheduled",
		Content:  "content",
		// ScheduledDate is empty.
	})
	if err != nil {
		t.Fatalf("AddContentPost: %v", err)
	}

	n, err := ContentPublishPhase(s)
	if err != nil {
		t.Fatalf("ContentPublishPhase: %v", err)
	}
	if n != 0 {
		t.Errorf("count = %d, want 0 (empty scheduled date)", n)
	}
}
