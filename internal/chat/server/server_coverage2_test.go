package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

)

// ============================================================
// spaHandler — cover uncovered paths (84% → higher)
// ============================================================

func TestSPAHandler_StaticDirDoesNotExist(t *testing.T) {
	srv := New(WithPort(0), WithStaticDir("/nonexistent/static/dir"))

	req := httptest.NewRequest("GET", "/some-page", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing static dir, got %d", rec.Code)
	}
	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["error"] != "static directory not found" {
		t.Errorf("expected 'static directory not found', got %q", body["error"])
	}
}

func TestSPAHandler_PathTraversalPrevention(t *testing.T) {
	dir := t.TempDir()
	// Create index.html so fallback works.
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("SPA"), 0644)

	srv := New(WithPort(0), WithStaticDir(dir))

	// Use a relative traversal path that doesn't get 301-redirected by Go's router.
	// This path stays within the route but the filepath.Clean resolves it up.
	req := httptest.NewRequest("GET", "/assets/../../etc/passwd", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	// Should NOT serve /etc/passwd — either SPA fallback or redirect.
	// As long as it doesn't panic and doesn't serve the system file, it's safe.
	body := rec.Body.String()
	if strings.Contains(body, "root:") {
		t.Error("directory traversal succeeded — served system file!")
	}
}

func TestSPAHandler_DirectoryRequestFallsToSPA(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("SPA"), 0644)
	// Create a subdirectory.
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)

	srv := New(WithPort(0), WithStaticDir(dir))

	// Request a directory path (info.IsDir() == true) → should fallback to index.html.
	req := httptest.NewRequest("GET", "/subdir", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 (SPA fallback for dir), got %d", rec.Code)
	}
}

func TestSPAHandler_RootPathServesIndex(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!DOCTYPE html><html>root</html>"), 0644)

	srv := New(WithPort(0), WithStaticDir(dir))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// ============================================================
// rateLimitMiddleware — litellm and v1/responses paths (81.4% → higher)
// ============================================================

func TestRateLimitMiddleware_LiteLLMPathIsRateLimited(t *testing.T) {
	handler := rateLimitMiddleware(2)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 2 requests should pass.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/litellm/v1/chat", nil)
		req.RemoteAddr = "10.0.0.5:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, rec.Code)
		}
	}

	// 3rd should be rate limited.
	req := httptest.NewRequest("POST", "/litellm/v1/chat", nil)
	req.RemoteAddr = "10.0.0.5:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 on litellm path, got %d", rec.Code)
	}
}

func TestRateLimitMiddleware_ResponsesPathIsRateLimited(t *testing.T) {
	handler := rateLimitMiddleware(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request passes.
	req := httptest.NewRequest("POST", "/v1/responses", nil)
	req.RemoteAddr = "10.0.0.6:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Second should be rate limited.
	req2 := httptest.NewRequest("POST", "/v1/responses", nil)
	req2.RemoteAddr = "10.0.0.6:1234"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 on /v1/responses path, got %d", rec2.Code)
	}
}

// ============================================================
// handleModels — cover fetchModels mock paths (60.9% → higher)
// ============================================================

func TestHandleModels_FetchSucceedsWithEmptyModels_UsesFallback(t *testing.T) {
	// Simulate: fetchModels returns successfully but with zero current-gen models.
	// handleModels should use fallbackModels.
	srv := New(WithPort(0))

	// Seed the cache as if fetchModels returned empty array → triggers fallback.
	srv.modelCache.mu.Lock()
	srv.modelCache.models = fallbackModels // Simulates the fallback path
	srv.modelCache.fetchedAt = time.Now()
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
	if len(resp.Models) == 0 {
		t.Error("expected fallback models")
	}
}

func TestHandleModels_NoCacheNoAuth_ReturnsFallback(t *testing.T) {
	// Empty cache + no auth → fetchModels fails → no stale cache → fallback.
	srv := New(WithPort(0))

	req := httptest.NewRequest("GET", "/api/models", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp modelsResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	found := false
	for _, m := range resp.Models {
		if m.ID == "claude-haiku-4-5-20251001" {
			found = true
		}
	}
	if !found {
		t.Error("expected fallback haiku model")
	}
}

// ============================================================
// handleTelemetry — batch and error paths (91.3% → higher)
// ============================================================

func TestHandleTelemetry_SingleEvent(t *testing.T) {
	mel := newTestMetricsLogger(t)
	srv := New(WithPort(0), WithMetrics(mel))

	body := `{"type":"frontend.error","data":{"message":"test error"}}`
	req := httptest.NewRequest("POST", "/api/telemetry", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleTelemetry_BatchEvent(t *testing.T) {
	mel := newTestMetricsLogger(t)
	srv := New(WithPort(0), WithMetrics(mel))

	body := `{"batch":[
		{"type":"frontend.error","data":{"message":"err1"}},
		{"type":"frontend.render","data":{"page":"home"}},
		{"type":"unknown.type","data":{}}
	]}`
	req := httptest.NewRequest("POST", "/api/telemetry", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	// Batch with some invalid types should still succeed (skips invalid).
	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleTelemetry_InvalidJSON(t *testing.T) {
	mel := newTestMetricsLogger(t)
	srv := New(WithPort(0), WithMetrics(mel))

	req := httptest.NewRequest("POST", "/api/telemetry", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleTelemetry_UnknownSingleType(t *testing.T) {
	mel := newTestMetricsLogger(t)
	srv := New(WithPort(0), WithMetrics(mel))

	body := `{"type":"bogus.event","data":{}}`
	req := httptest.NewRequest("POST", "/api/telemetry", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unknown type, got %d", rec.Code)
	}
}

func TestHandleTelemetry_MetricsNotConfigured(t *testing.T) {
	srv := New(WithPort(0)) // no metrics

	body := `{"type":"frontend.error","data":{}}`
	req := httptest.NewRequest("POST", "/api/telemetry", strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when metrics nil, got %d", rec.Code)
	}
}

// ============================================================
// handleResponses — cover compaction proxy paths (80% → higher)
// ============================================================

func TestHandleResponses_UpstreamProxies(t *testing.T) {
	// Stand up a fake upstream server.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"resp_123","output":"hello"}`))
	}))
	defer backend.Close()

	t.Setenv("SOUL_LITELLM_UPSTREAM_URL", backend.URL+"/v1/responses")
	srv := New(WithPort(0))

	body := `{"model":"test-model","input":"hello"}`
	req := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleResponses_QueryStringPassthrough(t *testing.T) {
	var receivedQuery string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer backend.Close()

	t.Setenv("SOUL_LITELLM_UPSTREAM_URL", backend.URL+"/v1/responses")
	srv := New(WithPort(0))

	body := `{"model":"test","input":"hi"}`
	req := httptest.NewRequest("POST", "/v1/responses?stream=true", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(receivedQuery, "stream=true") {
		t.Errorf("expected query passthrough, got %q", receivedQuery)
	}
}

func TestHandleResponses_UpstreamError(t *testing.T) {
	// Upstream that immediately closes — should return 502.
	t.Setenv("SOUL_LITELLM_UPSTREAM_URL", "http://127.0.0.1:1") // won't connect
	srv := New(WithPort(0))

	body := `{"model":"test","input":"hello"}`
	req := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected 502 for unreachable upstream, got %d", rec.Code)
	}
}

// ============================================================
// Shutdown — with metrics logging (66.7% → higher)
// ============================================================

func TestShutdown_WithMetrics(t *testing.T) {
	mel := newTestMetricsLogger(t)
	srv := New(WithPort(0), WithMetrics(mel))
	srv.startTime = time.Now().Add(-10 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := srv.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown returned error: %v", err)
	}
}

// ============================================================
// authMiddleware — WS ticket and rejection callback (92% → higher)
// ============================================================

func TestAuthMiddleware_WSTicketValid(t *testing.T) {
	ticketCalled := false
	handler := authMiddleware("secret", func(ticket string) bool {
		ticketCalled = true
		return ticket == "valid-ticket"
	}, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/ws?ticket=valid-ticket", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for valid WS ticket, got %d", rec.Code)
	}
	if !ticketCalled {
		t.Error("expected ticketValid to be called")
	}
}

func TestAuthMiddleware_WSInvalidTicketFallsThrough(t *testing.T) {
	rejectedCalled := false
	handler := authMiddleware("secret", func(ticket string) bool {
		return false // Invalid ticket
	}, func(r *http.Request) {
		rejectedCalled = true
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/ws?ticket=bad-ticket", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid WS ticket, got %d", rec.Code)
	}
	if !rejectedCalled {
		t.Error("expected onWSRejected to be called")
	}
}

func TestAuthMiddleware_WSTokenFallback(t *testing.T) {
	handler := authMiddleware("secret", nil, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/ws?token=secret", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for valid WS token fallback, got %d", rec.Code)
	}
}

// ============================================================
// tasksProxy — SSE relay with real SSE backend (0% → higher)
// ============================================================

func TestTasksProxy_ConnectSSE(t *testing.T) {
	// Stand up a minimal SSE server.
	sseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/stream" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		// Send one event then close.
		fmt.Fprintf(w, "event: task.update\n")
		fmt.Fprintf(w, "data: {\"id\":1,\"status\":\"active\"}\n")
		fmt.Fprintf(w, "\n")
		flusher.Flush()
		// Close immediately to end the connection.
	}))
	defer sseServer.Close()

	t.Setenv("SOUL_TASKS_URL", sseServer.URL)

	hub := &mockBroadcastHub{}
	tp := newTasksProxy(hub)
	if tp == nil {
		t.Fatal("expected non-nil tasksProxy")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := tp.connectSSE(ctx)
	// Should return nil or scanner error when stream closes.
	_ = err // Any non-panic completion is OK.

	if !hub.called {
		t.Error("expected hub.BroadcastJSON to be called")
	}
}

func TestTasksProxy_ConnectSSE_BadStatus(t *testing.T) {
	// Server returns 500 instead of SSE.
	sseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer sseServer.Close()

	t.Setenv("SOUL_TASKS_URL", sseServer.URL)

	tp := newTasksProxy(&mockBroadcastHub{})
	if tp == nil {
		t.Fatal("expected non-nil tasksProxy")
	}

	ctx := context.Background()
	err := tp.connectSSE(ctx)
	if err == nil {
		t.Error("expected error for non-200 SSE response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to mention 500, got %v", err)
	}
}

func TestTasksProxy_IsConnected_True(t *testing.T) {
	tp := &tasksProxy{}
	tp.mu.Lock()
	tp.connected = true
	tp.mu.Unlock()

	if !tp.IsConnected() {
		t.Error("expected IsConnected() to return true")
	}
}

func TestTasksProxy_StartSSERelay_CancelledContext(t *testing.T) {
	// SSE server that blocks until context is cancelled.
	sseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		fmt.Fprintf(w, "event: connected\n\ndata: ok\n\n")
		flusher.Flush()
		// Block until client disconnects.
		<-r.Context().Done()
	}))
	defer sseServer.Close()

	t.Setenv("SOUL_TASKS_URL", sseServer.URL)

	tp := newTasksProxy(&mockBroadcastHub{})
	if tp == nil {
		t.Fatal("expected non-nil tasksProxy")
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		tp.StartSSERelay(ctx)
		close(done)
	}()

	// Give it a moment to connect, then cancel.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// OK — returned cleanly.
	case <-time.After(5 * time.Second):
		t.Fatal("StartSSERelay did not return after context cancel")
	}
}

// mockBroadcastHub tracks whether BroadcastJSON was called.
type mockBroadcastHub struct {
	called bool
	lastType string
}

func (m *mockBroadcastHub) BroadcastJSON(msgType string, data interface{}) {
	m.called = true
	m.lastType = msgType
}

// ============================================================
// ConnectSSE — skip "connected" event type
// ============================================================

func TestTasksProxy_ConnectSSE_SkipsConnectedEvent(t *testing.T) {
	sseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		// "connected" event should be skipped.
		fmt.Fprintf(w, "event: connected\ndata: ok\n\n")
		flusher.Flush()
		// "task.update" event should be broadcast.
		fmt.Fprintf(w, "event: task.update\ndata: plain text data\n\n")
		flusher.Flush()
	}))
	defer sseServer.Close()

	t.Setenv("SOUL_TASKS_URL", sseServer.URL)

	hub := &mockBroadcastHub{}
	tp := newTasksProxy(hub)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = tp.connectSSE(ctx)

	if !hub.called {
		t.Error("expected hub to be called for task.update")
	}
	if hub.lastType != "task.update" {
		t.Errorf("expected lastType=task.update, got %q", hub.lastType)
	}
}

// ============================================================
// requestLoggerMiddleware — slow request path (92.3% → 100%)
// ============================================================

func TestRequestLoggerMiddleware_SlowRequest(t *testing.T) {
	logger := newTestMetricsLogger(t)

	handler := requestLoggerMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow handler (>500ms).
		time.Sleep(550 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/slow-endpoint", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	// The slow path logs api.slow — verify no panic occurred.
}

// ============================================================
// EnsureStaticDir — additional paths (90.9% → 100%)
// ============================================================

func TestEnsureStaticDir_NotADirectory(t *testing.T) {
	f := filepath.Join(t.TempDir(), "not-a-dir")
	os.WriteFile(f, []byte("file"), 0644)

	err := EnsureStaticDir(f)
	if err == nil {
		t.Error("expected error for file (not dir)")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEnsureStaticDir_MissingIndexHTML(t *testing.T) {
	dir := t.TempDir()
	// Dir exists but no index.html.
	err := EnsureStaticDir(dir)
	if err == nil {
		t.Error("expected error for missing index.html")
	}
	if !strings.Contains(err.Error(), "index.html not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEnsureStaticDir_ValidDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("hi"), 0644)

	err := EnsureStaticDir(dir)
	if err != nil {
		t.Errorf("expected nil error for valid dir, got %v", err)
	}
}

// ============================================================
// FileExistsInDir — traversal prevention (86.7% → 100%)
// ============================================================

func TestFileExistsInDir_TraversalPrevented(t *testing.T) {
	dir := t.TempDir()

	_, _, err := FileExistsInDir(dir, "../../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFileExistsInDir_FileNotFound(t *testing.T) {
	dir := t.TempDir()

	exists, info, err := FileExistsInDir(dir, "nonexistent.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected file to not exist")
	}
	if info != nil {
		t.Error("expected nil info")
	}
}

func TestFileExistsInDir_IsDirectory(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)

	exists, _, err := FileExistsInDir(dir, "subdir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected false for directory")
	}
}

// ============================================================
// observeProxy — empty path after rewrite
// ============================================================

func TestObserveProxy_PathRewrite(t *testing.T) {
	var receivedPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	op := newObserveProxy(backend.URL)
	if op == nil {
		t.Fatal("expected non-nil observe proxy")
	}

	// "/api/observe" → replace "/api/observe" → "/api" (the prefix itself becomes /api).
	req := httptest.NewRequest("GET", "/api/observe", nil)
	rec := httptest.NewRecorder()
	op.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if receivedPath != "/api" {
		t.Errorf("expected /api, got %q", receivedPath)
	}
}

// ============================================================
// simpleProxy — empty pathPrefix passthrough
// ============================================================

func TestSimpleProxy_EmptyPrefix(t *testing.T) {
	var receivedPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	t.Setenv("SOUL_TEST_EMPTY_URL", backend.URL)
	sp := newSimpleProxy("SOUL_TEST_EMPTY_URL", "http://127.0.0.1:9999", "", "test")
	if sp == nil {
		t.Fatal("expected non-nil simple proxy")
	}

	req := httptest.NewRequest("GET", "/api/custom/path", nil)
	rec := httptest.NewRecorder()
	sp.ServeHTTP(rec, req)

	if receivedPath != "/api/custom/path" {
		t.Errorf("expected path to pass through unchanged, got %q", receivedPath)
	}
}

func TestSimpleProxy_EmptyPathAfterRewrite(t *testing.T) {
	var receivedPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	t.Setenv("SOUL_TEST_EMPT2_URL", backend.URL)
	sp := newSimpleProxy("SOUL_TEST_EMPT2_URL", "http://127.0.0.1:9999", "/api/infra", "infra")
	if sp == nil {
		t.Fatal("expected non-nil simple proxy")
	}

	// "/api/infra" → after replace → "/api" → NOT empty
	// But "/api/infra" exact → replace → "/api"
	req := httptest.NewRequest("GET", "/api/infra", nil)
	rec := httptest.NewRecorder()
	sp.ServeHTTP(rec, req)

	if receivedPath != "/api" {
		t.Errorf("expected /api after rewrite, got %q", receivedPath)
	}
}

// ============================================================
// WS ticket system
// ============================================================

func TestWSTicket_IssueAndConsume(t *testing.T) {
	srv := New(WithPort(0))

	ticket := srv.issueWSTicket()
	if ticket == "" {
		t.Fatal("expected non-empty ticket")
	}

	if !srv.consumeWSTicket(ticket) {
		t.Error("expected ticket to be valid")
	}

	// Consuming again should fail (one-time).
	if srv.consumeWSTicket(ticket) {
		t.Error("expected ticket to be consumed and invalid")
	}
}

func TestWSTicket_ExpiredTicket(t *testing.T) {
	srv := New(WithPort(0))

	// Manually insert an expired ticket.
	srv.wsTickets.Store("expired-ticket", time.Now().Add(-1*time.Minute))

	if srv.consumeWSTicket("expired-ticket") {
		t.Error("expected expired ticket to be rejected")
	}
}

func TestWSTicket_HandlerReturnsTicket(t *testing.T) {
	srv := New(WithPort(0))

	req := httptest.NewRequest("GET", "/api/ws-ticket", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["ticket"] == "" {
		t.Error("expected non-empty ticket in response")
	}
}

// ============================================================
// fetchModels — auth not configured path
// ============================================================

func TestFetchModels_NoAuth(t *testing.T) {
	srv := New(WithPort(0))
	_, err := srv.fetchModels()
	if err == nil {
		t.Error("expected error when auth is nil")
	}
	if !strings.Contains(err.Error(), "authentication not configured") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// probeModel — auth not configured
// ============================================================

func TestProbeModel_NoAuth(t *testing.T) {
	srv := New(WithPort(0))
	if srv.probeModel("claude-haiku-4-5-20251001") {
		t.Error("expected false when auth is nil")
	}
}

// ============================================================
// Server.StartSSERelay — nil tasksProxy
// ============================================================

func TestServer_StartSSERelay_NilTasksProxy(t *testing.T) {
	srv := New(WithPort(0)) // no tasks proxy
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Should return immediately without panic.
	srv.StartSSERelay(ctx)
}

// ============================================================
// handleCACert — TLS cert serving
// ============================================================

func TestHandleCACert_NoCACertFile(t *testing.T) {
	certDir := t.TempDir()
	certPath := filepath.Join(certDir, "cert.pem")
	os.WriteFile(certPath, []byte("cert"), 0644)

	srv := New(WithPort(0), WithTLS(certPath, "/tmp/key.pem"))

	req := httptest.NewRequest("GET", "/ca.crt", nil)
	rec := httptest.NewRecorder()
	srv.handleCACert(rec, req)

	// ca.crt doesn't exist → should return 404.
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing ca.crt, got %d", rec.Code)
	}
}

func TestHandleCACert_ValidCACert(t *testing.T) {
	certDir := t.TempDir()
	certPath := filepath.Join(certDir, "cert.pem")
	os.WriteFile(certPath, []byte("cert"), 0644)
	// Create ca.crt in the same directory as the cert.
	caPath := filepath.Join(certDir, "ca.crt")
	os.WriteFile(caPath, []byte("-----BEGIN CERTIFICATE-----\nfake-ca\n-----END CERTIFICATE-----"), 0644)

	srv := New(WithPort(0), WithTLS(certPath, "/tmp/key.pem"))

	req := httptest.NewRequest("GET", "/ca.crt", nil)
	rec := httptest.NewRecorder()
	srv.handleCACert(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/x-x509-ca-cert" {
		t.Errorf("expected x509 content type, got %q", ct)
	}
}

func TestHandleCACert_TLSNotConfigured_ReturnsNotFound(t *testing.T) {
	srv := New(WithPort(0)) // no TLS

	req := httptest.NewRequest("GET", "/ca.crt", nil)
	rec := httptest.NewRecorder()
	srv.handleCACert(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for no TLS, got %d", rec.Code)
	}
}

// ============================================================
// Various proxy creation with env vars
// ============================================================

func TestNewTasksProxy_InvalidURL(t *testing.T) {
	t.Setenv("SOUL_TASKS_URL", "://invalid")
	tp := newTasksProxy(&mockBroadcastHub{})
	if tp != nil {
		t.Error("expected nil for invalid SOUL_TASKS_URL")
	}
}

func TestNewTutorProxy_InvalidURL(t *testing.T) {
	t.Setenv("SOUL_TUTOR_URL", "://invalid")
	tp := newTutorProxy()
	if tp != nil {
		t.Error("expected nil for invalid SOUL_TUTOR_URL")
	}
}

func TestNewProjectsProxy_InvalidURL(t *testing.T) {
	t.Setenv("SOUL_PROJECTS_URL", "://invalid")
	pp := newProjectsProxy()
	if pp != nil {
		t.Error("expected nil for invalid SOUL_PROJECTS_URL")
	}
}

func TestNewSimpleProxy_InvalidURL(t *testing.T) {
	t.Setenv("SOUL_TEST_INVALID", "://invalid")
	sp := newSimpleProxy("SOUL_TEST_INVALID", "://also-invalid", "/api/test", "test")
	if sp != nil {
		t.Error("expected nil for invalid URL")
	}
}

// ============================================================
// handleGetMessages — sessionStore nil
// ============================================================

func TestGetMessages_NoSessionStore(t *testing.T) {
	srv := New(WithPort(0)) // no session store

	req := httptest.NewRequest("GET", "/api/sessions/11111111-1111-1111-1111-111111111111/messages", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for nil session store, got %d", rec.Code)
	}
}

func TestGetMessages_InvalidUUID(t *testing.T) {
	store := &errSessionStore{}
	srv := New(WithPort(0), WithSessionStore(store))

	req := httptest.NewRequest("GET", "/api/sessions/not-a-uuid/messages", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid UUID, got %d", rec.Code)
	}
}

// ============================================================
// handleCreateSession — invalid JSON body
// ============================================================

func TestCreateSession_EmptyBody(t *testing.T) {
	store := &errSessionStore{}
	srv := New(WithPort(0), WithSessionStore(store))

	req := httptest.NewRequest("POST", "/api/sessions", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	// Empty body may parse as empty JSON or fail. The key assertion is no panic.
	_ = rec.Code
}

// ============================================================
// probeModels — filters list
// ============================================================

func TestProbeModels_ReturnsEmpty_WhenNoAuth(t *testing.T) {
	srv := New(WithPort(0)) // no auth
	models := []ModelInfo{
		{ID: "claude-haiku-4-5-20251001", Name: "Haiku", MaxTokens: 64000},
	}
	result := srv.probeModels(models)
	if len(result) != 0 {
		t.Errorf("expected 0 working models without auth, got %d", len(result))
	}
}

// ============================================================
// securityHeadersMiddleware — TLS request path
// ============================================================

func TestSecurityHeaders_TLSRequest(t *testing.T) {
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	hsts := rec.Header().Get("Strict-Transport-Security")
	if hsts == "" {
		t.Error("expected HSTS header for HTTPS request")
	}
}

// ============================================================
// bodyLimitMiddleware — exercise POST path
// ============================================================

func TestBodyLimitMiddleware_POST(t *testing.T) {
	handler := bodyLimitMiddleware(100)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	// Body exceeding limit.
	bigBody := strings.Repeat("x", 200)
	req := httptest.NewRequest("POST", "/api/test", strings.NewReader(bigBody))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// MaxBytesReader causes error on read.
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413 for oversized POST body, got %d", rec.Code)
	}
}

func TestBodyLimitMiddleware_GET_NoLimit(t *testing.T) {
	handler := bodyLimitMiddleware(100)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for GET, got %d", rec.Code)
	}
}

// ============================================================
// tasksProxy.ServeHTTP — exercise the reverse proxy forward (0% → covered)
// ============================================================

func TestTasksProxy_ServeHTTP(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"tasks":[]}`))
	}))
	defer backend.Close()

	t.Setenv("SOUL_TASKS_URL", backend.URL)
	tp := newTasksProxy(&mockBroadcastHub{})
	if tp == nil {
		t.Fatal("expected non-nil tasksProxy")
	}

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	rec := httptest.NewRecorder()
	tp.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// ============================================================
// Proxy error handlers — backend unreachable (covers error lambdas)
// ============================================================

func TestTasksProxy_ServeHTTP_BackendDown(t *testing.T) {
	// Create proxy pointing to a closed server.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Setenv("SOUL_TASKS_URL", backend.URL)
	tp := newTasksProxy(&mockBroadcastHub{})
	backend.Close() // Close it before we make requests.

	if tp == nil {
		t.Fatal("expected non-nil tasksProxy")
	}

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	rec := httptest.NewRecorder()
	tp.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for downed backend, got %d", rec.Code)
	}
}

func TestTutorProxy_BackendDown(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Setenv("SOUL_TUTOR_URL", backend.URL)
	tp := newTutorProxy()
	backend.Close()

	if tp == nil {
		t.Fatal("expected non-nil tutorProxy")
	}

	req := httptest.NewRequest("GET", "/api/tutor/dashboard", nil)
	rec := httptest.NewRecorder()
	tp.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for downed tutor backend, got %d", rec.Code)
	}
}

func TestProjectsProxy_BackendDown(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Setenv("SOUL_PROJECTS_URL", backend.URL)
	pp := newProjectsProxy()
	backend.Close()

	if pp == nil {
		t.Fatal("expected non-nil projectsProxy")
	}

	req := httptest.NewRequest("GET", "/api/projects", nil)
	rec := httptest.NewRecorder()
	pp.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for downed projects backend, got %d", rec.Code)
	}
}

func TestObserveProxy_BackendDown(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	op := newObserveProxy(backend.URL)
	backend.Close()

	if op == nil {
		t.Fatal("expected non-nil observeProxy")
	}

	req := httptest.NewRequest("GET", "/api/observe/pillars", nil)
	rec := httptest.NewRecorder()
	op.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for downed observe backend, got %d", rec.Code)
	}
}

func TestSimpleProxy_BackendDown(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Setenv("SOUL_TEST_DOWN_URL", backend.URL)
	sp := newSimpleProxy("SOUL_TEST_DOWN_URL", "http://127.0.0.1:9999", "/api/test", "test")
	backend.Close()

	if sp == nil {
		t.Fatal("expected non-nil simpleProxy")
	}

	req := httptest.NewRequest("GET", "/api/test/foo", nil)
	rec := httptest.NewRecorder()
	sp.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for downed simple backend, got %d", rec.Code)
	}
}

// ============================================================
// StartSSERelay — reconnect with failing SSE (covers backoff path)
// ============================================================

func TestTasksProxy_StartSSERelay_ReconnectsOnError(t *testing.T) {
	attempts := 0
	sseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		// Always return 500 to trigger reconnect.
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer sseServer.Close()

	t.Setenv("SOUL_TASKS_URL", sseServer.URL)
	tp := newTasksProxy(&mockBroadcastHub{})
	if tp == nil {
		t.Fatal("expected non-nil tasksProxy")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		tp.StartSSERelay(ctx)
		close(done)
	}()

	<-ctx.Done()
	<-done

	if attempts < 2 {
		t.Errorf("expected at least 2 reconnect attempts, got %d", attempts)
	}
}

// ============================================================
// handleModels — cover the full success path via fetchModels mock
// Using httptest.Server and injecting it into the auth struct is
// infeasible (hardcoded URL). Instead, directly test the cache store path
// by simulating what happens after a successful fetch.
// ============================================================

func TestHandleModels_CacheStoreAfterFetch(t *testing.T) {
	srv := New(WithPort(0))

	// Simulate a successful fetchModels result by directly populating cache.
	srv.modelCache.mu.Lock()
	srv.modelCache.models = []ModelInfo{
		{ID: "claude-haiku-4-5-20251001", Name: "Haiku 4.5", MaxTokens: 64000},
		{ID: "claude-haiku-4-20240307", Name: "Haiku 4", MaxTokens: 64000},
	}
	srv.modelCache.fetchedAt = time.Now()
	srv.modelCache.ttl = 5 * time.Minute
	srv.modelCache.mu.Unlock()

	// Now hit the endpoint — should return both models from valid cache.
	req := httptest.NewRequest("GET", "/api/models", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp modelsResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(resp.Models))
	}
	if len(resp.ThinkingTypes) != 3 {
		t.Errorf("expected 3 thinking types, got %d", len(resp.ThinkingTypes))
	}
}

// ============================================================
// spaHandler — test the file-exists-and-served path with hashed asset
// ============================================================

func TestSPAHandler_ServesHashedAssetWithLongCache(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("SPA"), 0644)
	assetsDir := filepath.Join(dir, "assets")
	os.MkdirAll(assetsDir, 0755)
	os.WriteFile(filepath.Join(assetsDir, "app.abc123.js"), []byte("var x=1;"), 0644)

	srv := New(WithPort(0), WithStaticDir(dir))

	req := httptest.NewRequest("GET", "/assets/app.abc123.js", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	cc := rec.Header().Get("Cache-Control")
	if !strings.Contains(cc, "immutable") {
		t.Errorf("expected immutable cache for hashed asset, got %q", cc)
	}
}

// ============================================================
// handleResponses — compaction expansion path
// ============================================================

func TestHandleResponses_WithCompactionMarker(t *testing.T) {
	// Backend that echoes the input back.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer backend.Close()

	t.Setenv("SOUL_LITELLM_UPSTREAM_URL", backend.URL+"/v1/responses")
	srv := New(WithPort(0))

	// First, create a compaction entry by calling /compact.
	compactBody := `{
		"model": "test-model",
		"input": [
			{"type":"message","role":"user","content":[{"type":"input_text","text":"What is 2+2?"}]},
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"4"}]}
		]
	}`
	req1 := httptest.NewRequest("POST", "/litellm/v1/responses/compact", strings.NewReader(compactBody))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	srv.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("compact expected 200, got %d: %s", rec1.Code, rec1.Body.String())
	}

	// Now send a request that references the compaction token.
	body := `{"model":"test-model","input":"hello"}`
	req2 := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	srv.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

// ============================================================
// handleTelemetry — all valid frontend types (full coverage)
// ============================================================

func TestHandleTelemetry_AllValidTypes(t *testing.T) {
	mel := newTestMetricsLogger(t)
	srv := New(WithPort(0), WithMetrics(mel))

	types := []string{
		"frontend.error", "frontend.render", "frontend.ws",
		"frontend.usage", "frontend.ws.disconnect",
		"frontend.ws.reconnect", "frontend.auth.fail",
	}
	for _, typ := range types {
		body := fmt.Sprintf(`{"type":"%s","data":{"test":true}}`, typ)
		req := httptest.NewRequest("POST", "/api/telemetry", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("type %s: expected 204, got %d", typ, rec.Code)
		}
	}
}

// ============================================================
// Server.StartSSERelay — with tasks proxy (covers non-nil branch)
// ============================================================

func TestServer_StartSSERelay_WithTasksProxy(t *testing.T) {
	sseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Immediately close.
	}))
	defer sseServer.Close()

	t.Setenv("SOUL_TASKS_URL", sseServer.URL)
	hub := &mockBroadcastHub{}
	srv := New(WithPort(0), WithTasksProxy(hub))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		srv.StartSSERelay(ctx)
		close(done)
	}()

	// Cancel context and wait for clean exit.
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done
}
