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

func testTutorServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return New(
		WithStore(s),
		WithEvaluator(eval.New(nil)),
		WithContentDir(dir),
		WithHost("127.0.0.1"),
		WithPort(0),
	)
}

// --- Drill Start ---

func TestHandleDrillStart_HappyPath(t *testing.T) {
	srv := testTutorServer(t)

	// Create topic + question.
	topic, err := srv.store.CreateTopic("dsa", "arrays", "Two Sum", "easy", "")
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	_, err = srv.store.CreateQuizQuestion(topic.ID, "easy",
		"What is the time complexity of brute force two sum?",
		"O(n^2) because we check all pairs.",
		"", "")
	if err != nil {
		t.Fatalf("CreateQuizQuestion: %v", err)
	}

	body, _ := json.Marshal(map[string]interface{}{
		"topic_id": float64(topic.ID),
	})
	req := httptest.NewRequest("POST", "/api/tutor/drill/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/tutor/drill/start: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["mode"] != "question" {
		t.Errorf("expected mode=question, got %v", resp["mode"])
	}
}

func TestHandleDrillStart_InvalidJSON(t *testing.T) {
	srv := testTutorServer(t)
	req := httptest.NewRequest("POST", "/api/tutor/drill/start", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --- Drill Answer ---

func TestHandleDrillAnswer_HappyPath(t *testing.T) {
	srv := testTutorServer(t)

	topic, err := srv.store.CreateTopic("dsa", "arrays", "Binary Search", "easy", "")
	if err != nil {
		t.Fatal(err)
	}
	q, err := srv.store.CreateQuizQuestion(topic.ID, "easy",
		"What is binary search time complexity?",
		"O(log n) because we halve the search space each step.",
		"", "")
	if err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]interface{}{
		"question_id": float64(q.ID),
		"answer":      "Binary search is O(log n) since it halves the array each iteration.",
	})
	req := httptest.NewRequest("POST", "/api/tutor/drill/answer", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/tutor/drill/answer: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["mode"] != "result" {
		t.Errorf("expected mode=result, got %v", resp["mode"])
	}
}

func TestHandleDrillAnswer_InvalidJSON(t *testing.T) {
	srv := testTutorServer(t)
	req := httptest.NewRequest("POST", "/api/tutor/drill/answer", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleDrillAnswer_MissingQuestionID(t *testing.T) {
	srv := testTutorServer(t)
	body, _ := json.Marshal(map[string]interface{}{
		"answer": "some answer",
	})
	req := httptest.NewRequest("POST", "/api/tutor/drill/answer", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleDrillAnswer_MissingAnswer(t *testing.T) {
	srv := testTutorServer(t)
	body, _ := json.Marshal(map[string]interface{}{
		"question_id": 1,
	})
	req := httptest.NewRequest("POST", "/api/tutor/drill/answer", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --- Drill Due ---

func TestHandleDrillDue_EmptyStore(t *testing.T) {
	srv := testTutorServer(t)
	req := httptest.NewRequest("GET", "/api/tutor/drill/due", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	count, _ := resp["count"].(float64)
	if count != 0 {
		t.Errorf("expected count=0, got %v", count)
	}
}

// --- WithOptions ---

func TestWithOptions_AllSet(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	srv := New(
		WithStore(s),
		WithHost("0.0.0.0"),
		WithPort(9999),
		WithContentDir("/tmp/content"),
		WithEvaluator(eval.New(nil)),
	)
	if srv.host != "0.0.0.0" {
		t.Errorf("host = %q, want '0.0.0.0'", srv.host)
	}
	if srv.port != 9999 {
		t.Errorf("port = %d, want 9999", srv.port)
	}
	if srv.contentDir != "/tmp/content" {
		t.Errorf("contentDir = %q, want '/tmp/content'", srv.contentDir)
	}
}

// --- Tool Execute dispatch ---

func TestHandleToolExecute_BehavioralNarrative(t *testing.T) {
	srv := testTutorServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{})
	resp, err := http.Post(ts.URL+"/api/tools/behavioral_narrative/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandleToolExecute_SysDesignLearn(t *testing.T) {
	srv := testTutorServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	// Create a topic first.
	topic, err := srv.store.CreateTopic("sysdesign", "distributed", "MapReduce", "hard", "")
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}

	body, _ := json.Marshal(map[string]interface{}{
		"topic_id": float64(topic.ID),
	})
	resp, err := http.Post(ts.URL+"/api/tools/sysdesign_learn/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandleToolExecute_SysDesignGenerate(t *testing.T) {
	srv := testTutorServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"category": "storage",
		"name":     "S3 Architecture",
	})
	resp, err := http.Post(ts.URL+"/api/tools/sysdesign_generate/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// --- Recovery middleware ---

func TestRecoveryMiddleware_Panic(t *testing.T) {
	panicker := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
	handler := recoveryMiddleware(panicker)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// --- Import ---

func TestHandleImport(t *testing.T) {
	srv := testTutorServer(t)
	req := httptest.NewRequest("POST", "/api/tutor/import", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Import returns 200 (source dir exists) or 500 (missing source dir).
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}
