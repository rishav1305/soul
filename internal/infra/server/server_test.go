package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleHealth(t *testing.T) {
	s := New()
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
	products, ok := body["products"].([]interface{})
	if !ok || len(products) == 0 {
		t.Error("expected non-empty products list")
	}
}

func TestHandleToolExecute_ValidTool(t *testing.T) {
	s := New()
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{"input": map[string]string{}})
	resp, err := http.Post(ts.URL+"/api/tools/devops__analyze/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["success"] != true {
		t.Errorf("success = %v, want true", result["success"])
	}
}

func TestHandleToolExecute_InvalidTool(t *testing.T) {
	s := New()
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{"input": map[string]string{}})
	resp, err := http.Post(ts.URL+"/api/tools/unknown/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestWithOptions(t *testing.T) {
	s := New(WithHost("0.0.0.0"), WithPort(9999))
	if s.host != "0.0.0.0" {
		t.Errorf("host = %q, want %q", s.host, "0.0.0.0")
	}
	if s.port != 9999 {
		t.Errorf("port = %d, want 9999", s.port)
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
	writeError(w, http.StatusNotFound, "missing")
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestCspMiddleware(t *testing.T) {
	s := New()
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	// Use the full middleware chain via httpServer.Handler
	s.httpServer.Handler.ServeHTTP(w, req)

	if got := w.Header().Get("Content-Security-Policy"); got == "" {
		t.Error("expected CSP header")
	}
}
