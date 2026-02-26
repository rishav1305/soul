package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rishav1305/soul/internal/config"
	"github.com/rishav1305/soul/internal/server"
)

func TestHealthEndpoint(t *testing.T) {
	cfg := config.Default()
	srv := server.New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}

	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", body["status"])
	}
}

func TestSPAFallback(t *testing.T) {
	cfg := config.Default()
	srv := server.New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/some/spa/route", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 for SPA fallback, got %d", w.Code)
	}

	body := w.Body.String()
	if len(body) == 0 {
		t.Fatal("expected non-empty body for SPA fallback")
	}

	// Should contain our placeholder index.html content
	if !containsString(body, "Soul") {
		t.Fatalf("expected body to contain 'Soul', got %q", body)
	}

	if !containsString(body, "id=\"root\"") {
		t.Fatalf("expected body to contain div#root, got %q", body)
	}
}

func TestAPINotFoundReturns404(t *testing.T) {
	cfg := config.Default()
	srv := server.New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}

	if body["error"] == "" {
		t.Fatal("expected error field in JSON response")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
