package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rishav1305/soul/internal/chat/metrics"
	"github.com/rishav1305/soul/internal/chat/session"
)

// --- Mock session store that returns errors on demand ---

type errSessionStore struct {
	listErr     error
	getErr      error
	createErr   error
	deleteErr   error
	messagesErr error
	sessions    map[string]*session.Session
}

func (e *errSessionStore) CreateSession(title string) (*session.Session, error) {
	if e.createErr != nil {
		return nil, e.createErr
	}
	return &session.Session{ID: "new-id", Title: title}, nil
}
func (e *errSessionStore) GetSession(id string) (*session.Session, error) {
	if e.getErr != nil {
		return nil, e.getErr
	}
	if s, ok := e.sessions[id]; ok {
		return s, nil
	}
	return nil, errors.New("not found")
}
func (e *errSessionStore) ListSessions() ([]*session.Session, error) {
	if e.listErr != nil {
		return nil, e.listErr
	}
	return nil, nil
}
func (e *errSessionStore) UpdateSessionTitle(id, title string) (*session.Session, error) {
	return nil, nil
}
func (e *errSessionStore) UpdateSessionStatus(id string, status session.Status) error {
	return nil
}
func (e *errSessionStore) DeleteSession(id string) error {
	if e.deleteErr != nil {
		return e.deleteErr
	}
	return nil
}
func (e *errSessionStore) AddMessage(sessionID, role, content string) (*session.Message, error) {
	return nil, nil
}
func (e *errSessionStore) AddMessageTx(tx *sql.Tx, sessionID, role, content string) (*session.Message, error) {
	return nil, nil
}
func (e *errSessionStore) GetMessages(sessionID string) ([]*session.Message, error) {
	if e.messagesErr != nil {
		return nil, e.messagesErr
	}
	return nil, nil
}
func (e *errSessionStore) RunInTransaction(fn func(tx *sql.Tx) error) error {
	return nil
}
func (e *errSessionStore) ResetUnreadCount(id string) error     { return nil }
func (e *errSessionStore) SetLastMessage(id, content string) error { return nil }
func (e *errSessionStore) SetProduct(sessionID, product string) error { return nil }
func (e *errSessionStore) Close() error { return nil }

// --- Helper to create a metrics logger ---

func newTestMetricsLogger(t *testing.T) *metrics.EventLogger {
	t.Helper()
	mel, err := metrics.NewEventLogger(t.TempDir(), "")
	if err != nil {
		t.Fatalf("metrics.NewEventLogger: %v", err)
	}
	t.Cleanup(func() { _ = mel.Close() })
	return mel
}

// --- requestLoggerMiddleware tests ---

func TestRequestLoggerMiddleware_LogsNormalRequest(t *testing.T) {
	logger := newTestMetricsLogger(t)

	handler := requestLoggerMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}
}

func TestRequestLoggerMiddleware_SkipsHealthEndpoint(t *testing.T) {
	logger := newTestMetricsLogger(t)

	called := false
	handler := requestLoggerMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("handler not called for health endpoint")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRequestLoggerMiddleware_SkipsWSEndpoint(t *testing.T) {
	logger := newTestMetricsLogger(t)

	called := false
	handler := requestLoggerMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/ws", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("handler not called for ws endpoint")
	}
}

func TestRequestLoggerMiddleware_CapturesStatusCode(t *testing.T) {
	logger := newTestMetricsLogger(t)

	handler := requestLoggerMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest("GET", "/api/unknown", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestRequestLoggerMiddleware_DefaultStatusIs200(t *testing.T) {
	logger := newTestMetricsLogger(t)

	handler := requestLoggerMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't call WriteHeader — implicit 200.
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/api/data", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// --- statusRecorder tests ---

func TestStatusRecorder_WriteHeader_Captures(t *testing.T) {
	rec := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rec, status: 200}

	sr.WriteHeader(http.StatusForbidden)

	if sr.status != http.StatusForbidden {
		t.Errorf("status = %d, want %d", sr.status, http.StatusForbidden)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("underlying recorder code = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestStatusRecorder_DefaultStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rec, status: 200}

	// Write body without calling WriteHeader — status should remain 200.
	sr.Write([]byte("hello"))

	if sr.status != 200 {
		t.Errorf("default status = %d, want 200", sr.status)
	}
}

// --- handleGetMessages error paths ---

func TestGetMessages_InternalError(t *testing.T) {
	store := &errSessionStore{
		getErr: errors.New("database connection lost"),
	}
	srv := New(WithPort(0), WithSessionStore(store))

	req := httptest.NewRequest("GET", "/api/sessions/11111111-1111-1111-1111-111111111111/messages", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetMessages_GetMessagesError(t *testing.T) {
	validID := "22222222-2222-2222-2222-222222222222"
	store := &errSessionStore{
		sessions: map[string]*session.Session{
			validID: {ID: validID, Title: "Test"},
		},
		messagesErr: errors.New("messages table corrupted"),
	}
	srv := New(WithPort(0), WithSessionStore(store))

	req := httptest.NewRequest("GET", "/api/sessions/"+validID+"/messages", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "failed to get messages" {
		t.Errorf("error = %q, want 'failed to get messages'", resp["error"])
	}
}

// --- handleListSessions error path ---

func TestListSessions_InternalError(t *testing.T) {
	store := &errSessionStore{
		listErr: errors.New("database locked"),
	}
	srv := New(WithPort(0), WithSessionStore(store))

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- handleCreateSession error path ---

func TestCreateSession_StoreError(t *testing.T) {
	store := &errSessionStore{
		createErr: errors.New("disk full"),
	}
	srv := New(WithPort(0), WithSessionStore(store))

	body := `{"title":"test"}`
	req := httptest.NewRequest("POST", "/api/sessions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- handleDeleteSession error paths ---

func TestDeleteSession_DeleteInternalError(t *testing.T) {
	store := &errSessionStore{
		deleteErr: errors.New("database timeout"),
	}
	srv := New(WithPort(0), WithSessionStore(store))

	req := httptest.NewRequest("DELETE", "/api/sessions/33333333-3333-3333-3333-333333333333", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteSession_DeleteError(t *testing.T) {
	validID := "44444444-4444-4444-4444-444444444444"
	store := &errSessionStore{
		sessions: map[string]*session.Session{
			validID: {ID: validID, Title: "To Delete"},
		},
		deleteErr: errors.New("foreign key constraint"),
	}
	srv := New(WithPort(0), WithSessionStore(store))

	req := httptest.NewRequest("DELETE", "/api/sessions/"+validID, nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- serveIndexHTML edge cases ---

func TestServeIndexHTML_NotFound(t *testing.T) {
	dir := t.TempDir()
	srv := New(WithPort(0), WithStaticDir(dir))

	req := httptest.NewRequest("GET", "/nonexistent-page", nil)
	rec := httptest.NewRecorder()
	srv.serveIndexHTML(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestServeIndexHTML_Exists(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.html")
	os.WriteFile(indexPath, []byte("<!DOCTYPE html><html></html>"), 0644)

	srv := New(WithPort(0), WithStaticDir(dir))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	srv.serveIndexHTML(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}
}

// --- handleModels: stale cache with fallback ---

func TestHandleModels_StaleCacheReturnsStaleOnFetchError(t *testing.T) {
	srv := New(WithPort(0))

	// Seed cache with a model and make it stale.
	srv.modelCache.mu.Lock()
	srv.modelCache.models = []ModelInfo{
		{ID: "claude-haiku-4-5-20251001", Name: "Haiku Cached", MaxTokens: 64000},
	}
	srv.modelCache.fetchedAt = time.Now().Add(-2 * time.Hour) // stale
	srv.modelCache.ttl = 5 * time.Minute
	srv.modelCache.mu.Unlock()

	// No auth → fetchModels will fail → should return stale cache.
	req := httptest.NewRequest("GET", "/api/models", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp modelsResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp.Models) != 1 {
		t.Fatalf("expected 1 cached model, got %d", len(resp.Models))
	}
	if resp.Models[0].Name != "Haiku Cached" {
		t.Errorf("expected cached model name, got %q", resp.Models[0].Name)
	}
}

// --- clientIP edge cases ---

func TestClientIP_NonLoopbackIgnoresXFF(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "192.168.1.10:12345"
	req.Header.Set("X-Forwarded-For", "10.0.0.1")

	ip := clientIP(req)
	if ip != "192.168.1.10" {
		t.Errorf("expected 192.168.1.10 (ignores XFF from non-loopback), got %q", ip)
	}
}

func TestClientIP_LoopbackTrustsXFF(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.50")

	ip := clientIP(req)
	if ip != "203.0.113.50" {
		t.Errorf("expected 203.0.113.50 (trusts XFF from loopback), got %q", ip)
	}
}

func TestClientIP_LoopbackXFFMultipleIPs(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 10.0.0.1")

	ip := clientIP(req)
	if ip != "203.0.113.50" {
		t.Errorf("expected first XFF IP 203.0.113.50, got %q", ip)
	}
}

func TestClientIP_InvalidRemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "not-an-ip"

	ip := clientIP(req)
	if ip != "not-an-ip" {
		t.Errorf("expected raw RemoteAddr when SplitHostPort fails, got %q", ip)
	}
}

// --- setCacheHeaders ---

func TestSetCacheHeaders_IndexHTML(t *testing.T) {
	srv := New(WithPort(0))
	rec := httptest.NewRecorder()
	srv.setCacheHeaders(rec, "index.html")

	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("expected no-cache for index.html, got %q", cc)
	}
}

func TestSetCacheHeaders_HashedAsset(t *testing.T) {
	srv := New(WithPort(0))
	rec := httptest.NewRecorder()
	srv.setCacheHeaders(rec, "main.a1b2c3d4.js")

	cc := rec.Header().Get("Cache-Control")
	if cc != "public, max-age=31536000, immutable" {
		t.Errorf("expected immutable cache for hashed asset, got %q", cc)
	}
}

func TestSetCacheHeaders_NonHashedAsset(t *testing.T) {
	srv := New(WithPort(0))
	rec := httptest.NewRecorder()
	srv.setCacheHeaders(rec, "favicon.ico")

	cc := rec.Header().Get("Cache-Control")
	if cc != "public, max-age=3600" {
		t.Errorf("expected short cache for non-hashed asset, got %q", cc)
	}
}

// --- Shutdown ---

func TestShutdown_Graceful(t *testing.T) {
	srv := New(WithPort(0))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	// Should not panic; httpServer is set in New().
	_ = srv.Shutdown(ctx)
}

// --- rateLimitMiddleware exemptions ---

func TestRateLimitMiddleware_ExemptsHealthEndpoint(t *testing.T) {
	handler := rateLimitMiddleware(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/api/health", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200 for /api/health, got %d", i, rec.Code)
		}
	}
}

func TestRateLimitMiddleware_ExemptsModelsEndpoint(t *testing.T) {
	handler := rateLimitMiddleware(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/api/models", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200 for /api/models, got %d", i, rec.Code)
		}
	}
}

func TestRateLimitMiddleware_ExemptsAuthStatus(t *testing.T) {
	handler := rateLimitMiddleware(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/api/auth/status", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200 for /api/auth/status, got %d", i, rec.Code)
		}
	}
}

// --- WithMetrics integration through full server ---

func TestWithMetrics_RequestLogging(t *testing.T) {
	mel := newTestMetricsLogger(t)
	srv := New(WithPort(0), WithMetrics(mel))

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	// Should log the request without panicking.
	// Status 503 because no session store configured.
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

func TestWithMetrics_HealthSkipsLogging(t *testing.T) {
	mel := newTestMetricsLogger(t)
	srv := New(WithPort(0), WithMetrics(mel))

	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// --- Proxy option coverage ---

type mockHub struct{}

func (m *mockHub) BroadcastJSON(msgType string, data interface{}) {}

func TestWithTasksProxy(t *testing.T) {
	hub := &mockHub{}
	srv := New(WithPort(0), WithTasksProxy(hub))
	if srv.tasksProxy == nil {
		t.Error("expected tasks proxy to be set")
	}
}

func TestWithObserveProxy(t *testing.T) {
	srv := New(WithPort(0), WithObserveProxy("http://127.0.0.1:3010"))
	if srv.observeProxy == nil {
		t.Error("expected observe proxy to be set")
	}
}

func TestNewObserveProxy_InvalidURL(t *testing.T) {
	op := newObserveProxy("://invalid")
	if op != nil {
		t.Error("expected nil for invalid URL")
	}
}

func TestObserveProxy_PathRewriting(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Path", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	op := newObserveProxy(backend.URL)
	if op == nil {
		t.Fatal("expected observe proxy")
	}

	req := httptest.NewRequest("GET", "/api/observe/pillars", nil)
	rec := httptest.NewRecorder()
	op.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestTasksProxy_IsConnected_NilSafe(t *testing.T) {
	var tp *tasksProxy
	if tp.IsConnected() {
		t.Error("nil tasksProxy should return false")
	}
}

func TestTasksProxy_StartSSERelay_NilHub(t *testing.T) {
	tp := &tasksProxy{}
	// Should return immediately without blocking.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tp.StartSSERelay(ctx)
}

// --- handleModels: additional paths ---

func TestHandleModels_ValidCacheHit(t *testing.T) {
	srv := New(WithPort(0))

	// Set valid cache.
	srv.modelCache.mu.Lock()
	srv.modelCache.models = []ModelInfo{
		{ID: "claude-haiku-4-5-20251001", Name: "Haiku", MaxTokens: 64000},
	}
	srv.modelCache.fetchedAt = time.Now() // fresh
	srv.modelCache.ttl = 5 * time.Minute
	srv.modelCache.mu.Unlock()

	req := httptest.NewRequest("GET", "/api/models", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp modelsResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(resp.Models))
	}
	if resp.Models[0].ID != "claude-haiku-4-5-20251001" {
		t.Errorf("model ID = %q", resp.Models[0].ID)
	}
}

// --- handleReauth coverage of the metrics log branch ---

func TestReauth_WithMetrics(t *testing.T) {
	mel := newTestMetricsLogger(t)
	srv := New(WithPort(0), WithMetrics(mel))
	// No auth configured — should still return without panic.
	req := httptest.NewRequest("POST", "/api/reauth", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// --- EnsureStaticDir edge case ---

func TestEnsureStaticDir_MissingDir(t *testing.T) {
	err := EnsureStaticDir("/nonexistent/path/to/static")
	if err == nil {
		t.Error("expected error for nonexistent dir")
	}
}

// --- FileExistsInDir coverage ---

func TestFileExistsInDir_NestedPath(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, "file.txt"), []byte("hi"), 0644)

	exists, info, err := FileExistsInDir(dir, "sub/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("expected file to exist")
	}
	if info == nil {
		t.Error("expected non-nil FileInfo")
	}
}
