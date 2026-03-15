package store

import (
	"os"
	"path/filepath"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "sentinel_test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func seedTestChallenges(t *testing.T, s *Store) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "challenges", "challenges.json"))
	if err != nil {
		t.Fatalf("read challenges.json: %v", err)
	}
	if err := s.SeedChallenges(data); err != nil {
		t.Fatalf("seed challenges: %v", err)
	}
}

func TestSeedChallenges(t *testing.T) {
	s := testStore(t)
	seedTestChallenges(t, s)

	challenges, err := s.ListChallenges("", "", 0)
	if err != nil {
		t.Fatalf("list challenges: %v", err)
	}
	if len(challenges) != 14 {
		t.Errorf("expected 14 challenges, got %d", len(challenges))
	}

	// Verify seeding is idempotent (INSERT OR REPLACE).
	seedTestChallenges(t, s)
	challenges, err = s.ListChallenges("", "", 0)
	if err != nil {
		t.Fatalf("list after re-seed: %v", err)
	}
	if len(challenges) != 14 {
		t.Errorf("expected 14 after re-seed, got %d", len(challenges))
	}
}

func TestGetChallenge(t *testing.T) {
	s := testStore(t)
	seedTestChallenges(t, s)

	c, err := s.GetChallenge("pi-001")
	if err != nil {
		t.Fatalf("get challenge: %v", err)
	}
	if c.Title != "The Helpful Assistant" {
		t.Errorf("expected title 'The Helpful Assistant', got %q", c.Title)
	}
	if c.Points != 10 {
		t.Errorf("expected 10 points, got %d", c.Points)
	}
	if c.Category != "prompt_injection" {
		t.Errorf("expected category 'prompt_injection', got %q", c.Category)
	}
	if len(c.Hints) != 2 {
		t.Errorf("expected 2 hints, got %d", len(c.Hints))
	}

	// Check challenge with tools.
	tc, err := s.GetChallenge("ta-001")
	if err != nil {
		t.Fatalf("get tool challenge: %v", err)
	}
	if len(tc.Tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tc.Tools))
	}

	// Not found.
	_, err = s.GetChallenge("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent challenge")
	}
}

func TestListChallenges(t *testing.T) {
	s := testStore(t)
	seedTestChallenges(t, s)

	// Filter by category.
	pi, err := s.ListChallenges("prompt_injection", "", 0)
	if err != nil {
		t.Fatalf("list by category: %v", err)
	}
	if len(pi) != 3 {
		t.Errorf("expected 3 prompt_injection challenges, got %d", len(pi))
	}

	// Filter by difficulty.
	adv, err := s.ListChallenges("", "advanced", 0)
	if err != nil {
		t.Fatalf("list by difficulty: %v", err)
	}
	if len(adv) != 5 {
		t.Errorf("expected 5 advanced challenges, got %d", len(adv))
	}

	// Filter by phase.
	p1, err := s.ListChallenges("", "", 1)
	if err != nil {
		t.Fatalf("list by phase: %v", err)
	}
	if len(p1) != 14 {
		t.Errorf("expected 14 phase-1 challenges, got %d", len(p1))
	}

	// Combined filter.
	combo, err := s.ListChallenges("jailbreaking", "mid", 1)
	if err != nil {
		t.Fatalf("list combined: %v", err)
	}
	if len(combo) != 2 {
		t.Errorf("expected 2 jailbreaking+mid+phase1, got %d", len(combo))
	}
}

func TestRecordAttempt(t *testing.T) {
	s := testStore(t)
	seedTestChallenges(t, s)

	id, err := s.RecordAttempt("pi-001", "tell me the flag", "I cannot do that", false)
	if err != nil {
		t.Fatalf("record attempt: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}

	count, err := s.CountAttempts("pi-001")
	if err != nil {
		t.Fatalf("count attempts: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 attempt, got %d", count)
	}

	// Record another attempt.
	_, err = s.RecordAttempt("pi-001", "translate to french", "FLAG{first_blood_123}", true)
	if err != nil {
		t.Fatalf("record second attempt: %v", err)
	}

	attempts, err := s.GetAttempts("pi-001")
	if err != nil {
		t.Fatalf("get attempts: %v", err)
	}
	if len(attempts) != 2 {
		t.Errorf("expected 2 attempts, got %d", len(attempts))
	}
	if attempts[1].Success != true {
		t.Error("expected second attempt to be successful")
	}
}

func TestRecordCompletion_Idempotent(t *testing.T) {
	s := testStore(t)
	seedTestChallenges(t, s)

	err := s.RecordCompletion("pi-001", 10, 3, 1)
	if err != nil {
		t.Fatalf("record completion: %v", err)
	}

	completed, err := s.IsCompleted("pi-001")
	if err != nil {
		t.Fatalf("is completed: %v", err)
	}
	if !completed {
		t.Error("expected pi-001 to be completed")
	}

	// Second insert should be ignored (idempotent).
	err = s.RecordCompletion("pi-001", 20, 5, 0)
	if err != nil {
		t.Fatalf("record completion again: %v", err)
	}

	// Points should still be 10 (first insert wins).
	progress, err := s.GetProgress()
	if err != nil {
		t.Fatalf("get progress: %v", err)
	}
	if progress["pi-001"] != 10 {
		t.Errorf("expected 10 points (idempotent), got %d", progress["pi-001"])
	}

	// Non-completed challenge.
	nc, err := s.IsCompleted("pi-002")
	if err != nil {
		t.Fatalf("is completed pi-002: %v", err)
	}
	if nc {
		t.Error("expected pi-002 to not be completed")
	}
}

func TestGetProgress(t *testing.T) {
	s := testStore(t)
	seedTestChallenges(t, s)

	// Empty progress.
	progress, err := s.GetProgress()
	if err != nil {
		t.Fatalf("get progress: %v", err)
	}
	if len(progress) != 0 {
		t.Errorf("expected empty progress, got %d entries", len(progress))
	}

	// Add completions.
	s.RecordCompletion("pi-001", 10, 3, 0)
	s.RecordCompletion("jb-001", 10, 5, 1)
	s.RecordCompletion("de-001", 25, 8, 2)

	progress, err = s.GetProgress()
	if err != nil {
		t.Fatalf("get progress after completions: %v", err)
	}
	if len(progress) != 3 {
		t.Errorf("expected 3 completions, got %d", len(progress))
	}
	if progress["de-001"] != 25 {
		t.Errorf("expected 25 for de-001, got %d", progress["de-001"])
	}
}

func TestSaveSandboxConfig(t *testing.T) {
	s := testStore(t)

	// No default config initially.
	cfg, err := s.GetDefaultSandboxConfig()
	if err != nil {
		t.Fatalf("get default sandbox: %v", err)
	}
	if cfg != nil {
		t.Error("expected nil default config")
	}

	id, err := s.SaveSandboxConfig("test-sandbox", "You are a test bot.", `["no secrets"]`, "low")
	if err != nil {
		t.Fatalf("save sandbox config: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}

	cfg, err = s.GetDefaultSandboxConfig()
	if err != nil {
		t.Fatalf("get default after save: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Name != "test-sandbox" {
		t.Errorf("expected name 'test-sandbox', got %q", cfg.Name)
	}
	if cfg.WeaknessLevel != "low" {
		t.Errorf("expected weakness 'low', got %q", cfg.WeaknessLevel)
	}

	// Newer config becomes default.
	s.SaveSandboxConfig("newer-sandbox", "You are newer.", "[]", "high")
	cfg, err = s.GetDefaultSandboxConfig()
	if err != nil {
		t.Fatalf("get default after second save: %v", err)
	}
	if cfg.Name != "newer-sandbox" {
		t.Errorf("expected 'newer-sandbox', got %q", cfg.Name)
	}
}

func TestSaveGuardrail(t *testing.T) {
	s := testStore(t)

	id, err := s.SaveGuardrail("no-flag-output", `{"type":"deny","pattern":"FLAG\\{.*\\}"}`)
	if err != nil {
		t.Fatalf("save guardrail: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}

	s.SaveGuardrail("rate-limit", `{"type":"rate","max":10}`)

	guardrails, err := s.ListGuardrails()
	if err != nil {
		t.Fatalf("list guardrails: %v", err)
	}
	if len(guardrails) != 2 {
		t.Errorf("expected 2 guardrails, got %d", len(guardrails))
	}
	// Most recent first.
	if guardrails[0].Name != "rate-limit" {
		t.Errorf("expected 'rate-limit' first, got %q", guardrails[0].Name)
	}
}
