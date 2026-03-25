package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nhooyr.io/websocket"

	"github.com/rishav1305/soul-v2/internal/chat/metrics"
	"github.com/rishav1305/soul-v2/internal/chat/session"
	"github.com/rishav1305/soul-v2/internal/chat/stream"
	"github.com/rishav1305/soul-v2/internal/chat/ws"
)

// ---------------------------------------------------------------------------
// Mock servers for error scenarios
// ---------------------------------------------------------------------------

// mockClaudeErrorServer returns a test server that always responds with the
// given HTTP status code and optional body. It can also set Retry-After header.
func mockClaudeErrorServer(t *testing.T, statusCode int, retryAfter string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"type":    "error",
			"message": fmt.Sprintf("simulated %d error", statusCode),
		})
	}))
}

// mockClaudeMidStreamErrorServer returns a test server that sends some tokens
// then sends an SSE error event.
func mockClaudeMidStreamErrorServer(t *testing.T, tokens []string, errMessage string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req stream.Request
		json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher := w.(http.Flusher)

		// message_start
		fmt.Fprintf(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_err\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"%s\",\"content\":[],\"usage\":{\"input_tokens\":5,\"output_tokens\":0}}}\n\n", req.Model)
		flusher.Flush()

		// content_block_start
		fmt.Fprintf(w, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
		flusher.Flush()

		// Send some tokens
		for _, token := range tokens {
			fmt.Fprintf(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":%q}}\n\n", token)
			flusher.Flush()
		}

		// Send error event instead of message_stop
		fmt.Fprintf(w, "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"api_error\",\"message\":%q}}\n\n", errMessage)
		flusher.Flush()
	}))
}

// mockClaudeDropConnectionServer returns a test server that sends some tokens
// then closes the connection without sending message_stop.
func mockClaudeDropConnectionServer(t *testing.T, tokens []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req stream.Request
		json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher := w.(http.Flusher)

		// message_start
		fmt.Fprintf(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_drop\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"%s\",\"content\":[],\"usage\":{\"input_tokens\":5,\"output_tokens\":0}}}\n\n", req.Model)
		flusher.Flush()

		// content_block_start
		fmt.Fprintf(w, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
		flusher.Flush()

		// Send some tokens
		for _, token := range tokens {
			fmt.Fprintf(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":%q}}\n\n", token)
			flusher.Flush()
		}

		// Close connection abruptly — no content_block_stop, no message_delta, no message_stop.
		// The httptest.Server will close the response writer when the handler returns.
	}))
}

// ---------------------------------------------------------------------------
// Setup helper with metrics
// ---------------------------------------------------------------------------

// setupErrorEnv creates a test environment with a custom mock server and
// metrics logging to a temp directory. Returns hub, store, metrics dir, and cancel.
func setupErrorEnv(t *testing.T, mockSrv *httptest.Server) (*ws.Hub, *session.Store, string, context.CancelFunc) {
	t.Helper()

	t.Cleanup(mockSrv.Close)

	// Stream client pointing at mock server.
	sc := stream.NewClient(
		&staticTokenSource{token: "test-token"},
		stream.WithBaseURL(mockSrv.URL),
	)

	// Session store.
	dbPath := filepath.Join(t.TempDir(), "error-integration.db")
	store, err := session.Open(dbPath)
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	// Metrics logger with temp dir.
	metricsDir := filepath.Join(t.TempDir(), "metrics")
	mel, err := metrics.NewEventLogger(metricsDir, "")
	if err != nil {
		t.Fatalf("create metrics logger: %v", err)
	}
	t.Cleanup(func() { mel.Close() })

	// Hub + handler with stream client and metrics.
	hub := ws.NewHub(ws.WithSessionStore(store))
	handler := ws.NewMessageHandler(hub, store, mel, ws.WithStreamClient(sc))
	hub.SetHandler(handler)

	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	return hub, store, metricsDir, cancel
}

// collectUntilError reads WebSocket messages until a chat.error is received.
// Returns all token messages and the error message.
func collectUntilError(t *testing.T, ctx context.Context, conn *websocket.Conn) (tokenMsgs []map[string]interface{}, errData map[string]interface{}) {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case <-timeout:
			t.Fatal("timed out waiting for chat.error")
		default:
		}

		msg := readJSON(t, ctx, conn)
		msgType := msg["type"].(string)

		switch msgType {
		case "chat.token":
			tokenMsgs = append(tokenMsgs, msg)
		case "chat.error":
			data := msg["data"].(map[string]interface{})
			return tokenMsgs, data
		case "chat.done":
			t.Fatal("received unexpected chat.done — expected chat.error")
		case "session.updated":
			// async hub broadcast (auto-title, status transitions) — discard
		}
	}
}

// ---------------------------------------------------------------------------
// Test 1: 401 Unauthorized → auth error message + api.error metric
// ---------------------------------------------------------------------------

func TestStreamError_401_AuthFailure(t *testing.T) {
	mockSrv := mockClaudeErrorServer(t, 401, "")
	hub, store, metricsDir, cancel := setupErrorEnv(t, mockSrv)
	defer cancel()

	sess, err := store.CreateSession("Auth Error Test")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.Background()
	srv := wsStreamServer(t, hub)
	conn := dial(t, ctx, srv)
	defer conn.Close(websocket.StatusNormalClosure, "")
	drainInitial(t, ctx, conn)

	// Send chat.send — should fail with 401.
	sendJSON(t, ctx, conn, `{"type":"chat.send","sessionId":"`+sess.ID+`","content":"test auth error"}`)

	_, errData := collectUntilError(t, ctx, conn)

	// Verify client receives auth-specific error.
	errMsg := errData["error"].(string)
	if !strings.Contains(errMsg, "authentication failed") {
		t.Errorf("expected auth error message, got: %s", errMsg)
	}

	// Verify api.error metric was logged.
	events, err := metrics.ReadEventsFiltered(metricsDir, "api.error")
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least one api.error metric event")
	}

	ev := events[0]
	if ev.Data["error_type"] != "auth" {
		t.Errorf("error_type = %v, want auth", ev.Data["error_type"])
	}
	// JSON numbers are float64 when unmarshaled.
	if statusCode, ok := ev.Data["status_code"].(float64); !ok || int(statusCode) != 401 {
		t.Errorf("status_code = %v, want 401", ev.Data["status_code"])
	}
}

// ---------------------------------------------------------------------------
// Test 2: 429 Rate Limit → rate limit error message + api.error metric
// ---------------------------------------------------------------------------

func TestStreamError_429_RateLimit(t *testing.T) {
	mockSrv := mockClaudeErrorServer(t, 429, "30")
	hub, store, metricsDir, cancel := setupErrorEnv(t, mockSrv)
	defer cancel()

	sess, err := store.CreateSession("Rate Limit Test")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.Background()
	srv := wsStreamServer(t, hub)
	conn := dial(t, ctx, srv)
	defer conn.Close(websocket.StatusNormalClosure, "")
	drainInitial(t, ctx, conn)

	sendJSON(t, ctx, conn, `{"type":"chat.send","sessionId":"`+sess.ID+`","content":"test rate limit"}`)

	_, errData := collectUntilError(t, ctx, conn)

	// Verify client receives rate-limit-specific error.
	errMsg := errData["error"].(string)
	if !strings.Contains(errMsg, "rate limited") {
		t.Errorf("expected rate limit error message, got: %s", errMsg)
	}

	// Verify api.error metric was logged with retry_after_ms.
	events, err := metrics.ReadEventsFiltered(metricsDir, "api.error")
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least one api.error metric event")
	}

	ev := events[0]
	if ev.Data["error_type"] != "rate_limit" {
		t.Errorf("error_type = %v, want rate_limit", ev.Data["error_type"])
	}
	if statusCode, ok := ev.Data["status_code"].(float64); !ok || int(statusCode) != 429 {
		t.Errorf("status_code = %v, want 429", ev.Data["status_code"])
	}
	if retryAfterMs, ok := ev.Data["retry_after_ms"].(float64); !ok || retryAfterMs != 30000 {
		t.Errorf("retry_after_ms = %v, want 30000", ev.Data["retry_after_ms"])
	}
}

// ---------------------------------------------------------------------------
// Test 3: 500 Server Error → server error message + api.error metric
// ---------------------------------------------------------------------------

func TestStreamError_500_ServerError(t *testing.T) {
	mockSrv := mockClaudeErrorServer(t, 500, "")
	hub, store, metricsDir, cancel := setupErrorEnv(t, mockSrv)
	defer cancel()

	sess, err := store.CreateSession("Server Error Test")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.Background()
	srv := wsStreamServer(t, hub)
	conn := dial(t, ctx, srv)
	defer conn.Close(websocket.StatusNormalClosure, "")
	drainInitial(t, ctx, conn)

	sendJSON(t, ctx, conn, `{"type":"chat.send","sessionId":"`+sess.ID+`","content":"test server error"}`)

	_, errData := collectUntilError(t, ctx, conn)

	// Verify client receives server error message.
	errMsg := errData["error"].(string)
	if !strings.Contains(errMsg, "Claude API temporarily unavailable") {
		t.Errorf("expected server error message, got: %s", errMsg)
	}

	// Verify api.error metric.
	events, err := metrics.ReadEventsFiltered(metricsDir, "api.error")
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least one api.error metric event")
	}

	ev := events[0]
	if ev.Data["error_type"] != "server" {
		t.Errorf("error_type = %v, want server", ev.Data["error_type"])
	}
	if statusCode, ok := ev.Data["status_code"].(float64); !ok || int(statusCode) != 500 {
		t.Errorf("status_code = %v, want 500", ev.Data["status_code"])
	}
}

// ---------------------------------------------------------------------------
// Test 4: Mid-stream SSE error event → partial tokens then chat.error
// ---------------------------------------------------------------------------

func TestStreamError_MidStreamSSEError(t *testing.T) {
	tokens := []string{"Hello", " ", "world"}
	mockSrv := mockClaudeMidStreamErrorServer(t, tokens, "overloaded")
	hub, store, metricsDir, cancel := setupErrorEnv(t, mockSrv)
	defer cancel()

	sess, err := store.CreateSession("Mid-Stream Error Test")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.Background()
	srv := wsStreamServer(t, hub)
	conn := dial(t, ctx, srv)
	defer conn.Close(websocket.StatusNormalClosure, "")
	drainInitial(t, ctx, conn)

	sendJSON(t, ctx, conn, `{"type":"chat.send","sessionId":"`+sess.ID+`","content":"test mid-stream error"}`)

	tokenMsgs, errData := collectUntilError(t, ctx, conn)

	// Verify we received partial tokens before the error.
	if len(tokenMsgs) != len(tokens) {
		t.Errorf("expected %d token events before error, got %d", len(tokens), len(tokenMsgs))
	}

	// Verify token content.
	for i, msg := range tokenMsgs {
		data := msg["data"].(map[string]interface{})
		if data["token"] != tokens[i] {
			t.Errorf("token[%d]: expected %q, got %q", i, tokens[i], data["token"])
		}
	}

	// Verify the error message (sanitized, not raw API message).
	errMsg := errData["error"].(string)
	if !strings.Contains(errMsg, "stream interrupted") {
		t.Errorf("expected error containing 'stream interrupted', got: %s", errMsg)
	}

	// Verify api.error metric for mid-stream error.
	events, err := metrics.ReadEventsFiltered(metricsDir, "api.error")
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least one api.error metric event")
	}

	ev := events[0]
	if ev.Data["error_type"] != "stream" {
		t.Errorf("error_type = %v, want stream", ev.Data["error_type"])
	}
}

// ---------------------------------------------------------------------------
// Test 5: Dropped connection (no message_stop) → incomplete stream error
// ---------------------------------------------------------------------------

func TestStreamError_DroppedConnection_IncompleteStream(t *testing.T) {
	tokens := []string{"Partial", " response"}
	mockSrv := mockClaudeDropConnectionServer(t, tokens)
	hub, store, metricsDir, cancel := setupErrorEnv(t, mockSrv)
	defer cancel()

	sess, err := store.CreateSession("Dropped Connection Test")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.Background()
	srv := wsStreamServer(t, hub)
	conn := dial(t, ctx, srv)
	defer conn.Close(websocket.StatusNormalClosure, "")
	drainInitial(t, ctx, conn)

	sendJSON(t, ctx, conn, `{"type":"chat.send","sessionId":"`+sess.ID+`","content":"test dropped connection"}`)

	// When a stream drops mid-way, the handler persists partial content and
	// sends chat.done (graceful degradation, not chat.error). Collect tokens +
	// session.updated broadcasts until chat.done.
	var tokenMsgs []map[string]interface{}
	var chatDone map[string]interface{}
	timeout := time.After(10 * time.Second)
	for {
		select {
		case <-timeout:
			t.Fatal("timed out waiting for chat.done after dropped stream")
		default:
		}
		msg := readJSON(t, ctx, conn)
		switch msg["type"] {
		case "chat.token":
			tokenMsgs = append(tokenMsgs, msg)
		case "chat.done":
			chatDone = msg
		case "session.updated":
			// async broadcast — discard
		default:
			t.Fatalf("unexpected message type: %v", msg["type"])
		}
		if chatDone != nil {
			break
		}
	}

	// Should have received some tokens before stream was dropped.
	if len(tokenMsgs) < 1 {
		t.Error("expected at least 1 token event before incomplete stream completes")
	}

	// Verify the stored message contains the incomplete marker.
	msgs, err := store.GetMessages(sess.ID)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	var assistantMsg string
	for _, m := range msgs {
		if m.Role == "assistant" {
			assistantMsg = m.Content
			break
		}
	}
	if !strings.Contains(assistantMsg, "[incomplete") {
		t.Errorf("expected partial message to contain [incomplete], got: %q", assistantMsg)
	}

	// Verify api.error metric for incomplete stream was still logged.
	events, err := metrics.ReadEventsFiltered(metricsDir, "api.error")
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least one api.error metric event for incomplete stream")
	}

	ev := events[0]
	if ev.Data["error_type"] != "incomplete_stream" {
		t.Errorf("error_type = %v, want incomplete_stream", ev.Data["error_type"])
	}
	if ev.Data["session_id"] != sess.ID {
		t.Errorf("session_id = %v, want %s", ev.Data["session_id"], sess.ID)
	}
}

// ---------------------------------------------------------------------------
// Test 6: Metrics file verification — api.error events have correct structure
// ---------------------------------------------------------------------------

func TestStreamError_MetricsFileVerification(t *testing.T) {
	// Use a 502 error to test server error classification.
	mockSrv := mockClaudeErrorServer(t, 502, "")
	hub, store, metricsDir, cancel := setupErrorEnv(t, mockSrv)
	defer cancel()

	sess, err := store.CreateSession("Metrics Verification")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.Background()
	srv := wsStreamServer(t, hub)
	conn := dial(t, ctx, srv)
	defer conn.Close(websocket.StatusNormalClosure, "")
	drainInitial(t, ctx, conn)

	sendJSON(t, ctx, conn, `{"type":"chat.send","sessionId":"`+sess.ID+`","content":"metrics test"}`)

	// Wait for the error.
	collectUntilError(t, ctx, conn)

	// Read all events and verify structure.
	allEvents, err := metrics.ReadEvents(metricsDir)
	if err != nil {
		t.Fatalf("read all metrics: %v", err)
	}

	// Find the api.error event.
	var found bool
	for _, ev := range allEvents {
		if ev.EventType != metrics.EventAPIError {
			continue
		}
		found = true

		// Verify required fields.
		requiredFields := []string{"session_id", "error_type", "status_code", "error_message"}
		for _, field := range requiredFields {
			if _, ok := ev.Data[field]; !ok {
				t.Errorf("api.error event missing required field %q", field)
			}
		}

		// Verify session_id matches.
		if ev.Data["session_id"] != sess.ID {
			t.Errorf("session_id = %v, want %s", ev.Data["session_id"], sess.ID)
		}

		// Verify error_type is "server" for 502.
		if ev.Data["error_type"] != "server" {
			t.Errorf("error_type = %v, want server", ev.Data["error_type"])
		}

		// Verify status_code is 502.
		if statusCode, ok := ev.Data["status_code"].(float64); !ok || int(statusCode) != 502 {
			t.Errorf("status_code = %v, want 502", ev.Data["status_code"])
		}

		// Verify error_message is non-empty.
		if msg, ok := ev.Data["error_message"].(string); !ok || msg == "" {
			t.Error("error_message should be non-empty")
		}

		// Verify timestamp is valid (non-zero).
		if ev.Timestamp.IsZero() {
			t.Error("event timestamp should not be zero")
		}
	}

	if !found {
		t.Fatal("no api.error event found in metrics file")
	}
}
