package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

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

func TestStartTask_NotImplemented(t *testing.T) {
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
	if rec.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501", rec.Code)
	}
}
