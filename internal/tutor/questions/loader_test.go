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
	files := []string{"dsa_python.json", "ai_llm.json", "system_design.json"}
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
