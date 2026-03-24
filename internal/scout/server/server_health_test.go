package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul-v2/internal/scout/store"
)

// newTestServer creates a minimal Scout server with routes registered for testing.
// Bypasses Start() to avoid network binding; registers routes directly.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	srv := New(WithStore(st))
	srv.registerRoutes() // register routes without starting the HTTP listener
	return srv
}

func TestHandleHealth_APIPath(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /api/health: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("GET /api/health: invalid JSON: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("GET /api/health: expected status=ok, got %v", resp["status"])
	}
}

func TestHandleHealth_RootPath(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /health: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("GET /health: invalid JSON: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("GET /health: expected status=ok, got %v", resp["status"])
	}
	if resp["service"] != "soul-scout" {
		t.Errorf("GET /health: expected service=soul-scout, got %v", resp["service"])
	}
}
