package store

import (
	"path/filepath"
	"testing"
)

func TestQuizQuestionSourceDedup(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	topic, err := s.CreateTopic("dsa", "arrays", "two-sum", "medium", "")
	if err != nil {
		t.Fatal(err)
	}

	q1, err := s.CreateQuizQuestion(topic.ID, "medium", "What is two sum?", "Use hash map", "O(n)", "dsa_python:arrays:001")
	if err != nil {
		t.Fatal(err)
	}

	// Insert duplicate source — should not create a new row.
	q2, err := s.CreateQuizQuestion(topic.ID, "medium", "What is two sum?", "Use hash map", "O(n)", "dsa_python:arrays:001")
	if err != nil {
		t.Fatal(err)
	}

	if q1.ID != q2.ID {
		t.Errorf("expected dedup: q1.ID=%d q2.ID=%d", q1.ID, q2.ID)
	}

	// Different source should create a new row.
	q3, err := s.CreateQuizQuestion(topic.ID, "hard", "Three sum?", "Sort + two pointers", "O(n^2)", "dsa_python:arrays:002")
	if err != nil {
		t.Fatal(err)
	}
	if q3.ID == q1.ID {
		t.Error("expected different question for different source")
	}
}
