package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rishav1305/soul/internal/chat/metrics"
	"github.com/rishav1305/soul/internal/tutor/eval"
	"github.com/rishav1305/soul/internal/tutor/store"
)

// --- Start/Shutdown lifecycle ---

func TestStartShutdown(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	srv := New(WithStore(s), WithPort(0))

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

// --- WithMetrics option ---

func TestWithMetrics(t *testing.T) {
	mel, err := metrics.NewEventLogger(t.TempDir(), "test")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	srv := New(WithMetrics(mel))
	if srv.metrics != mel {
		t.Error("expected metrics to be set")
	}
}

// --- requestLoggerMiddleware ---

func TestRequestLoggerMiddleware(t *testing.T) {
	mel, _ := metrics.NewEventLogger(t.TempDir(), "test")
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := requestLoggerMiddleware(mel)(inner)

	req := httptest.NewRequest("GET", "/api/tutor/dashboard", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRequestLoggerMiddleware_SkipsHealth(t *testing.T) {
	mel, _ := metrics.NewEventLogger(t.TempDir(), "test")
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := requestLoggerMiddleware(mel)(inner)

	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// --- statusRecorder ---

func TestStatusRecorder_WriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rec, status: 200}
	sr.WriteHeader(http.StatusNotFound)
	if sr.status != 404 {
		t.Errorf("status = %d, want 404", sr.status)
	}
}

// --- handleMockAnswer error paths ---

func TestHandleMockAnswer_InvalidID(t *testing.T) {
	srv := newTestServer(t)
	body, _ := json.Marshal(map[string]interface{}{"overall_score": 75.0})
	req := httptest.NewRequest("POST", "/api/tutor/mocks/notanid/answer", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleMockAnswer_InvalidJSON(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/tutor/mocks/1/answer", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --- handleCreateMock error paths ---

func TestHandleCreateMock_InvalidJSON(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/tutor/mocks", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --- handleCreatePlan error paths ---

func TestHandleCreatePlan_InvalidJSON(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/tutor/plan", strings.NewReader("bad json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --- handleEvaluate error paths ---

func TestHandleEvaluate_InvalidJSON(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/tutor/evaluate", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleEvaluate_EmptyAnswer(t *testing.T) {
	srv := newTestServer(t)
	body, _ := json.Marshal(map[string]interface{}{"question_id": 1, "answer": ""})
	req := httptest.NewRequest("POST", "/api/tutor/evaluate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleEvaluate_AIModule(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	srv := New(WithStore(s), WithEvaluator(eval.New(nil)))

	topic, _ := s.CreateTopic("ai", "ml", "Backpropagation", "medium", "")
	q, _ := s.CreateQuizQuestion(topic.ID, "medium", "What is backprop?",
		"Backpropagation computes gradients via the chain rule", "", "")

	body, _ := json.Marshal(map[string]interface{}{
		"question_id": q.ID,
		"answer":      "Backprop computes partial derivatives layer by layer",
	})
	req := httptest.NewRequest("POST", "/api/tutor/evaluate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleEvaluate_SysDesignModule(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	srv := New(WithStore(s), WithEvaluator(eval.New(nil)))

	topic, _ := s.CreateTopic("sysdesign", "distributed", "CAP Theorem", "hard", "")
	q, _ := s.CreateQuizQuestion(topic.ID, "hard", "Explain CAP theorem",
		"CAP states you can only have 2 of 3: consistency, availability, partition tolerance", "", "")

	body, _ := json.Marshal(map[string]interface{}{
		"question_id": q.ID,
		"answer":      "In distributed systems, CAP theorem limits to 2 of 3 guarantees",
	})
	req := httptest.NewRequest("POST", "/api/tutor/evaluate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- handleListTopics with module filter ---

func TestHandleListTopics_WithModuleFilter(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/tutor/topics?module=dsa", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- handleToolExecute additional dispatch paths ---

func TestHandleToolExecute_DSALearn(t *testing.T) {
	srv := testTutorServer(t)
	srv.store.CreateTopic("dsa", "arrays", "Quick Sort", "medium", "")
	body, _ := json.Marshal(map[string]interface{}{"topic": "Quick Sort"})
	req := httptest.NewRequest("POST", "/api/tools/dsa_learn/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleToolExecute_DSABuild(t *testing.T) {
	srv := testTutorServer(t)
	body, _ := json.Marshal(map[string]interface{}{"topic_id": float64(1)})
	req := httptest.NewRequest("POST", "/api/tools/dsa_build/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	// dsa_build may return 200 or 500 depending on topic existence, but should not panic.
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleToolExecute_DSAGenerate(t *testing.T) {
	srv := testTutorServer(t)
	body, _ := json.Marshal(map[string]interface{}{"category": "graphs", "name": "DFS"})
	req := httptest.NewRequest("POST", "/api/tools/dsa_generate/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleToolExecute_AILearn(t *testing.T) {
	srv := testTutorServer(t)
	srv.store.CreateTopic("ai", "ml", "Neural Nets", "medium", "")
	body, _ := json.Marshal(map[string]interface{}{"topic": "Neural Nets"})
	req := httptest.NewRequest("POST", "/api/tools/ai_learn/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleToolExecute_AIGenerate(t *testing.T) {
	srv := testTutorServer(t)
	body, _ := json.Marshal(map[string]interface{}{"category": "ml", "name": "CNN"})
	req := httptest.NewRequest("POST", "/api/tools/ai_generate/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleToolExecute_BehavioralStar(t *testing.T) {
	srv := testTutorServer(t)
	body, _ := json.Marshal(map[string]interface{}{"competency": "leadership"})
	req := httptest.NewRequest("POST", "/api/tools/behavioral_star/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleToolExecute_BehavioralHR(t *testing.T) {
	srv := testTutorServer(t)
	body, _ := json.Marshal(map[string]interface{}{"category": "gaps"})
	req := httptest.NewRequest("POST", "/api/tools/behavioral_hr/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleToolExecute_MockAnalyzeJD(t *testing.T) {
	srv := testTutorServer(t)
	body, _ := json.Marshal(map[string]interface{}{"text": "Senior SWE at Google"})
	req := httptest.NewRequest("POST", "/api/tools/mock_analyze_jd/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleToolExecute_DSASolve(t *testing.T) {
	srv := testTutorServer(t)
	topic, _ := srv.store.CreateTopic("dsa", "arrays", "Merge Sort", "medium", "")
	q, _ := srv.store.CreateQuizQuestion(topic.ID, "medium", "Implement merge sort", "merge sort impl", "", "")
	body, _ := json.Marshal(map[string]interface{}{
		"question_id": float64(q.ID),
		"solution":    "func mergeSort(arr []int) []int { return arr }",
	})
	req := httptest.NewRequest("POST", "/api/tools/dsa_solve/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleToolExecute_AIModule_Drill(t *testing.T) {
	srv := testTutorServer(t)
	topic, _ := srv.store.CreateTopic("ai", "ml", "Gradient Descent", "easy", "")
	q, _ := srv.store.CreateQuizQuestion(topic.ID, "easy", "What is gradient descent?",
		"It optimizes by moving in the direction of steepest descent", "", "")
	body, _ := json.Marshal(map[string]interface{}{
		"question_id": float64(q.ID),
		"answer":      "Gradient descent minimizes loss by following the gradient",
	})
	req := httptest.NewRequest("POST", "/api/tools/ai_drill/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleToolExecute_DSADrill(t *testing.T) {
	srv := testTutorServer(t)
	topic, _ := srv.store.CreateTopic("dsa", "trees", "BST Traversal", "easy", "")
	body, _ := json.Marshal(map[string]interface{}{
		"topic_id": float64(topic.ID),
	})
	req := httptest.NewRequest("POST", "/api/tools/dsa_drill/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	// Drill with topic_id starts a quiz - should work or fail gracefully.
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleToolExecute_SysDesignDrill(t *testing.T) {
	srv := testTutorServer(t)
	topic, _ := srv.store.CreateTopic("sysdesign", "distributed", "Consensus", "hard", "")
	q, _ := srv.store.CreateQuizQuestion(topic.ID, "hard", "What is Raft?",
		"Raft is a consensus algorithm for distributed log replication", "", "")
	body, _ := json.Marshal(map[string]interface{}{
		"question_id": float64(q.ID),
		"answer":      "Raft manages replicated log with leader election",
	})
	req := httptest.NewRequest("POST", "/api/tools/sysdesign_drill/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- WithMetrics middleware chain ---

func TestNewWithMetrics_MiddlewareChain(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mel, _ := metrics.NewEventLogger(t.TempDir(), "test")

	srv := New(WithStore(s), WithMetrics(mel))
	// The httpServer handler should include requestLoggerMiddleware.
	handler := srv.httpServer.Handler

	req := httptest.NewRequest("GET", "/api/tutor/dashboard", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
