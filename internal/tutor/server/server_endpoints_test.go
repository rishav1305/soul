package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rishav1305/soul/internal/tutor/eval"
	"github.com/rishav1305/soul/internal/tutor/store"
)

// newTestServer creates an isolated Server backed by a temp SQLite database.
// The evaluator is set to nil so no Claude API calls are made.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "tutor_test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return New(WithStore(s), WithEvaluator(eval.New(nil)))
}

// --- Dashboard ---

func TestHandleDashboard(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/tutor/dashboard", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/tutor/dashboard: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	// Response must contain readiness or moduleStats fields.
	if _, ok := resp["readiness"]; !ok {
		if _, ok2 := resp["moduleStats"]; !ok2 {
			t.Errorf("expected 'readiness' or 'moduleStats' in response, got keys: %v", resp)
		}
	}
}

// --- Analytics ---

func TestHandleAnalytics(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/tutor/analytics", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/tutor/analytics: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	// Expect last30Days or confidenceGaps.
	if _, ok := resp["last30Days"]; !ok {
		if _, ok2 := resp["confidenceGaps"]; !ok2 {
			t.Errorf("expected 'last30Days' or 'confidenceGaps' in analytics response, got: %v", resp)
		}
	}
}

// --- Topics ---

func TestHandleListTopics(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/tutor/topics", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/tutor/topics: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	// Response should be valid JSON (empty list is OK).
	var resp interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestHandleGetTopic_NotFound(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/tutor/topics/9999", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("GET /api/tutor/topics/9999: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleGetTopic_InvalidID(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/tutor/topics/notanid", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GET /api/tutor/topics/notanid: expected 400, got %d", w.Code)
	}
}

// --- Drill Due ---

func TestHandleDrillDue(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/tutor/drill/due", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/tutor/drill/due: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := resp["due"]; !ok {
		t.Errorf("expected 'due' field in response, got: %v", resp)
	}
	if _, ok := resp["count"]; !ok {
		t.Errorf("expected 'count' field in response, got: %v", resp)
	}
}

// --- Mock Sessions ---

func TestHandleListMocks(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/tutor/mocks", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/tutor/mocks: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	// Valid JSON is enough.
	var resp interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestHandleCreateMock(t *testing.T) {
	srv := newTestServer(t)

	body, _ := json.Marshal(map[string]interface{}{
		"type": "technical",
	})
	req := httptest.NewRequest("POST", "/api/tutor/mocks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("POST /api/tutor/mocks: expected 201 or 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	// Response should contain session with an id.
	session, ok := resp["session"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'session' object in response, got: %v", resp)
	}
	if _, ok := session["id"]; !ok {
		t.Errorf("expected 'id' in session object, got: %v", session)
	}
}

func TestHandleCreateMock_InvalidType(t *testing.T) {
	srv := newTestServer(t)

	body, _ := json.Marshal(map[string]interface{}{
		"type": "unknown_type",
	})
	req := httptest.NewRequest("POST", "/api/tutor/mocks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Server should return 4xx for invalid mock type.
	if w.Code < 400 || w.Code >= 500 {
		t.Errorf("POST /api/tutor/mocks with invalid type: expected 4xx, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleGetMock_Valid(t *testing.T) {
	srv := newTestServer(t)

	// First create a session.
	body, _ := json.Marshal(map[string]interface{}{"type": "behavioral"})
	req := httptest.NewRequest("POST", "/api/tutor/mocks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("setup: POST /api/tutor/mocks: got %d: %s", w.Code, w.Body.String())
	}
	var createResp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&createResp)
	session := createResp["session"].(map[string]interface{})
	idFloat := session["id"].(float64)
	id := int64(idFloat)

	// Now GET the session.
	req = httptest.NewRequest("GET", "/api/tutor/mocks/"+formatID(id), nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/tutor/mocks/%d: expected 200, got %d: %s", id, w.Code, w.Body.String())
	}
	var getResp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&getResp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := getResp["session"]; !ok {
		t.Errorf("expected 'session' in GET response, got: %v", getResp)
	}
	if _, ok := getResp["scores"]; !ok {
		t.Errorf("expected 'scores' in GET response, got: %v", getResp)
	}
}

func TestHandleGetMock_NotFound(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/tutor/mocks/99999", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("GET /api/tutor/mocks/99999: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleGetMock_BadID(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/tutor/mocks/notanid", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GET /api/tutor/mocks/notanid: expected 400, got %d", w.Code)
	}
}

// --- Plan ---

func TestHandleGetPlan_NoActivePlan(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/tutor/plan", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// GET plan returns 200 with exists:false when no plan exists.
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("GET /api/tutor/plan: expected 200 or 404, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestHandleCreatePlan(t *testing.T) {
	srv := newTestServer(t)

	body, _ := json.Marshal(map[string]interface{}{
		"target_role": "SWE",
		"target_date": "2027-01-01",
	})
	req := httptest.NewRequest("POST", "/api/tutor/plan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("POST /api/tutor/plan: expected 201 or 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestHandleCreatePlan_MissingRole(t *testing.T) {
	srv := newTestServer(t)

	body, _ := json.Marshal(map[string]interface{}{
		"target_date": "2027-01-01",
	})
	req := httptest.NewRequest("POST", "/api/tutor/plan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /api/tutor/plan without role: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleCreatePlan_PastDate(t *testing.T) {
	srv := newTestServer(t)

	body, _ := json.Marshal(map[string]interface{}{
		"target_role": "SWE",
		"target_date": "2020-01-01",
	})
	req := httptest.NewRequest("POST", "/api/tutor/plan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /api/tutor/plan with past date: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Tool Execute ---

func TestHandleToolExecute_UnknownTool(t *testing.T) {
	srv := newTestServer(t)

	body, _ := json.Marshal(map[string]interface{}{"key": "value"})
	req := httptest.NewRequest("POST", "/api/tools/nonexistent_tool/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("POST /api/tools/nonexistent_tool/execute: expected 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := resp["error"]; !ok {
		t.Errorf("expected 'error' field in response for unknown tool, got: %v", resp)
	}
}

func TestHandleToolExecute_KnownTool_MockInterview(t *testing.T) {
	srv := newTestServer(t)

	body, _ := json.Marshal(map[string]interface{}{"type": "technical"})
	req := httptest.NewRequest("POST", "/api/tools/mock_interview/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/tools/mock_interview/execute: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp["tool"] != "mock_interview" {
		t.Errorf("expected tool=mock_interview, got %v", resp["tool"])
	}
}

func TestHandleToolExecute_Progress(t *testing.T) {
	srv := newTestServer(t)

	body, _ := json.Marshal(map[string]interface{}{"view": "dashboard"})
	req := httptest.NewRequest("POST", "/api/tools/progress/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/tools/progress/execute: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleToolExecute_Planner(t *testing.T) {
	srv := newTestServer(t)

	body, _ := json.Marshal(map[string]interface{}{"action": "get"})
	req := httptest.NewRequest("POST", "/api/tools/planner/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/tools/planner/execute: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleToolExecute_InvalidJSON(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("POST", "/api/tools/progress/execute", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

// --- Middleware Tests ---

func TestBodyLimitMiddleware(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "tutor_test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Build a full server with middleware chain (not just mux).
	srv := New(WithStore(s))
	handler := srv.httpServer.Handler

	// Build valid JSON body > 64KB so MaxBytesReader fires mid-parse and
	// the request is rejected with an error status (400 or 413).
	bigValue := strings.Repeat("a", 64*1024)
	oversizedBody, _ := json.Marshal(map[string]string{"data": bigValue})
	req := httptest.NewRequest("POST", "/api/tutor/mocks", bytes.NewReader(oversizedBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// The body limit middleware should reject requests exceeding 64KB.
	// The server returns 400 (json decode error wrapping MaxBytesError) or
	// 413 (RequestEntityTooLarge) depending on handler implementation.
	if w.Code != http.StatusRequestEntityTooLarge && w.Code != http.StatusBadRequest {
		t.Errorf("body > 64KB: expected 413 or 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCSPMiddleware(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "tutor_test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	srv := New(WithStore(s))
	handler := srv.httpServer.Handler

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Errorf("expected Content-Security-Policy header, got none")
	}
	xct := w.Header().Get("X-Content-Type-Options")
	if xct == "" {
		t.Errorf("expected X-Content-Type-Options header, got none")
	}
	xfo := w.Header().Get("X-Frame-Options")
	if xfo == "" {
		t.Errorf("expected X-Frame-Options header, got none")
	}
}

// --- Drill Due (no reviews exist) ---

func TestHandleDrillDue_Empty(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/tutor/drill/due", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	count, ok := resp["count"].(float64)
	if !ok {
		t.Fatalf("expected numeric count, got %T: %v", resp["count"], resp["count"])
	}
	if count != 0 {
		t.Errorf("expected 0 due items on empty db, got %v", count)
	}
}

// --- Mock Answer (store-only, no Claude) ---

func TestHandleMockAnswer(t *testing.T) {
	srv := newTestServer(t)

	// Create a session first.
	body, _ := json.Marshal(map[string]interface{}{"type": "technical"})
	req := httptest.NewRequest("POST", "/api/tutor/mocks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("setup POST /api/tutor/mocks: got %d: %s", w.Code, w.Body.String())
	}
	var createResp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&createResp)
	session := createResp["session"].(map[string]interface{})
	id := int64(session["id"].(float64))

	// Submit answer (score + feedback).
	answerBody, _ := json.Marshal(map[string]interface{}{
		"overall_score": 85.0,
		"feedback_json": `{"summary":"Good"}`,
		"scores": []map[string]interface{}{
			{"dimension": "communication", "score": 90.0},
		},
	})
	req = httptest.NewRequest("POST", "/api/tutor/mocks/"+formatID(id)+"/answer", bytes.NewReader(answerBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /api/tutor/mocks/%d/answer: expected 200, got %d: %s", id, w.Code, w.Body.String())
	}
}

func TestHandleMockAnswer_NotFound(t *testing.T) {
	srv := newTestServer(t)

	body, _ := json.Marshal(map[string]interface{}{
		"overall_score": 75.0,
		"feedback_json": "{}",
	})
	req := httptest.NewRequest("POST", "/api/tutor/mocks/99999/answer", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("POST /api/tutor/mocks/99999/answer: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Update Plan ---

func TestHandleUpdatePlan_NoActivePlan(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("PATCH", "/api/tutor/plan", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusOK {
		t.Errorf("PATCH /api/tutor/plan with no plan: expected 404 or 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Helper ---

func formatID(id int64) string {
	return itoa(id)
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 20)
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	if neg {
		buf = append(buf, '-')
	}
	// reverse
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
