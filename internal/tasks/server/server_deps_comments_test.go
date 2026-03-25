package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateComment(t *testing.T) {
	srv := newTestServer(t)

	// Create a task first.
	createReq := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(`{"title":"commented task"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	srv.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create task: status = %d", createRec.Code)
	}
	var created struct{ ID int64 `json:"id"` }
	json.NewDecoder(createRec.Body).Decode(&created)

	body := `{"author":"shuri","type":"feedback","body":"looks good"}`
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%d/comments", created.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["id"] == nil {
		t.Error("expected id field in response")
	}
}

func TestCreateComment_MissingFields(t *testing.T) {
	srv := newTestServer(t)

	createReq := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(`{"title":"t"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	srv.ServeHTTP(createRec, createReq)
	var created struct{ ID int64 `json:"id"` }
	json.NewDecoder(createRec.Body).Decode(&created)

	// Missing body field.
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%d/comments", created.ID),
		strings.NewReader(`{"author":"shuri"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCreateComment_DefaultType(t *testing.T) {
	srv := newTestServer(t)

	createReq := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(`{"title":"t"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	srv.ServeHTTP(createRec, createReq)
	var created struct{ ID int64 `json:"id"` }
	json.NewDecoder(createRec.Body).Decode(&created)

	// No type field — should default to "feedback".
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%d/comments", created.ID),
		strings.NewReader(`{"author":"shuri","body":"auto-typed"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAddDependency(t *testing.T) {
	srv := newTestServer(t)

	// Create two tasks.
	var ids [2]int64
	for i, title := range []string{"task-A", "task-B"} {
		req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(`{"title":"`+title+`"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		var t2 struct{ ID int64 `json:"id"` }
		json.NewDecoder(rec.Body).Decode(&t2)
		ids[i] = t2.ID
	}

	// task-A depends on task-B.
	body := fmt.Sprintf(`{"depends_on":%d}`, ids[1])
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%d/dependencies", ids[0]), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAddDependency_MissingField(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(`{"title":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var created struct{ ID int64 `json:"id"` }
	json.NewDecoder(rec.Body).Decode(&created)

	// Empty body — depends_on=0 → bad request.
	depReq := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%d/dependencies", created.ID),
		strings.NewReader(`{}`))
	depReq.Header.Set("Content-Type", "application/json")
	depRec := httptest.NewRecorder()
	srv.ServeHTTP(depRec, depReq)

	if depRec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", depRec.Code)
	}
}

func TestRemoveDependency(t *testing.T) {
	srv := newTestServer(t)

	// Create two tasks.
	var ids [2]int64
	for i, title := range []string{"dep-A", "dep-B"} {
		req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(`{"title":"`+title+`"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		var t2 struct{ ID int64 `json:"id"` }
		json.NewDecoder(rec.Body).Decode(&t2)
		ids[i] = t2.ID
	}

	// Add dependency first.
	body := fmt.Sprintf(`{"depends_on":%d}`, ids[1])
	addReq := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%d/dependencies", ids[0]), strings.NewReader(body))
	addReq.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(httptest.NewRecorder(), addReq)

	// Remove it.
	delReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/tasks/%d/dependencies/%d", ids[0], ids[1]), nil)
	delRec := httptest.NewRecorder()
	srv.ServeHTTP(delRec, delReq)

	if delRec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", delRec.Code, delRec.Body.String())
	}
}

func TestRemoveDependency_BadDepID(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(`{"title":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var created struct{ ID int64 `json:"id"` }
	json.NewDecoder(rec.Body).Decode(&created)

	delReq := httptest.NewRequest("DELETE",
		fmt.Sprintf("/api/tasks/%d/dependencies/not-an-id", created.ID), nil)
	delRec := httptest.NewRecorder()
	srv.ServeHTTP(delRec, delReq)

	if delRec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", delRec.Code)
	}
}
