package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rishav1305/soul/internal/chat/metrics"
	"github.com/rishav1305/soul/internal/tasks/executor"
	"github.com/rishav1305/soul/internal/tasks/store"
	"github.com/rishav1305/soul/pkg/events"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return New(WithStore(s), WithLogger(events.NopLogger{}))
}

func TestHealthEndpoint(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("status = %v, want ok", body["status"])
	}
}

func TestCreateTask(t *testing.T) {
	srv := newTestServer(t)
	body := `{"title":"Test task","description":"A description"}`
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	var task store.Task
	json.NewDecoder(rec.Body).Decode(&task)
	if task.Title != "Test task" {
		t.Errorf("Title = %q, want %q", task.Title, "Test task")
	}
}

func TestListTasks(t *testing.T) {
	srv := newTestServer(t)

	// Create two tasks.
	for _, title := range []string{"A", "B"} {
		body := `{"title":"` + title + `"}`
		req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
	}

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var tasks []store.Task
	json.NewDecoder(rec.Body).Decode(&tasks)
	if len(tasks) != 2 {
		t.Errorf("len = %d, want 2", len(tasks))
	}
}

func TestGetTask(t *testing.T) {
	srv := newTestServer(t)

	body := `{"title":"Get me"}`
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var created store.Task
	json.NewDecoder(rec.Body).Decode(&created)

	req = httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%d", created.ID), nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestGetTask_NotFound(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/tasks/999", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestUpdateTask(t *testing.T) {
	srv := newTestServer(t)

	body := `{"title":"Original"}`
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var created store.Task
	json.NewDecoder(rec.Body).Decode(&created)

	patchBody := `{"title":"Updated","stage":"active"}`
	req = httptest.NewRequest("PATCH", fmt.Sprintf("/api/tasks/%d", created.ID), strings.NewReader(patchBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var updated store.Task
	json.NewDecoder(rec.Body).Decode(&updated)
	if updated.Title != "Updated" {
		t.Errorf("Title = %q, want %q", updated.Title, "Updated")
	}
}

func TestDeleteTask(t *testing.T) {
	srv := newTestServer(t)

	body := `{"title":"Delete me"}`
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var created store.Task
	json.NewDecoder(rec.Body).Decode(&created)

	req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/tasks/%d", created.ID), nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

func TestTaskActivity(t *testing.T) {
	srv := newTestServer(t)

	body := `{"title":"With activity"}`
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var created store.Task
	json.NewDecoder(rec.Body).Decode(&created)

	req = httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%d/activity", created.ID), nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	// Should have a task.created activity from the POST handler.
	var activities []store.Activity
	json.NewDecoder(rec.Body).Decode(&activities)
	if len(activities) < 1 {
		t.Error("expected at least 1 activity entry")
	}
}

func newTestServerWithExecutor(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	exec := executor.New(executor.Config{Store: s, MaxParallel: 3})
	srv := New(WithStore(s), WithLogger(events.NopLogger{}), WithExecutor(exec))
	return srv, s
}

func TestStartTask_NoExecutor(t *testing.T) {
	srv := newTestServer(t)

	body := `{"title":"Start me"}`
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var created store.Task
	json.NewDecoder(rec.Body).Decode(&created)

	req = httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%d/start", created.ID), nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

func TestStartTask(t *testing.T) {
	srv, _ := newTestServerWithExecutor(t)

	body := `{"title":"Start me"}`
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var created store.Task
	json.NewDecoder(rec.Body).Decode(&created)

	req = httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%d/start", created.ID), nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["status"] != "started" {
		t.Errorf("status = %q, want started", resp["status"])
	}
}

func TestStopTask_NotRunning(t *testing.T) {
	srv, _ := newTestServerWithExecutor(t)

	body := `{"title":"Stop me"}`
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var created store.Task
	json.NewDecoder(rec.Body).Decode(&created)

	req = httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%d/stop", created.ID), nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

func TestSyncEndpoint_FullSync(t *testing.T) {
	srv := newTestServer(t)
	srv.store.Create("task1", "", "")
	srv.store.Create("task2", "", "")

	req := httptest.NewRequest("GET", "/api/tasks/sync", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp struct {
		Tasks    []store.Task `json:"tasks"`
		Deleted  []int64      `json:"deleted"`
		Cursor   string       `json:"cursor"`
		FullSync bool         `json:"fullSync"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	if !resp.FullSync {
		t.Error("expected fullSync=true for no cursor")
	}
	if len(resp.Tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(resp.Tasks))
	}
	if resp.Cursor == "" {
		t.Error("expected non-empty cursor")
	}
}

func TestSyncEndpoint_DeltaSync(t *testing.T) {
	srv := newTestServer(t)
	srv.store.Create("task1", "", "")

	// Full sync to get cursor.
	req1 := httptest.NewRequest("GET", "/api/tasks/sync", nil)
	w1 := httptest.NewRecorder()
	srv.ServeHTTP(w1, req1)
	var resp1 struct{ Cursor string `json:"cursor"` }
	json.NewDecoder(w1.Body).Decode(&resp1)

	// Create another task.
	srv.store.Create("task2", "", "")

	// Delta sync.
	req2 := httptest.NewRequest("GET", "/api/tasks/sync?cursor="+resp1.Cursor, nil)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)

	var resp2 struct {
		Tasks    []store.Task `json:"tasks"`
		FullSync bool         `json:"fullSync"`
	}
	json.NewDecoder(w2.Body).Decode(&resp2)

	if resp2.FullSync {
		t.Error("expected fullSync=false for delta sync")
	}
	if len(resp2.Tasks) != 1 || resp2.Tasks[0].Title != "task2" {
		t.Errorf("expected 1 delta task (task2), got %d", len(resp2.Tasks))
	}
}

func TestSyncEndpoint_StaleCursor(t *testing.T) {
	srv := newTestServer(t)
	srv.store.Create("task1", "", "")

	stale := store.EncodeCursor(1, 0) // ts=0 (epoch) is definitely >24h ago
	req := httptest.NewRequest("GET", "/api/tasks/sync?cursor="+stale, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var resp struct{ FullSync bool `json:"fullSync"` }
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.FullSync {
		t.Error("expected fullSync=true for stale cursor")
	}
}

func TestActivityEndpoint_AfterParam(t *testing.T) {
	srv := newTestServer(t)
	task, _ := srv.store.Create("test", "", "")
	act1, _ := srv.store.AddActivity(task.ID, "evt1", nil)
	srv.store.AddActivity(task.ID, "evt2", nil)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%d/activity?after=%d", task.ID, act1.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var activities []store.Activity
	json.NewDecoder(w.Body).Decode(&activities)
	if len(activities) != 1 || activities[0].EventType != "evt2" {
		t.Errorf("expected 1 activity (evt2), got %d", len(activities))
	}
}

func TestCommentsEndpoint_AfterParam(t *testing.T) {
	srv := newTestServer(t)
	task, _ := srv.store.Create("test", "", "")
	cmt1, _ := srv.store.InsertComment(task.ID, "user", "feedback", "first")
	srv.store.InsertComment(task.ID, "agent", "response", "second")

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%d/comments?after=%d", task.ID, cmt1.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var comments []store.Comment
	json.NewDecoder(w.Body).Decode(&comments)
	if len(comments) != 1 || comments[0].Body != "second" {
		t.Errorf("expected 1 comment (second), got %d", len(comments))
	}
}

// --- Option tests ---

func TestWithHost(t *testing.T) {
	srv := New(WithHost("0.0.0.0"))
	if srv.host != "0.0.0.0" {
		t.Errorf("host = %q, want 0.0.0.0", srv.host)
	}
}

func TestWithPort(t *testing.T) {
	srv := New(WithPort(9999))
	if srv.port != 9999 {
		t.Errorf("port = %d, want 9999", srv.port)
	}
}

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

// --- Middleware tests ---

func TestCSPMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := cspMiddleware(inner)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("expected CSP header to be set")
	}
	if !strings.Contains(csp, "default-src 'none'") {
		t.Errorf("CSP = %q, want default-src 'none'", csp)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("expected X-Content-Type-Options: nosniff")
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("expected X-Frame-Options: DENY")
	}
}

func TestBodyLimitMiddleware_POST(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	handler := bodyLimitMiddleware(10)(inner)

	// POST with body exceeding limit
	body := strings.NewReader("this body is definitely longer than ten bytes")
	req := httptest.NewRequest("POST", "/", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rec.Code)
	}
}

func TestBodyLimitMiddleware_GET_NoLimit(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := bodyLimitMiddleware(10)(inner)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := requestIDMiddleware(inner)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	rid := rec.Header().Get("X-Request-ID")
	if rid == "" {
		t.Error("expected X-Request-ID header")
	}
	if !strings.HasPrefix(rid, "tasks-") {
		t.Errorf("X-Request-ID = %q, want tasks- prefix", rid)
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
	handler := recoveryMiddleware(inner)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 after panic", rec.Code)
	}
}

func TestStatusRecorder_WriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rec, status: 200}
	sr.WriteHeader(http.StatusNotFound)
	if sr.status != 404 {
		t.Errorf("status = %d, want 404", sr.status)
	}
	if rec.Code != 404 {
		t.Errorf("underlying recorder code = %d, want 404", rec.Code)
	}
}

func TestRequestLoggerMiddleware(t *testing.T) {
	mel, _ := metrics.NewEventLogger(t.TempDir(), "test")
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := requestLoggerMiddleware(mel)(inner)

	// Normal request — should be logged.
	req := httptest.NewRequest("GET", "/api/tasks", nil)
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

	// Health request — should be skipped (no logging).
	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRequestLoggerMiddleware_SkipsStream(t *testing.T) {
	mel, _ := metrics.NewEventLogger(t.TempDir(), "test")
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := requestLoggerMiddleware(mel)(inner)

	req := httptest.NewRequest("GET", "/api/stream", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// --- Server lifecycle tests ---

func TestShutdown(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	srv := New(WithStore(s), WithPort(0))

	// Start in background, capture actual port
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	// Double shutdown should not panic
	srv.Shutdown(ctx)
}

// --- handleStream SSE test ---

func TestHandleStream(t *testing.T) {
	srv := newTestServer(t)

	// Use a context with cancel to simulate client disconnect.
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/api/stream", nil).WithContext(ctx)

	// httptest.NewRecorder doesn't implement Flusher — use a custom one.
	rec := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}

	done := make(chan struct{})
	go func() {
		srv.ServeHTTP(rec, req)
		close(done)
	}()

	// Let the handler start and send the connected event.
	time.Sleep(50 * time.Millisecond)

	// Broadcast an event.
	srv.broadcaster.Broadcast(Event{Type: "task.created", Data: `{"id":1}`})
	time.Sleep(50 * time.Millisecond)

	// Cancel context to stop the handler.
	cancel()
	<-done

	body := rec.Body.String()
	if !strings.Contains(body, "event: connected") {
		t.Error("expected connected event in SSE stream")
	}
	if !strings.Contains(body, "event: task.created") {
		t.Error("expected task.created event in SSE stream")
	}
}

// flushRecorder wraps httptest.ResponseRecorder with a Flush method.
type flushRecorder struct {
	*httptest.ResponseRecorder
}

func (f *flushRecorder) Flush() {}

// --- Dependency endpoint tests ---

func TestAddDependency_InvalidBody(t *testing.T) {
	srv := newTestServer(t)
	t1, _ := srv.store.Create("task", "", "")

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%d/dependencies", t1.ID), strings.NewReader("bad json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestListTasks_FilterByStage(t *testing.T) {
	srv := newTestServer(t)
	task, _ := srv.store.Create("filtered", "", "")
	srv.store.Update(task.ID, map[string]interface{}{"stage": "active"})

	req := httptest.NewRequest("GET", "/api/tasks?stage=active", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var tasks []store.Task
	json.NewDecoder(rec.Body).Decode(&tasks)
	if len(tasks) != 1 {
		t.Errorf("expected 1 active task, got %d", len(tasks))
	}
}
