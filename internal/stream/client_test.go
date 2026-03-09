package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockTokenSource implements TokenSource for testing.
type mockTokenSource struct {
	token string
	err   error
}

func (m *mockTokenSource) Token() (string, error) {
	return m.token, m.err
}

func TestNewClient_Defaults(t *testing.T) {
	ts := &mockTokenSource{token: "test-token"}
	c := NewClient(ts)

	if c.model != DefaultModel {
		t.Fatalf("expected default model %q, got %q", DefaultModel, c.model)
	}
	if c.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, c.baseURL)
	}
	if c.apiVersion != DefaultAPIVersion {
		t.Fatalf("expected default API version %q, got %q", DefaultAPIVersion, c.apiVersion)
	}
	if c.httpClient == nil {
		t.Fatal("expected non-nil HTTP client")
	}
	if c.httpClient.Timeout != DefaultTimeout {
		t.Fatalf("expected timeout %v, got %v", DefaultTimeout, c.httpClient.Timeout)
	}
	if c.auth == nil {
		t.Fatal("expected non-nil auth")
	}
}

func TestNewClient_WithModel(t *testing.T) {
	ts := &mockTokenSource{token: "test-token"}
	c := NewClient(ts, WithModel("claude-opus-4-20250514"))

	if c.model != "claude-opus-4-20250514" {
		t.Fatalf("expected model claude-opus-4-20250514, got %q", c.model)
	}
}

func TestNewClient_WithBaseURL(t *testing.T) {
	ts := &mockTokenSource{token: "test-token"}
	c := NewClient(ts, WithBaseURL("http://localhost:8080"))

	if c.baseURL != "http://localhost:8080" {
		t.Fatalf("expected base URL http://localhost:8080, got %q", c.baseURL)
	}
}

func TestNewClient_WithHTTPClient(t *testing.T) {
	ts := &mockTokenSource{token: "test-token"}
	custom := &http.Client{Timeout: 60 * time.Second}
	c := NewClient(ts, WithHTTPClient(custom))

	if c.httpClient != custom {
		t.Fatal("expected custom HTTP client")
	}
	if c.httpClient.Timeout != 60*time.Second {
		t.Fatalf("expected 60s timeout, got %v", c.httpClient.Timeout)
	}
}

func TestNewClient_WithAPIVersion(t *testing.T) {
	ts := &mockTokenSource{token: "test-token"}
	c := NewClient(ts, WithAPIVersion("2024-01-01"))

	if c.apiVersion != "2024-01-01" {
		t.Fatalf("expected API version 2024-01-01, got %q", c.apiVersion)
	}
}

func TestNewClient_MultipleOptions(t *testing.T) {
	ts := &mockTokenSource{token: "test-token"}
	c := NewClient(ts,
		WithModel("claude-opus-4-20250514"),
		WithBaseURL("http://localhost:9090"),
		WithAPIVersion("2024-06-01"),
	)

	if c.model != "claude-opus-4-20250514" {
		t.Fatalf("expected model claude-opus-4-20250514, got %q", c.model)
	}
	if c.baseURL != "http://localhost:9090" {
		t.Fatalf("expected base URL http://localhost:9090, got %q", c.baseURL)
	}
	if c.apiVersion != "2024-06-01" {
		t.Fatalf("expected API version 2024-06-01, got %q", c.apiVersion)
	}
}

func TestTokenSourceInterface(t *testing.T) {
	// Verify that mockTokenSource satisfies the TokenSource interface.
	var _ TokenSource = (*mockTokenSource)(nil)
}

// ---------------------------------------------------------------------------
// Mock Claude API server helpers
// ---------------------------------------------------------------------------

// mockClaudeServer returns an httptest.Server that simulates Claude API
// streaming responses. It verifies headers and returns SSE events.
func mockClaudeServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method and path.
		if r.Method != "POST" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/v1/messages" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		// Verify required headers.
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "bad content type", http.StatusBadRequest)
			return
		}
		if r.Header.Get("anthropic-version") == "" {
			http.Error(w, "missing api version", http.StatusBadRequest)
			return
		}

		// Decode request to check if streaming is requested.
		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}

		if !req.Stream {
			// Non-streaming: return a complete response.
			w.Header().Set("Content-Type", "application/json")
			resp := Response{
				ID:         "msg_test123",
				Type:       "message",
				Role:       "assistant",
				Model:      req.Model,
				StopReason: "end_turn",
				Content: []ContentBlock{
					{Type: "text", Text: "Hello world!"},
				},
				Usage: &Usage{InputTokens: 10, OutputTokens: 5},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Streaming response.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_test123\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"%s\",\"content\":[],\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\n", req.Model)
		flusher.Flush()

		fmt.Fprintf(w, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
		flusher.Flush()

		for _, token := range []string{"Hello", " ", "world", "!"} {
			fmt.Fprintf(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":%q}}\n\n", token)
			flusher.Flush()
		}

		fmt.Fprintf(w, "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n")
		flusher.Flush()

		fmt.Fprintf(w, "event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":10}}\n\n")
		flusher.Flush()

		fmt.Fprintf(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
		flusher.Flush()
	}))
}

// validRequest returns a minimal valid stream.Request for testing.
func validRequest() *Request {
	return &Request{
		MaxTokens: 1024,
		Messages: []Message{
			{
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: "Hello"},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Stream() tests
// ---------------------------------------------------------------------------

func TestStream_ReturnsEventsInOrder(t *testing.T) {
	srv := mockClaudeServer(t)
	defer srv.Close()

	ts := &mockTokenSource{token: "test-token"}
	c := NewClient(ts, WithBaseURL(srv.URL))

	ctx := context.Background()
	ch, err := c.Stream(ctx, validRequest())
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	// Collect all events.
	var events []SSEEvent
	for evt := range ch {
		events = append(events, evt)
	}

	if len(events) < 4 {
		t.Fatalf("expected at least 4 events, got %d", len(events))
	}

	// Verify event sequence.
	expectedTypes := []string{
		"message_start",
		"content_block_start",
		"content_block_delta", // Hello
		"content_block_delta", // " "
		"content_block_delta", // world
		"content_block_delta", // !
		"content_block_stop",
		"message_delta",
		"message_stop",
	}

	if len(events) != len(expectedTypes) {
		t.Fatalf("expected %d events, got %d", len(expectedTypes), len(events))
	}

	for i, evt := range events {
		if evt.Type != expectedTypes[i] {
			t.Errorf("event[%d]: expected type %q, got %q", i, expectedTypes[i], evt.Type)
		}
	}

	// Verify message_start has message ID.
	if events[0].Message == nil || events[0].Message.ID != "msg_test123" {
		t.Errorf("message_start should have message ID msg_test123")
	}

	// Verify content deltas carry text.
	texts := []string{"Hello", " ", "world", "!"}
	for i, txt := range texts {
		evt := events[i+2] // skip message_start, content_block_start
		if evt.Delta == nil || evt.Delta.Text != txt {
			t.Errorf("delta[%d]: expected text %q, got %v", i, txt, evt.Delta)
		}
	}
}

func TestStream_ValidatesRequest(t *testing.T) {
	ts := &mockTokenSource{token: "test-token"}
	c := NewClient(ts)

	// Empty messages should fail validation.
	req := &Request{MaxTokens: 100}
	_, err := c.Stream(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid request")
	}
	if !containsStr(err.Error(), "invalid request") {
		t.Errorf("expected 'invalid request' in error, got %q", err.Error())
	}
}

func TestStream_SetsCorrectHeaders(t *testing.T) {
	var capturedHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		// Return a minimal streaming response.
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		fmt.Fprintf(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_h\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"test\",\"content\":[]}}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	ts := &mockTokenSource{token: "my-secret-token"}
	c := NewClient(ts, WithBaseURL(srv.URL), WithAPIVersion("2023-06-01"))

	ch, err := c.Stream(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	// Drain events.
	for range ch {
	}

	if capturedHeaders.Get("Authorization") != "Bearer my-secret-token" {
		t.Errorf("Authorization header: got %q, want %q", capturedHeaders.Get("Authorization"), "Bearer my-secret-token")
	}
	if capturedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type header: got %q", capturedHeaders.Get("Content-Type"))
	}
	if capturedHeaders.Get("anthropic-version") != "2023-06-01" {
		t.Errorf("anthropic-version header: got %q", capturedHeaders.Get("anthropic-version"))
	}
	if capturedHeaders.Get("anthropic-beta") != "prompt-caching-2024-07-31" {
		t.Errorf("anthropic-beta header: got %q", capturedHeaders.Get("anthropic-beta"))
	}
}

func TestStream_ClosesChannelOnMessageStop(t *testing.T) {
	srv := mockClaudeServer(t)
	defer srv.Close()

	ts := &mockTokenSource{token: "test-token"}
	c := NewClient(ts, WithBaseURL(srv.URL))

	ch, err := c.Stream(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	// Channel should be closed after all events are consumed.
	count := 0
	for range ch {
		count++
	}
	if count == 0 {
		t.Fatal("expected at least one event before channel close")
	}

	// Reading from closed channel should give zero value immediately.
	evt, ok := <-ch
	if ok {
		t.Errorf("expected channel to be closed, got event: %v", evt)
	}
}

func TestStream_HandlesContextCancellation(t *testing.T) {
	// Create a slow server that delays between events.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		fmt.Fprintf(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_slow\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"test\",\"content\":[]}}\n\n")
		flusher.Flush()

		// Delay long enough for context to be cancelled.
		time.Sleep(5 * time.Second)

		fmt.Fprintf(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	ts := &mockTokenSource{token: "test-token"}
	c := NewClient(ts, WithBaseURL(srv.URL))

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	ch, err := c.Stream(ctx, validRequest())
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	// Consume events — channel should close when context is cancelled.
	var events []SSEEvent
	for evt := range ch {
		events = append(events, evt)
	}

	// Should have gotten message_start but not message_stop.
	if len(events) == 0 {
		t.Fatal("expected at least one event before cancellation")
	}

	// Verify we got message_start.
	if events[0].Type != "message_start" {
		t.Errorf("first event should be message_start, got %q", events[0].Type)
	}

	// Should NOT have gotten message_stop (cancelled before).
	for _, evt := range events {
		if evt.Type == "message_stop" {
			t.Error("should not have received message_stop after cancellation")
		}
	}
}

func TestStream_TokenSourceError(t *testing.T) {
	ts := &mockTokenSource{err: errors.New("token expired")}
	c := NewClient(ts)

	_, err := c.Stream(context.Background(), validRequest())
	if err == nil {
		t.Fatal("expected error when token source fails")
	}
	if !containsStr(err.Error(), "get token") {
		t.Errorf("expected 'get token' in error, got %q", err.Error())
	}
}

func TestStream_Handles401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"type":    "authentication_error",
			"message": "invalid token",
		})
	}))
	defer srv.Close()

	ts := &mockTokenSource{token: "bad-token"}
	c := NewClient(ts, WithBaseURL(srv.URL))

	_, err := c.Stream(context.Background(), validRequest())
	if err == nil {
		t.Fatal("expected error for 401")
	}

	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Errorf("expected AuthError, got %T: %v", err, err)
	}
}

func TestStream_Handles429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"type":    "rate_limit_error",
			"message": "too many requests",
		})
	}))
	defer srv.Close()

	ts := &mockTokenSource{token: "test-token"}
	c := NewClient(ts, WithBaseURL(srv.URL))

	_, err := c.Stream(context.Background(), validRequest())
	if err == nil {
		t.Fatal("expected error for 429")
	}

	var rlErr *RateLimitError
	if !errors.As(err, &rlErr) {
		t.Errorf("expected RateLimitError, got %T: %v", err, err)
	}
	if rlErr.RetryAfter != 30*time.Second {
		t.Errorf("expected RetryAfter 30s, got %v", rlErr.RetryAfter)
	}
}

func TestStream_Handles500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"type":    "server_error",
			"message": "internal error",
		})
	}))
	defer srv.Close()

	ts := &mockTokenSource{token: "test-token"}
	c := NewClient(ts, WithBaseURL(srv.URL))

	_, err := c.Stream(context.Background(), validRequest())
	if err == nil {
		t.Fatal("expected error for 500")
	}

	var srvErr *ServerError
	if !errors.As(err, &srvErr) {
		t.Errorf("expected ServerError, got %T: %v", err, err)
	}
	if srvErr.StatusCode != 500 {
		t.Errorf("expected status 500, got %d", srvErr.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Send() tests
// ---------------------------------------------------------------------------

func TestSend_ReturnsCompleteResponse(t *testing.T) {
	srv := mockClaudeServer(t)
	defer srv.Close()

	ts := &mockTokenSource{token: "test-token"}
	c := NewClient(ts, WithBaseURL(srv.URL))

	resp, err := c.Send(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}

	if resp.ID != "msg_test123" {
		t.Errorf("expected ID msg_test123, got %q", resp.ID)
	}
	if resp.Role != "assistant" {
		t.Errorf("expected role assistant, got %q", resp.Role)
	}
	if len(resp.Content) != 1 || resp.Content[0].Text != "Hello world!" {
		t.Errorf("unexpected content: %+v", resp.Content)
	}
	if resp.Usage == nil || resp.Usage.InputTokens != 10 || resp.Usage.OutputTokens != 5 {
		t.Errorf("unexpected usage: %+v", resp.Usage)
	}
}

func TestSend_ValidatesRequest(t *testing.T) {
	ts := &mockTokenSource{token: "test-token"}
	c := NewClient(ts)

	req := &Request{MaxTokens: 100}
	_, err := c.Send(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid request")
	}
	if !containsStr(err.Error(), "invalid request") {
		t.Errorf("expected 'invalid request' in error, got %q", err.Error())
	}
}

func TestSend_SetsStreamFalse(t *testing.T) {
	var streamValue bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		json.NewDecoder(r.Body).Decode(&req)
		streamValue = req.Stream

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			ID: "msg_ns", Type: "message", Role: "assistant",
			Content: []ContentBlock{{Type: "text", Text: "ok"}},
		})
	}))
	defer srv.Close()

	ts := &mockTokenSource{token: "test-token"}
	c := NewClient(ts, WithBaseURL(srv.URL))

	_, err := c.Send(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if streamValue {
		t.Error("Send() should set stream=false in the request")
	}
}

func TestSend_UsesDefaultModel(t *testing.T) {
	var sentModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		json.NewDecoder(r.Body).Decode(&req)
		sentModel = req.Model

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			ID: "msg_m", Type: "message", Role: "assistant",
			Content: []ContentBlock{{Type: "text", Text: "ok"}},
		})
	}))
	defer srv.Close()

	ts := &mockTokenSource{token: "test-token"}
	c := NewClient(ts, WithBaseURL(srv.URL), WithModel("claude-test-model"))

	req := validRequest()
	req.Model = "" // Should be filled with client default.
	_, err := c.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if sentModel != "claude-test-model" {
		t.Errorf("expected model claude-test-model, got %q", sentModel)
	}
}

// ---------------------------------------------------------------------------
// Error type tests
// ---------------------------------------------------------------------------

func TestErrorTypes_Implement_Error(t *testing.T) {
	// Verify all error types implement error interface.
	var _ error = &AuthError{Err: errors.New("test")}
	var _ error = &RateLimitError{Err: errors.New("test")}
	var _ error = &ServerError{Err: errors.New("test")}
	var _ error = &IncompleteStreamError{}
}

func TestAuthError_Unwrap(t *testing.T) {
	inner := errors.New("inner")
	err := &AuthError{Err: inner}
	if !errors.Is(err, inner) {
		t.Error("AuthError.Unwrap should return inner error")
	}
}

func TestRateLimitError_Unwrap(t *testing.T) {
	inner := errors.New("inner")
	err := &RateLimitError{Err: inner}
	if !errors.Is(err, inner) {
		t.Error("RateLimitError.Unwrap should return inner error")
	}
}

func TestServerError_Unwrap(t *testing.T) {
	inner := errors.New("inner")
	err := &ServerError{Err: inner}
	if !errors.Is(err, inner) {
		t.Error("ServerError.Unwrap should return inner error")
	}
}

// containsStr is a simple substring check helper.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && contains(s, substr)
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
