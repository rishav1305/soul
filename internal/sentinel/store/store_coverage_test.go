package store

import (
	"path/filepath"
	"testing"
)

func openCovSentinelStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "cov.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// --- Not-found / ErrNoRows paths ---

func TestGetChallenge_NotFound(t *testing.T) {
	s := openCovSentinelStore(t)
	_, err := s.GetChallenge("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent challenge")
	}
}

func TestIsCompleted_NotCompleted(t *testing.T) {
	s := openCovSentinelStore(t)
	completed, err := s.IsCompleted("never-completed")
	if err != nil {
		t.Fatalf("IsCompleted: %v", err)
	}
	if completed {
		t.Error("expected false for never-completed challenge")
	}
}

func TestGetDefaultSandboxConfig_NoneExists(t *testing.T) {
	s := openCovSentinelStore(t)
	cfg, err := s.GetDefaultSandboxConfig()
	if err != nil {
		t.Fatalf("GetDefaultSandboxConfig: %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil when no configs, got %+v", cfg)
	}
}

func TestCountAttempts_NoAttempts(t *testing.T) {
	s := openCovSentinelStore(t)
	// Seed a challenge first (FK constraint).
	seedJSON := `[{"id":"c1","category":"web","difficulty":"easy","title":"Test","description":"d","objective":"o","flag":"f","system_prompt":"sp","tools":[],"hints":[],"points":10,"max_turns":5,"phase":1}]`
	if err := s.SeedChallenges([]byte(seedJSON)); err != nil {
		t.Fatalf("SeedChallenges: %v", err)
	}
	count, err := s.CountAttempts("c1")
	if err != nil {
		t.Fatalf("CountAttempts: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 attempts, got %d", count)
	}
}

func TestGetAttempts_NoAttempts(t *testing.T) {
	s := openCovSentinelStore(t)
	attempts, err := s.GetAttempts("no-challenge")
	if err != nil {
		t.Fatalf("GetAttempts: %v", err)
	}
	if len(attempts) != 0 {
		t.Errorf("expected 0 attempts, got %d", len(attempts))
	}
}

func TestGetProgress_NoCompletions(t *testing.T) {
	s := openCovSentinelStore(t)
	progress, err := s.GetProgress()
	if err != nil {
		t.Fatalf("GetProgress: %v", err)
	}
	if len(progress) != 0 {
		t.Errorf("expected empty progress, got %d entries", len(progress))
	}
}

func TestListGuardrails_Empty(t *testing.T) {
	s := openCovSentinelStore(t)
	guardrails, err := s.ListGuardrails()
	if err != nil {
		t.Fatalf("ListGuardrails: %v", err)
	}
	if len(guardrails) != 0 {
		t.Errorf("expected 0 guardrails, got %d", len(guardrails))
	}
}

func TestListChallenges_WithFilters(t *testing.T) {
	s := openCovSentinelStore(t)
	seedJSON := `[
		{"id":"c1","category":"web","difficulty":"easy","title":"T1","description":"d","objective":"o","flag":"f","system_prompt":"sp","tools":[],"hints":[],"points":10,"max_turns":5,"phase":1},
		{"id":"c2","category":"crypto","difficulty":"hard","title":"T2","description":"d","objective":"o","flag":"f","system_prompt":"sp","tools":[],"hints":[],"points":20,"max_turns":10,"phase":2},
		{"id":"c3","category":"web","difficulty":"hard","title":"T3","description":"d","objective":"o","flag":"f","system_prompt":"sp","tools":[],"hints":[],"points":15,"max_turns":8,"phase":1}
	]`
	if err := s.SeedChallenges([]byte(seedJSON)); err != nil {
		t.Fatal(err)
	}

	// Filter by category only.
	challenges, err := s.ListChallenges("web", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(challenges) != 2 {
		t.Errorf("web filter: expected 2, got %d", len(challenges))
	}

	// Filter by difficulty only.
	challenges, err = s.ListChallenges("", "hard", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(challenges) != 2 {
		t.Errorf("hard filter: expected 2, got %d", len(challenges))
	}

	// Filter by phase only.
	challenges, err = s.ListChallenges("", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(challenges) != 2 {
		t.Errorf("phase 1 filter: expected 2, got %d", len(challenges))
	}

	// Combined filters.
	challenges, err = s.ListChallenges("web", "hard", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(challenges) != 1 {
		t.Errorf("combined filter: expected 1, got %d", len(challenges))
	}
}

// --- SeedChallenges edge cases ---

func TestSeedChallenges_InvalidJSON(t *testing.T) {
	s := openCovSentinelStore(t)
	err := s.SeedChallenges([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSeedChallenges_ToolsAsObjects(t *testing.T) {
	s := openCovSentinelStore(t)
	// Tools as array of objects (not strings).
	seedJSON := `[{"id":"ct1","category":"web","difficulty":"easy","title":"T","description":"d","objective":"o","flag":"f","system_prompt":"sp","tools":[{"name":"tool1","desc":"d"}],"hints":["h1"],"points":10,"max_turns":5,"phase":1}]`
	err := s.SeedChallenges([]byte(seedJSON))
	if err != nil {
		t.Fatalf("SeedChallenges with tool objects: %v", err)
	}

	c, err := s.GetChallenge("ct1")
	if err != nil {
		t.Fatalf("GetChallenge: %v", err)
	}
	if len(c.Tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(c.Tools))
	}
	if len(c.Hints) != 1 {
		t.Errorf("expected 1 hint, got %d", len(c.Hints))
	}
}

func TestSeedChallenges_ToolsAsStrings(t *testing.T) {
	s := openCovSentinelStore(t)
	seedJSON := `[{"id":"ct2","category":"web","difficulty":"easy","title":"T","description":"d","objective":"o","flag":"f","system_prompt":"sp","tools":["read_file","write_file"],"hints":[],"points":10,"max_turns":5,"phase":1}]`
	err := s.SeedChallenges([]byte(seedJSON))
	if err != nil {
		t.Fatalf("SeedChallenges with tool strings: %v", err)
	}

	c, err := s.GetChallenge("ct2")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(c.Tools))
	}
}

func TestSeedChallenges_NullTools(t *testing.T) {
	s := openCovSentinelStore(t)
	seedJSON := `[{"id":"ct3","category":"web","difficulty":"easy","title":"T","description":"d","objective":"o","flag":"f","system_prompt":"sp","tools":null,"hints":null,"points":10,"max_turns":5,"phase":1}]`
	err := s.SeedChallenges([]byte(seedJSON))
	if err != nil {
		t.Fatalf("SeedChallenges with null tools: %v", err)
	}
}

// --- CRUD roundtrips ---

func TestRecordAttempt_AndGetAttempts(t *testing.T) {
	s := openCovSentinelStore(t)
	seedJSON := `[{"id":"a1","category":"web","difficulty":"easy","title":"T","description":"d","objective":"o","flag":"f","system_prompt":"sp","tools":[],"hints":[],"points":10,"max_turns":5,"phase":1}]`
	s.SeedChallenges([]byte(seedJSON))

	id, err := s.RecordAttempt("a1", "payload1", "response1", false)
	if err != nil {
		t.Fatalf("RecordAttempt: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero attempt ID")
	}

	id2, err := s.RecordAttempt("a1", "payload2", "response2", true)
	if err != nil {
		t.Fatalf("RecordAttempt 2: %v", err)
	}
	if id2 <= id {
		t.Error("second attempt ID should be greater")
	}

	count, err := s.CountAttempts("a1")
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}

	attempts, err := s.GetAttempts("a1")
	if err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 2 {
		t.Errorf("got %d attempts, want 2", len(attempts))
	}
}

func TestRecordCompletion_AndProgress(t *testing.T) {
	s := openCovSentinelStore(t)
	seedJSON := `[{"id":"p1","category":"web","difficulty":"easy","title":"T","description":"d","objective":"o","flag":"f","system_prompt":"sp","tools":[],"hints":[],"points":10,"max_turns":5,"phase":1}]`
	s.SeedChallenges([]byte(seedJSON))

	if err := s.RecordCompletion("p1", 10, 3, 1); err != nil {
		t.Fatalf("RecordCompletion: %v", err)
	}

	completed, err := s.IsCompleted("p1")
	if err != nil {
		t.Fatal(err)
	}
	if !completed {
		t.Error("expected completed=true")
	}

	progress, err := s.GetProgress()
	if err != nil {
		t.Fatal(err)
	}
	if progress["p1"] != 10 {
		t.Errorf("progress[p1] = %d, want 10", progress["p1"])
	}
}

func TestSaveGuardrail_AndList(t *testing.T) {
	s := openCovSentinelStore(t)
	id, err := s.SaveGuardrail("no-sql-inject", `{"type":"regex","pattern":".*"}`)
	if err != nil {
		t.Fatalf("SaveGuardrail: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero guardrail ID")
	}

	guardrails, err := s.ListGuardrails()
	if err != nil {
		t.Fatal(err)
	}
	if len(guardrails) != 1 {
		t.Errorf("expected 1 guardrail, got %d", len(guardrails))
	}
	if guardrails[0].Name != "no-sql-inject" {
		t.Errorf("name = %q, want no-sql-inject", guardrails[0].Name)
	}
}

func TestSaveSandboxConfig_AndGetDefault(t *testing.T) {
	s := openCovSentinelStore(t)
	id, err := s.SaveSandboxConfig("test-config", "You are a helpful AI", "[]", "none")
	if err != nil {
		t.Fatalf("SaveSandboxConfig: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero config ID")
	}

	cfg, err := s.GetDefaultSandboxConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if cfg.Name != "test-config" {
		t.Errorf("name = %q, want test-config", cfg.Name)
	}
	if cfg.WeaknessLevel != "none" {
		t.Errorf("weakness = %q, want none", cfg.WeaknessLevel)
	}
}

// --- Closed store error paths ---

func TestClosedSentinelStore(t *testing.T) {
	s := openCovSentinelStore(t)
	s.Close()

	if _, err := s.GetChallenge("x"); err == nil {
		t.Error("expected error from GetChallenge on closed store")
	}
	if _, err := s.ListChallenges("", "", 0); err == nil {
		t.Error("expected error from ListChallenges on closed store")
	}
	if _, err := s.RecordAttempt("x", "p", "r", false); err == nil {
		t.Error("expected error from RecordAttempt on closed store")
	}
	if _, err := s.CountAttempts("x"); err == nil {
		t.Error("expected error from CountAttempts on closed store")
	}
	if _, err := s.GetAttempts("x"); err == nil {
		t.Error("expected error from GetAttempts on closed store")
	}
	if err := s.RecordCompletion("x", 0, 0, 0); err == nil {
		t.Error("expected error from RecordCompletion on closed store")
	}
	if _, err := s.IsCompleted("x"); err == nil {
		t.Error("expected error from IsCompleted on closed store")
	}
	if _, err := s.GetProgress(); err == nil {
		t.Error("expected error from GetProgress on closed store")
	}
	if _, err := s.SaveGuardrail("n", "r"); err == nil {
		t.Error("expected error from SaveGuardrail on closed store")
	}
	if _, err := s.ListGuardrails(); err == nil {
		t.Error("expected error from ListGuardrails on closed store")
	}
	if _, err := s.SaveScanResult("p", "f", "s"); err == nil {
		t.Error("expected error from SaveScanResult on closed store")
	}
	if _, err := s.ListScanResults("p", 10); err == nil {
		t.Error("expected error from ListScanResults on closed store")
	}
	if _, err := s.SaveSandboxConfig("n", "sp", "[]", "none"); err == nil {
		t.Error("expected error from SaveSandboxConfig on closed store")
	}
	if _, err := s.GetDefaultSandboxConfig(); err == nil {
		t.Error("expected error from GetDefaultSandboxConfig on closed store")
	}
	if err := s.SeedChallenges([]byte(`[{"id":"x","category":"c","difficulty":"e","title":"t","description":"d","objective":"o","flag":"f","system_prompt":"sp","tools":[],"hints":[],"points":1,"max_turns":1,"phase":1}]`)); err == nil {
		t.Error("expected error from SeedChallenges on closed store")
	}
}

// --- Open error ---

func TestOpen_InvalidPath(t *testing.T) {
	_, err := Open("/nonexistent/deeply/nested/path/sentinel.db")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}
