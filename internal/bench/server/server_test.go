package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rishav1305/soul/internal/bench/harness"
	"github.com/rishav1305/soul/internal/bench/results"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	return New(WithHost("127.0.0.1"), WithPort(0), WithDataDir(dir))
}

func TestHandleHealth(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Errorf("status = %v, want ok", body["status"])
	}
}

func TestHandleListCategories(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest("GET", "/api/bench/prompts", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	cats, ok := body["categories"].([]interface{})
	if !ok || len(cats) == 0 {
		t.Error("expected non-empty categories list")
	}
}

func TestHandleGetPrompts_Valid(t *testing.T) {
	s := testServer(t)
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/bench/prompts/smoke-test")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestHandleGetPrompts_NotFound(t *testing.T) {
	s := testServer(t)
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/bench/prompts/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHandleListResults_Empty(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest("GET", "/api/bench/results", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandleListResults_NoDataDir(t *testing.T) {
	s := New() // No data dir
	req := httptest.NewRequest("GET", "/api/bench/results", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestHandleGetResult_NotFound(t *testing.T) {
	s := testServer(t)
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/bench/results/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHandleCompare_MissingParams(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest("GET", "/api/bench/compare", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleRunBenchmark_MissingEndpoint(t *testing.T) {
	s := testServer(t)
	body, _ := json.Marshal(runRequest{})
	req := httptest.NewRequest("POST", "/api/bench/run", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleRunBenchmark_InvalidBody(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest("POST", "/api/bench/run", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleSmoke_MissingEndpoint(t *testing.T) {
	s := testServer(t)
	body, _ := json.Marshal(runRequest{})
	req := httptest.NewRequest("POST", "/api/bench/smoke", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleToolExecute_ListCategories(t *testing.T) {
	s := testServer(t)
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	body, _ := json.Marshal(toolRequest{Input: map[string]interface{}{}})
	resp, err := http.Post(ts.URL+"/api/tools/bench_list_categories/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestHandleToolExecute_Unknown(t *testing.T) {
	s := testServer(t)
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	body, _ := json.Marshal(toolRequest{Input: map[string]interface{}{}})
	resp, err := http.Post(ts.URL+"/api/tools/unknown_tool/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHandleToolExecute_ListResults(t *testing.T) {
	s := testServer(t)

	// Save a result first.
	r := &harness.BenchResult{
		Model:            "test-model",
		Timestamp:        "2026-03-30T10:00:00Z",
		Results:          []harness.PromptResult{},
		CategoryAccuracy: map[string]float64{},
	}
	results.SaveResult(s.dataDir, r)

	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	body, _ := json.Marshal(toolRequest{Input: map[string]interface{}{}})
	resp, err := http.Post(ts.URL+"/api/tools/bench_list_results/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestCorsMiddleware(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest("OPTIONS", "/api/health", nil)
	w := httptest.NewRecorder()
	corsMiddleware(s.mux).ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got == "" {
		t.Error("expected CORS Allow-Origin header")
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, map[string]string{"key": "val"})

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusNotFound, "not found")

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["error"] != "not found" {
		t.Errorf("error = %q, want %q", body["error"], "not found")
	}
}

func TestWithOptions(t *testing.T) {
	s := New(WithHost("0.0.0.0"), WithPort(9999), WithDataDir("/tmp/test"))
	if s.host != "0.0.0.0" {
		t.Errorf("host = %q, want %q", s.host, "0.0.0.0")
	}
	if s.port != 9999 {
		t.Errorf("port = %d, want 9999", s.port)
	}
	if s.dataDir != "/tmp/test" {
		t.Errorf("dataDir = %q, want %q", s.dataDir, "/tmp/test")
	}
}

// --- Additional coverage tests ---

func TestRecoveryMiddleware(t *testing.T) {
	panicker := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
	handler := recoveryMiddleware(panicker)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["error"] != "internal server error" {
		t.Errorf("error = %q, want 'internal server error'", body["error"])
	}
}

func TestRecoveryMiddleware_NoPanic(t *testing.T) {
	normal := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	})
	handler := recoveryMiddleware(normal)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandleGetResult_NoDataDir(t *testing.T) {
	s := New() // No data dir
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/bench/results/any-id")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

func TestHandleCompare_NoDataDir(t *testing.T) {
	s := New() // No data dir
	req := httptest.NewRequest("GET", "/api/bench/compare?id1=a&id2=b", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestHandleCompare_NotFound(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest("GET", "/api/bench/compare?id1=nonexistent1&id2=nonexistent2", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleSmoke_InvalidBody(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest("POST", "/api/bench/smoke", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleToolExecute_InvalidBody(t *testing.T) {
	s := testServer(t)
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/tools/bench_list_categories/execute", "application/json", bytes.NewBufferString("not-json"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestHandleToolExecute_RunSmokeMissingEndpoint(t *testing.T) {
	s := testServer(t)
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	body, _ := json.Marshal(toolRequest{Input: map[string]interface{}{}})
	resp, err := http.Post(ts.URL+"/api/tools/bench_run_smoke/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestHandleToolExecute_Compare(t *testing.T) {
	s := testServer(t)
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	// bench_compare with empty IDs — should fail from results pkg.
	body, _ := json.Marshal(toolRequest{Input: map[string]interface{}{
		"id1": "",
		"id2": "",
	}})
	resp, err := http.Post(ts.URL+"/api/tools/bench_compare/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Will get 500 since results don't exist.
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

func TestStartShutdown(t *testing.T) {
	s := New(WithHost("127.0.0.1"), WithPort(0))
	errCh := make(chan error, 1)
	go func() { errCh <- s.Start() }()
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown error: %v", err)
	}
}

func TestCorsMiddleware_NonOptions(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := corsMiddleware(inner)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got == "" {
		t.Error("expected CORS header on non-OPTIONS request")
	}
}

// mockModelEndpoint returns a httptest.Server that simulates a model inference endpoint.
func mockModelEndpoint(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return a plausible response for any prompt.
		json.NewEncoder(w).Encode(map[string]interface{}{
			"response":          "The answer is 42. This is a test response.",
			"tokens_per_second": 100.0,
			"peak_ram_mb":       512.0,
			"peak_vram_mb":      0.0,
		})
	}))
}

func TestHandleRunBenchmark_WithMockEndpoint(t *testing.T) {
	model := mockModelEndpoint(t)
	defer model.Close()

	s := testServer(t)
	body, _ := json.Marshal(runRequest{
		ModelEndpoint: model.URL,
		Categories:    []string{"smoke-test"},
		MaxTokens:     100,
	})
	req := httptest.NewRequest("POST", "/api/bench/run", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["model"] == nil {
		t.Error("expected 'model' field in result")
	}
}

func TestHandleSmoke_WithMockEndpoint(t *testing.T) {
	model := mockModelEndpoint(t)
	defer model.Close()

	s := testServer(t)
	body, _ := json.Marshal(runRequest{
		ModelEndpoint: model.URL,
		MaxTokens:     100,
	})
	req := httptest.NewRequest("POST", "/api/bench/smoke", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandleRunBenchmark_EndpointError(t *testing.T) {
	// Endpoint that always returns 500.
	badModel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model error", http.StatusInternalServerError)
	}))
	defer badModel.Close()

	s := testServer(t)
	body, _ := json.Marshal(runRequest{
		ModelEndpoint: badModel.URL,
		Categories:    []string{"smoke-test"},
		MaxTokens:     100,
	})
	req := httptest.NewRequest("POST", "/api/bench/run", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	// RunBenchmark returns endpoint-unreachable results (accuracy=0) but still succeeds.
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandleToolExecute_RunSmokeWithMock(t *testing.T) {
	model := mockModelEndpoint(t)
	defer model.Close()

	s := testServer(t)
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	body, _ := json.Marshal(toolRequest{Input: map[string]interface{}{
		"model_endpoint": model.URL,
	}})
	resp, err := http.Post(ts.URL+"/api/tools/bench_run_smoke/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestHandleGetResult_WithSavedResult(t *testing.T) {
	s := testServer(t)

	// Save a result and then fetch it by ID.
	r := &harness.BenchResult{
		Model:            "test-model",
		Timestamp:        "2026-03-30T12:00:00Z",
		Results:          []harness.PromptResult{},
		CategoryAccuracy: map[string]float64{"smoke-test": 0.9},
	}
	results.SaveResult(s.dataDir, r)

	// List results to get the saved ID.
	list, _ := results.ListResults(s.dataDir)
	if len(list) == 0 {
		t.Fatal("no results saved")
	}

	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/bench/results/" + list[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestHandleListResults_WithSavedResult(t *testing.T) {
	s := testServer(t)

	// Save a benchmark result.
	r := &harness.BenchResult{
		Model:            "test-model",
		Timestamp:        "2026-03-30T10:00:00Z",
		Results:          []harness.PromptResult{},
		CategoryAccuracy: map[string]float64{"smoke-test": 1.0},
	}
	if err := results.SaveResult(s.dataDir, r); err != nil {
		t.Fatalf("SaveResult: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/bench/results", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	count, ok := body["count"].(float64)
	if !ok || count < 1 {
		t.Errorf("expected at least 1 result, got %v", body["count"])
	}
}
