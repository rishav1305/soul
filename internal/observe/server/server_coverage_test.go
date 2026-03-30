package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rishav1305/soul/internal/chat/metrics"
)

func newCovObserveServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	// Seed multiple event types for richer coverage.
	logger, err := metrics.NewEventLogger(dir, "")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	for i := 0; i < 10; i++ {
		logger.Log(metrics.EventAPIRequest, map[string]interface{}{
			"method": "GET", "path": "/api/health", "status": 200, "duration_ms": 5 + i,
		})
	}
	for i := 0; i < 5; i++ {
		logger.Log(metrics.EventWSConnect, map[string]interface{}{
			"client_id": "abc",
		})
	}
	logger.Log(metrics.EventDBQuery, map[string]interface{}{
		"method": "GetSession", "duration_ms": 3, "rows": 1,
	})
	logger.Close()

	return New(WithDataDir(dir), WithHost("127.0.0.1"), WithPort(0)), dir
}

// --- handleTail edge cases ---

func TestHandleTail_WithTypeFilter(t *testing.T) {
	srv, _ := newCovObserveServer(t)
	req := httptest.NewRequest("GET", "/api/tail?type=ws", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)
	events := body["events"].([]interface{})
	for _, ev := range events {
		m := ev.(map[string]interface{})
		if !strings.HasPrefix(m["event"].(string), "ws") {
			t.Errorf("expected ws prefix, got %q", m["event"])
		}
	}
}

func TestHandleTail_WithLimit(t *testing.T) {
	srv, _ := newCovObserveServer(t)
	req := httptest.NewRequest("GET", "/api/tail?limit=3", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)
	count := int(body["count"].(float64))
	if count > 3 {
		t.Errorf("count = %d, want <= 3", count)
	}
}

func TestHandleTail_LimitCappedAt500(t *testing.T) {
	srv, _ := newCovObserveServer(t)
	req := httptest.NewRequest("GET", "/api/tail?limit=9999", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHandleTail_InvalidLimit(t *testing.T) {
	srv, _ := newCovObserveServer(t)
	req := httptest.NewRequest("GET", "/api/tail?limit=abc", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (fallback to default limit)", rec.Code)
	}
}

func TestHandleTail_WithProduct(t *testing.T) {
	srv, dir := newCovObserveServer(t)

	// Create product-specific file.
	pLogger, _ := metrics.NewEventLogger(dir, "chat")
	pLogger.Log(metrics.EventWSConnect, map[string]interface{}{"product": "chat"})
	pLogger.Close()

	req := httptest.NewRequest("GET", "/api/tail?product=chat", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHandleTail_TypeAndLimit(t *testing.T) {
	srv, _ := newCovObserveServer(t)
	req := httptest.NewRequest("GET", "/api/tail?type=api&limit=2", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)
	count := int(body["count"].(float64))
	if count > 2 {
		t.Errorf("count = %d, want <= 2", count)
	}
}

// --- Product-filtered handler paths ---

func TestHandleOverview_WithProduct(t *testing.T) {
	srv, _ := newCovObserveServer(t)
	req := httptest.NewRequest("GET", "/api/overview?product=chat", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleSystem_WithProduct(t *testing.T) {
	srv, _ := newCovObserveServer(t)
	req := httptest.NewRequest("GET", "/api/system?product=chat", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)
	if _, ok := body["server_uptime"]; !ok {
		t.Error("missing server_uptime field")
	}
}

// --- Error paths using permission-restricted data dir ---

func TestHandleTail_ReadError(t *testing.T) {
	// Create a data dir that's a file (not a directory) to trigger ReadDir error.
	tmpDir := t.TempDir()
	fakeDir := filepath.Join(tmpDir, "notadir")
	os.WriteFile(fakeDir, []byte("not a dir"), 0600)

	srv := New(WithDataDir(fakeDir), WithHost("127.0.0.1"), WithPort(0))
	req := httptest.NewRequest("GET", "/api/tail", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 for unreadable data dir", rec.Code)
	}
}

func TestHandleLatency_EmptyDir(t *testing.T) {
	srv := New(WithDataDir(t.TempDir()), WithHost("127.0.0.1"), WithPort(0))
	req := httptest.NewRequest("GET", "/api/latency", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	// Empty dir → no events → should still succeed.
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHandlePillars_EmptyDir(t *testing.T) {
	srv := New(WithDataDir(t.TempDir()), WithHost("127.0.0.1"), WithPort(0))
	req := httptest.NewRequest("GET", "/api/pillars", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
