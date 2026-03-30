package questions

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul/internal/tutor/store"
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
