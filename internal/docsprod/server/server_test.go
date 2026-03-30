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
}

func TestHandleToolExecute_Valid(t *testing.T) {
	s := New()
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{"input": map[string]string{}})
	resp, err := http.Post(ts.URL+"/api/tools/docs__analyze/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestHandleToolExecute_Invalid(t *testing.T) {
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
		if resp.StatusCode != http.StatusOK {
			t.Errorf("tool %s: status = %d, want 200", tool, resp.StatusCode)
		}
	}
}

func TestHandleHealth_Products(t *testing.T) {
	s := New()
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	prods, ok := body["products"].([]interface{})
	if !ok {
		t.Fatal("expected products array")
	}
	if len(prods) != 2 {
		t.Errorf("expected 2 products, got %d", len(prods))
	}
}
