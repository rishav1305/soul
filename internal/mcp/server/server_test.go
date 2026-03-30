package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rishav1305/soul/internal/mcp/auth"
	"github.com/rishav1305/soul/internal/mcp/tools"
)

func testServerWithRegistry(t *testing.T) *Server {
	t.Helper()
	// nil dispatcher = list-only mode.
	reg := tools.NewRegistry(nil)
	return New(WithRegistry(reg), WithSecret("test-secret"))
}

func postMCP(t *testing.T, s *Server, req interface{}) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, r)
	return w
}

// --- Health ---

func TestHandleHealth(t *testing.T) {
	s := testServerWithRegistry(t)
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("status = %v, want ok", body["status"])
	}
	// Registry populated from context package — should have many tools.
	toolCount, _ := body["tools"].(float64)
	if toolCount == 0 {
		t.Error("expected non-zero tool count with registry")
	}
}

func TestHandleHealth_NoRegistry(t *testing.T) {
	s := New()
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["tools"] != float64(0) {
		t.Errorf("tools = %v, want 0", body["tools"])
	}
}

// --- MCP GET (405) ---

func TestHandleMCPGet(t *testing.T) {
	s := testServerWithRegistry(t)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
	if got := w.Header().Get("Allow"); got != "POST" {
		t.Errorf("Allow = %q, want POST", got)
	}
}

// --- MCP POST: initialize ---

func TestHandleMCP_Initialize(t *testing.T) {
	s := testServerWithRegistry(t)
	w := postMCP(t, s, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != nil {
		t.Errorf("unexpected error: %v", resp["error"])
	}
	result, _ := resp["result"].(map[string]interface{})
	if result["protocolVersion"] != "2025-03-26" {
		t.Errorf("protocolVersion = %v, want 2025-03-26", result["protocolVersion"])
	}
}

// --- MCP POST: ping ---

func TestHandleMCP_Ping(t *testing.T) {
	s := testServerWithRegistry(t)
	w := postMCP(t, s, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "ping",
	})

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// --- MCP POST: tools/list ---

func TestHandleMCP_ToolsList(t *testing.T) {
	s := testServerWithRegistry(t)
	w := postMCP(t, s, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/list",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result object, got %T", resp["result"])
	}
	toolsList, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("expected tools array, got %T", result["tools"])
	}
	if len(toolsList) == 0 {
		t.Error("expected non-empty tools list")
	}
}

func TestHandleMCP_ToolsList_NoRegistry(t *testing.T) {
	s := New()
	w := postMCP(t, s, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/list",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	result := resp["result"].(map[string]interface{})
	toolsList := result["tools"].([]interface{})
	if len(toolsList) != 0 {
		t.Errorf("tools count = %d, want 0", len(toolsList))
	}
}

// --- MCP POST: tools/call ---

func TestHandleMCP_ToolsCall_NoRegistry(t *testing.T) {
	s := New()
	w := postMCP(t, s, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      6,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "echo",
			"arguments": map[string]interface{}{},
		},
	})

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] == nil {
		t.Error("expected error when registry is nil")
	}
}

func TestHandleMCP_ToolsCall_UnknownTool(t *testing.T) {
	s := testServerWithRegistry(t)
	w := postMCP(t, s, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "nonexistent_tool_xyz",
			"arguments": map[string]interface{}{},
		},
	})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestHandleMCP_ToolsCall_NilDispatcher(t *testing.T) {
	// Registry exists but dispatcher is nil — Call should fail.
	s := testServerWithRegistry(t)

	// Get a real tool name from the list.
	toolsList := s.registry.List()
	if len(toolsList) == 0 {
		t.Skip("no tools in registry")
	}
	toolName := toolsList[0].Name

	w := postMCP(t, s, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      7,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      toolName,
			"arguments": map[string]interface{}{},
		},
	})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	// Should return a result with isError=true (dispatcher nil error).
	result, _ := resp["result"].(map[string]interface{})
	if result != nil {
		if isErr, ok := result["isError"].(bool); ok && isErr {
			// Expected — tool call returns error result when dispatcher is nil.
			return
		}
	}
	if resp["error"] != nil {
		// Also acceptable — JSON-RPC error.
		return
	}
	t.Error("expected error response for nil dispatcher tool call")
}

// --- MCP POST: unknown method ---

func TestHandleMCP_UnknownMethod(t *testing.T) {
	s := testServerWithRegistry(t)
	w := postMCP(t, s, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      8,
		"method":  "nonexistent/method",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] == nil {
		t.Error("expected error for unknown method")
	}
}

// --- MCP POST: notification (no ID → 202) ---

func TestHandleMCP_Notification(t *testing.T) {
	s := testServerWithRegistry(t)
	w := postMCP(t, s, map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	})

	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", w.Code, http.StatusAccepted)
	}
}

// --- MCP POST: invalid JSON ---

func TestHandleMCP_InvalidJSON(t *testing.T) {
	s := testServerWithRegistry(t)
	r := httptest.NewRequest("POST", "/", bytes.NewReader([]byte("not json")))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- MCP POST: invalid params for tools/call ---

func TestHandleMCP_ToolsCall_InvalidParams(t *testing.T) {
	s := testServerWithRegistry(t)
	w := postMCP(t, s, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      9,
		"method":  "tools/call",
		"params":  "not-an-object",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] == nil {
		t.Error("expected error for invalid params")
	}
}

// --- Middleware ---

func TestRecoveryMiddleware(t *testing.T) {
	panicker := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
	handler := recoveryMiddleware(panicker)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestBodyLimitMiddleware(t *testing.T) {
	echoer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		_, err := buf.ReadFrom(r.Body)
		if err != nil {
			http.Error(w, "too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.Write(buf.Bytes())
	})
	handler := bodyLimitMiddleware(16)(echoer)

	// Small body OK.
	req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte("small")))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("small body: status = %d, want %d", w.Code, http.StatusOK)
	}

	// Large body rejected.
	big := bytes.Repeat([]byte("x"), 100)
	req = httptest.NewRequest("POST", "/", bytes.NewReader(big))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code == http.StatusOK && w.Body.Len() == 100 {
		t.Error("expected body limit to reject large POST")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	counter := 0
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter++
		w.WriteHeader(http.StatusOK)
	})
	handler := rateLimitMiddleware(3)(inner)

	// First 3 should succeed.
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("POST", "/mcp", nil)
		req.RemoteAddr = "1.2.3.4:5678"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want %d", i, w.Code, http.StatusOK)
		}
	}

	// 4th should be rate-limited.
	req := httptest.NewRequest("POST", "/mcp", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("4th request: status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	// Health endpoint is exempt.
	req = httptest.NewRequest("GET", "/health", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("health exemption: status = %d, want %d", w.Code, http.StatusOK)
	}
}

// --- Helpers ---

func TestClientIP_Direct(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:8080"
	if got := clientIP(r); got != "10.0.0.1" {
		t.Errorf("clientIP = %q, want 10.0.0.1", got)
	}
}

func TestClientIP_XFF_Loopback(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:8080"
	r.Header.Set("X-Forwarded-For", "203.0.113.1, 10.0.0.1")
	if got := clientIP(r); got != "203.0.113.1" {
		t.Errorf("clientIP = %q, want 203.0.113.1", got)
	}
}

func TestClientIP_XFF_NonLoopback(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.5:8080"
	r.Header.Set("X-Forwarded-For", "203.0.113.1")
	if got := clientIP(r); got != "10.0.0.5" {
		t.Errorf("clientIP = %q, want 10.0.0.5", got)
	}
}

func TestClientIP_XFF_SingleIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:8080"
	r.Header.Set("X-Forwarded-For", "203.0.113.1")
	if got := clientIP(r); got != "203.0.113.1" {
		t.Errorf("clientIP = %q, want 203.0.113.1", got)
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, map[string]string{"key": "val"})
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestWithOptions(t *testing.T) {
	s := New(WithHost("0.0.0.0"), WithPort(9999), WithSecret("s3cret"))
	if s.host != "0.0.0.0" {
		t.Errorf("host = %q, want 0.0.0.0", s.host)
	}
	if s.port != 9999 {
		t.Errorf("port = %d, want 9999", s.port)
	}
	if s.secret != "s3cret" {
		t.Errorf("secret = %q, want s3cret", s.secret)
	}
}

func TestWithOAuth(t *testing.T) {
	h := auth.NewOAuthHandler("test-secret", "admin")
	s := New(WithOAuth(h))
	if s.oauth != h {
		t.Error("expected oauth handler to be set")
	}
}

func TestWithBaseURL_WithOAuth(t *testing.T) {
	h := auth.NewOAuthHandler("test-secret", "admin")
	s := New(WithOAuth(h), WithBaseURL("http://example.com"))
	_ = s // Just exercising the option application path
}

func TestWithBaseURL_WithoutOAuth(t *testing.T) {
	// WithBaseURL when oauth is nil should not panic.
	s := New(WithBaseURL("http://example.com"))
	_ = s
}

func TestStartShutdown(t *testing.T) {
	s := New(WithHost("127.0.0.1"), WithPort(0))
	errCh := make(chan error, 1)
	go func() { errCh <- s.Start() }()
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown error: %v", err)
	}
}

func TestRateLimitMiddleware_XForwardedFor(t *testing.T) {
	counter := 0
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter++
		w.WriteHeader(http.StatusOK)
	})
	handler := rateLimitMiddleware(2)(inner)

	// Two requests from same XFF IP should succeed.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/mcp", nil)
		req.RemoteAddr = "127.0.0.1:5678"
		req.Header.Set("X-Forwarded-For", "10.0.0.99")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want 200", i, w.Code)
		}
	}

	// Third should be rate-limited.
	req := httptest.NewRequest("POST", "/mcp", nil)
	req.RemoteAddr = "127.0.0.1:5678"
	req.Header.Set("X-Forwarded-For", "10.0.0.99")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("3rd request: status = %d, want 429", w.Code)
	}
}

func TestRateLimitMiddleware_DifferentClients(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rateLimitMiddleware(1)(inner)

	// First client.
	req := httptest.NewRequest("POST", "/mcp", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("client 1: status = %d, want 200", w.Code)
	}

	// Second client should also succeed (different IP).
	req2 := httptest.NewRequest("POST", "/mcp", nil)
	req2.RemoteAddr = "10.0.0.2:1234"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("client 2: status = %d, want 200", w2.Code)
	}
}

func TestHandleMCP_EmptyBody(t *testing.T) {
	s := testServerWithRegistry(t)
	r := httptest.NewRequest("POST", "/", bytes.NewReader(nil))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, r)

	// Empty body should return parse error.
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
