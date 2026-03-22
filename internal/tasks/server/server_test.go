package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rishav1305/soul-v2/internal/tasks/executor"
	"github.com/rishav1305/soul-v2/internal/tasks/store"
	"github.com/rishav1305/soul-v2/pkg/events"
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
