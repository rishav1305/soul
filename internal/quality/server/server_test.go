package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
	prods, ok := body["products"].([]interface{})
	if !ok || len(prods) != 3 {
		t.Errorf("expected 3 products, got %v", body["products"])
	}
}

func TestHandleToolExecute_QAStub(t *testing.T) {
	s := New()
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{"input": map[string]string{}})
	resp, err := http.Post(ts.URL+"/api/tools/qa__analyze/execute", "application/json", bytes.NewReader(body))
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

	body, _ := json.Marshal(map[string]interface{}{})
	resp, err := http.Post(ts.URL+"/api/tools/unknown/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestHandleToolExecute_ComplianceScan(t *testing.T) {
	s := New()
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	// compliance__scan routes to compliance.Service.ExecuteTool
	body := []byte(`{"target":"./test"}`)
	resp, err := http.Post(ts.URL+"/api/tools/compliance__scan/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Should get OK or 500 depending on compliance service — just verify it routes
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 200 or 500", resp.StatusCode)
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

// --- Middleware coverage tests ---

func TestRecoveryMiddleware_NoPanic(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := recoveryMiddleware(inner)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestRecoveryMiddleware_Panic(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
	handler := recoveryMiddleware(inner)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestCSPMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := cspMiddleware(inner)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	csp := w.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("expected CSP header")
	}
}

func TestBodyLimitMiddleware_GET(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := bodyLimitMiddleware(10)(inner)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestBodyLimitMiddleware_POST(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		_, err := r.Body.Read(buf)
		if err != nil {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	handler := bodyLimitMiddleware(10)(inner)

	body := strings.NewReader(strings.Repeat("x", 100))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("POST", "/", body))
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", w.Code)
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

func TestHandleToolExecute_AnalyticsStub(t *testing.T) {
	s := New()
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	body := []byte(`{}`)
	resp, err := http.Post(ts.URL+"/api/tools/analytics__analyze/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	data := result["data"].(map[string]interface{})
	if data["status"] != "not_yet_implemented" {
		t.Errorf("status = %v, want not_yet_implemented", data["status"])
	}
}

func TestHandleToolExecute_EmptyBody(t *testing.T) {
	s := New()
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	// Send with empty body — should use default {}
	resp, err := http.Post(ts.URL+"/api/tools/qa__report/execute", "application/json", bytes.NewReader(nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestHandleToolExecute_AllTools(t *testing.T) {
	s := New()
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	for tool := range validTools {
		body := []byte(`{}`)
		resp, err := http.Post(ts.URL+"/api/tools/"+tool+"/execute", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("tool %s: %v", tool, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("tool %s: status = %d, want 200 or 500", tool, resp.StatusCode)
		}
	}
}

func TestHandleToolExecute_ComplianceFix(t *testing.T) {
	s := New()
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	body := []byte(`{"target":"./test"}`)
	resp, err := http.Post(ts.URL+"/api/tools/compliance__fix/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 200 or 500", resp.StatusCode)
	}
}

func TestHandleToolExecute_ComplianceBadge(t *testing.T) {
	s := New()
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	body := []byte(`{}`)
	resp, err := http.Post(ts.URL+"/api/tools/compliance__badge/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 200 or 500", resp.StatusCode)
	}
}
