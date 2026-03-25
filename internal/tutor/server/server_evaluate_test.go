package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul/internal/tutor/eval"
	"github.com/rishav1305/soul/internal/tutor/store"
)

func TestHandleEvaluate(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	srv := New(WithStore(s), WithEvaluator(eval.New(nil)))

	// Create a topic + question.
	topic, _ := s.CreateTopic("dsa", "arrays", "test-topic", "medium", "")
	q, _ := s.CreateQuizQuestion(topic.ID, "medium", "What is a hash map?",
		"A hash map maps keys to values using a hash function", "O(1) lookup", "test:001")

	// Test valid evaluation.
	body, _ := json.Marshal(map[string]interface{}{
		"question_id": q.ID,
		"answer":      "A hash map uses a hash function to map keys to values",
	})
	req := httptest.NewRequest("POST", "/api/tutor/evaluate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Test missing question_id.
	body, _ = json.Marshal(map[string]interface{}{"answer": "test"})
	req = httptest.NewRequest("POST", "/api/tutor/evaluate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	// Test missing answer.
	body, _ = json.Marshal(map[string]interface{}{"question_id": q.ID})
	req = httptest.NewRequest("POST", "/api/tutor/evaluate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing answer, got %d", w.Code)
	}

	// Test non-existent question.
	body, _ = json.Marshal(map[string]interface{}{"question_id": 9999, "answer": "test"})
	req = httptest.NewRequest("POST", "/api/tutor/evaluate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleToolExecuteSysdesign(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	srv := New(WithStore(s), WithEvaluator(eval.New(nil)))

	// Test sysdesign_generate via tool execute.
	body, _ := json.Marshal(map[string]interface{}{
		"category": "ml_system",
		"name":     "Test Pipeline",
	})
	req := httptest.NewRequest("POST", "/api/tools/sysdesign_generate/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
