package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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

	resp, _ := http.Get(ts.URL + "/api/bench/prompts/nonexistent")
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

	resp, _ := http.Get(ts.URL + "/api/bench/results/nonexistent")
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
	resp, _ := http.Post(ts.URL+"/api/tools/unknown_tool/execute", "application/json", bytes.NewReader(body))
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
	resp, _ := http.Post(ts.URL+"/api/tools/bench_list_results/execute", "application/json", bytes.NewReader(body))
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
