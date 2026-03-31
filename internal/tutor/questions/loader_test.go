package questions

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul/internal/tutor/store"
	_ "modernc.org/sqlite"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	stats, err := Load(s)
	if err != nil {
		t.Fatal(err)
	}

	if stats.QuestionsCreated == 0 {
		t.Error("expected questions to be created")
	}

	// Verify idempotency — second load should not create duplicates.
	stats2, err := Load(s)
	if err != nil {
		t.Fatal(err)
	}

	// Verify total question count hasn't doubled.
	topics, _ := s.ListTopics("", "")
	totalQuestions := 0
	for _, topic := range topics {
		qs, _ := s.ListQuestions(topic.ID)
		totalQuestions += len(qs)
	}

	if totalQuestions != stats.QuestionsCreated {
		t.Errorf("expected %d total questions after 2 loads, got %d", stats.QuestionsCreated, totalQuestions)
	}
	_ = stats2
}

func TestLoadJSONValid(t *testing.T) {
	files := []string{"dsa_python.json", "ai_llm.json", "system_design.json", "behavioral.json"}
	for _, file := range files {
		data, err := questionFS.ReadFile(file)
		if err != nil {
			t.Errorf("cannot read %s: %v", file, err)
			continue
		}
		var questions []Question
		if err := json.Unmarshal(data, &questions); err != nil {
			t.Errorf("invalid JSON in %s: %v", file, err)
			continue
		}
		if len(questions) == 0 {
			t.Errorf("empty question bank: %s", file)
		}
		for i, q := range questions {
			if q.Module == "" || q.Category == "" || q.Topic == "" || q.Source == "" {
				t.Errorf("%s[%d]: missing required field", file, i)
			}
			if q.QuestionTxt == "" || q.Answer == "" {
				t.Errorf("%s[%d]: missing question or answer text", file, i)
			}
		}
	}
}

func TestLoad_ClosedStore(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "closed.db"))
	if err != nil {
		t.Fatal(err)
	}

	// Close the store to force CreateTopic/CreateQuizQuestion errors.
	s.Close()

	stats, err := Load(s)
	// Load should not return a hard error — it logs and skips individual failures.
	if err != nil {
		t.Fatalf("Load with closed store should not return error, got: %v", err)
	}

	// All questions should be skipped.
	if stats.QuestionsCreated != 0 {
		t.Errorf("expected 0 questions created with closed store, got %d", stats.QuestionsCreated)
	}
	if stats.QuestionsSkipped == 0 {
		t.Error("expected non-zero skipped count with closed store")
	}
}

func TestLoad_StatsStructure(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	stats, err := Load(s)
	if err != nil {
		t.Fatal(err)
	}

	// Verify stats are non-negative and consistent.
	if stats.TopicsCreated < 0 {
		t.Errorf("negative TopicsCreated: %d", stats.TopicsCreated)
	}
	if stats.QuestionsCreated < 0 {
		t.Errorf("negative QuestionsCreated: %d", stats.QuestionsCreated)
	}
	if stats.QuestionsSkipped < 0 {
		t.Errorf("negative QuestionsSkipped: %d", stats.QuestionsSkipped)
	}

	// At least some questions should load from the 4 JSON files.
	if stats.QuestionsCreated < 10 {
		t.Errorf("expected at least 10 questions from 4 JSON files, got %d", stats.QuestionsCreated)
	}
}

func TestLoad_MultipleCallsDoNotError(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "multi.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// First load should succeed.
	stats1, err := Load(s)
	if err != nil {
		t.Fatal(err)
	}
	if stats1.QuestionsCreated == 0 {
		t.Error("first Load created 0 questions")
	}

	// Second load should also succeed (no hard errors).
	stats2, err := Load(s)
	if err != nil {
		t.Fatalf("second Load errored: %v", err)
	}
	_ = stats2
}

func TestLoad_CreateQuizQuestionError(t *testing.T) {
	// Exercise the CreateQuizQuestion error path: topics exist (CreateTopic
	// succeeds), but quiz_questions table is dropped so INSERT fails.
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "qfail.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	// First load — seed topics and questions.
	stats1, err := Load(s)
	if err != nil {
		t.Fatal(err)
	}
	if stats1.QuestionsCreated == 0 {
		t.Fatal("first Load created 0 questions")
	}
	s.Close()

	// Drop quiz_questions table via direct SQL connection.
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DROP TABLE IF EXISTS quiz_questions"); err != nil {
		t.Fatalf("drop quiz_questions: %v", err)
	}
	// Recreate a minimal table so the SELECT in CreateQuizQuestion works
	// but the INSERT fails due to missing columns referenced by FK/indexes.
	// Actually, just recreate without the UNIQUE index — the SELECT will
	// return "no rows" (table exists but empty), then INSERT will work.
	// Instead, drop the table entirely and don't recreate it.
	// CreateQuizQuestion's SELECT will fail (table gone), fall through to
	// INSERT which also fails → error returned → Load logs and skips.
	db.Close()

	// Reopen the store — it will try to CREATE TABLE IF NOT EXISTS,
	// which recreates quiz_questions. We need to drop AFTER reopen.
	s2, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	// Drop table again after store.Open recreated it.
	db2, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db2.Exec("DROP TABLE quiz_questions"); err != nil {
		t.Fatalf("drop quiz_questions after reopen: %v", err)
	}
	db2.Close()

	// Now Load: topics still exist → CreateTopic returns existing topic (no error).
	// CreateQuizQuestion → SELECT fails (no table) → INSERT fails → error logged, skipped.
	stats2, err := Load(s2)
	if err != nil {
		t.Fatalf("Load should not return hard error, got: %v", err)
	}
	if stats2.QuestionsCreated != 0 {
		t.Errorf("expected 0 questions created, got %d", stats2.QuestionsCreated)
	}
	if stats2.QuestionsSkipped == 0 {
		t.Error("expected non-zero skipped count")
	}
	// TopicsCreated reflects topics returned (existing) — may be > 0.
	if stats2.TopicsCreated == 0 {
		t.Error("expected non-zero TopicsCreated (existing topics returned)")
	}
}
