package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rishav1305/soul/internal/chat/metrics"
	"github.com/rishav1305/soul/internal/projects/store"
)

// testServerForCoverage creates a test server with a seeded project.
func testServerForCoverage(t *testing.T) (*Server, int64) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	srv := New(WithStore(s), WithContentDir(dir))
	projID, err := s.CreateProject("test-project", "test desc", 1, 4, 20.0)
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	return srv, projID
}

// --- Start/Shutdown lifecycle ---

func TestStartShutdown(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	srv := New(WithStore(s), WithPort("0"))

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

	req := httptest.NewRequest("GET", "/api/projects/dashboard", nil)
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

func TestNewWithMetrics_MiddlewareChain(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mel, _ := metrics.NewEventLogger(t.TempDir(), "test")
	srv := New(WithStore(s), WithMetrics(mel))
	handler := srv.httpServer.Handler

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// --- Handler error paths ---

func TestHandleUpdateProject_InvalidID(t *testing.T) {
	srv, _ := testServerForCoverage(t)
	req := httptest.NewRequest("PATCH", "/api/projects/abc", strings.NewReader(`{"status":"active"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleUpdateMilestone_InvalidID(t *testing.T) {
	srv, _ := testServerForCoverage(t)
	req := httptest.NewRequest("PATCH", "/api/projects/1/milestones/abc", strings.NewReader(`{"status":"done"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleUpdateMilestone_InvalidJSON(t *testing.T) {
	srv, _ := testServerForCoverage(t)
	req := httptest.NewRequest("PATCH", "/api/projects/1/milestones/1", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleRecordMetric_InvalidID(t *testing.T) {
	srv, _ := testServerForCoverage(t)
	req := httptest.NewRequest("POST", "/api/projects/abc/metrics", strings.NewReader(`{"name":"loc","value":"1000"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleRecordMetric_InvalidJSON(t *testing.T) {
	srv, project := testServerForCoverage(t)
	req := httptest.NewRequest("POST", "/api/projects/"+itoa(project)+"/metrics", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleSyncPlatform_InvalidID(t *testing.T) {
	srv, _ := testServerForCoverage(t)
	req := httptest.NewRequest("POST", "/api/projects/abc/syncs", strings.NewReader(`{"platform":"github"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleSyncPlatform_InvalidJSON(t *testing.T) {
	srv, project := testServerForCoverage(t)
	req := httptest.NewRequest("POST", "/api/projects/"+itoa(project)+"/syncs", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleRecordReadiness_InvalidID(t *testing.T) {
	srv, _ := testServerForCoverage(t)
	req := httptest.NewRequest("POST", "/api/projects/abc/readiness", strings.NewReader(`{"self_score":5}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleRecordReadiness_InvalidJSON(t *testing.T) {
	srv, project := testServerForCoverage(t)
	req := httptest.NewRequest("POST", "/api/projects/"+itoa(project)+"/readiness", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleGetGuide_ValidID_NoGuide(t *testing.T) {
	srv, project := testServerForCoverage(t)
	req := httptest.NewRequest("GET", "/api/projects/"+itoa(project)+"/guide", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	// Should return 404 because no guide file exists for this project.
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// --- Tool dispatch error paths ---

func TestHandleToolExecute_ProjectDetailInvalidID(t *testing.T) {
	srv, _ := testServerForCoverage(t)
	body, _ := json.Marshal(map[string]interface{}{
		"input": map[string]interface{}{"project_id": "not_a_number"},
	})
	req := httptest.NewRequest("POST", "/api/tools/project_detail/execute", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp ToolResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Success {
		t.Error("expected success=false for invalid project_id")
	}
}

func TestHandleToolExecute_ProjectDetailEmptyName(t *testing.T) {
	srv, _ := testServerForCoverage(t)
	body, _ := json.Marshal(map[string]interface{}{
		"input": map[string]interface{}{"project_name": ""},
	})
	req := httptest.NewRequest("POST", "/api/tools/project_detail/execute", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp ToolResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Success {
		t.Error("expected success=false for empty project_name")
	}
}

func TestHandleToolExecute_UpdateProgressInvalidID(t *testing.T) {
	srv, _ := testServerForCoverage(t)
	body, _ := json.Marshal(map[string]interface{}{
		"input": map[string]interface{}{"project_id": "bad"},
	})
	req := httptest.NewRequest("POST", "/api/tools/update_progress/execute", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var resp ToolResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Success {
		t.Error("expected success=false for invalid project_id")
	}
}

func TestHandleToolExecute_UpdateProgressWithHours(t *testing.T) {
	srv, project := testServerForCoverage(t)
	body, _ := json.Marshal(map[string]interface{}{
		"input": map[string]interface{}{
			"project_id":   float64(project),
			"hours_actual": 10.5,
			"status":       "active",
		},
	})
	req := httptest.NewRequest("POST", "/api/tools/update_progress/execute", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var resp ToolResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if !resp.Success {
		t.Errorf("expected success=true, error: %s", resp.Error)
	}
}

func TestHandleToolExecute_RecordMetricInvalidID(t *testing.T) {
	srv, _ := testServerForCoverage(t)
	body, _ := json.Marshal(map[string]interface{}{
		"input": map[string]interface{}{"project_id": "bad", "name": "loc", "value": "100"},
	})
	req := httptest.NewRequest("POST", "/api/tools/record_metric/execute", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var resp ToolResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Success {
		t.Error("expected success=false")
	}
}

func TestHandleToolExecute_SyncProfileInvalidID(t *testing.T) {
	srv, _ := testServerForCoverage(t)
	body, _ := json.Marshal(map[string]interface{}{
		"input": map[string]interface{}{"project_id": "bad", "platform": "github"},
	})
	req := httptest.NewRequest("POST", "/api/tools/sync_profile/execute", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var resp ToolResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Success {
		t.Error("expected success=false")
	}
}

func TestHandleToolExecute_SyncProfileMissingID(t *testing.T) {
	srv, _ := testServerForCoverage(t)
	body, _ := json.Marshal(map[string]interface{}{
		"input": map[string]interface{}{"platform": "github"},
	})
	req := httptest.NewRequest("POST", "/api/tools/sync_profile/execute", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var resp ToolResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Success {
		t.Error("expected success=false")
	}
}

func TestHandleToolExecute_RecordMetricMissingID(t *testing.T) {
	srv, _ := testServerForCoverage(t)
	body, _ := json.Marshal(map[string]interface{}{
		"input": map[string]interface{}{"name": "loc", "value": "100"},
	})
	req := httptest.NewRequest("POST", "/api/tools/record_metric/execute", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var resp ToolResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Success {
		t.Error("expected success=false")
	}
}

// --- statusWriter ---

func TestStatusWriter_WriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec, status: 200}
	sw.WriteHeader(http.StatusNotFound)
	if sw.status != 404 {
		t.Errorf("status = %d, want 404", sw.status)
	}
}

// Helper
func itoa(n int64) string {
	return strings.TrimSpace(strings.Replace(
		func() string { b, _ := json.Marshal(n); return string(b) }(), "\"", "", -1))
}
