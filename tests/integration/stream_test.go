package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"nhooyr.io/websocket"

	"github.com/rishav1305/soul-v2/internal/chat/session"
	"github.com/rishav1305/soul-v2/internal/chat/stream"
	"github.com/rishav1305/soul-v2/internal/chat/ws"
)

// ---------------------------------------------------------------------------
// Mock Claude API server for integration tests
// ---------------------------------------------------------------------------

// mockClaudeStreamServer returns a test server that simulates Claude API
// streaming responses with configurable tokens.
func mockClaudeStreamServer(t *testing.T, tokens []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/messages" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		var req stream.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}

		if !req.Stream {
			// Non-streaming fallback.
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(stream.Response{
				ID: "msg_ns", Type: "message", Role: "assistant",
				Content: []stream.ContentBlock{{Type: "text", Text: "non-stream"}},
			})
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no flusher", http.StatusInternalServerError)
			return
		}

		// message_start
		fmt.Fprintf(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_int123\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"%s\",\"content\":[],\"usage\":{\"input_tokens\":5,\"output_tokens\":0}}}\n\n", req.Model)
		flusher.Flush()

		// content_block_start
		fmt.Fprintf(w, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
		flusher.Flush()

		// content_block_deltas
		for _, token := range tokens {
			fmt.Fprintf(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":%q}}\n\n", token)
			flusher.Flush()
		}

		// content_block_stop
		fmt.Fprintf(w, "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n")
		flusher.Flush()

		// message_delta
		fmt.Fprintf(w, "event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":%d}}\n\n", len(tokens))
		flusher.Flush()

		// message_stop
		fmt.Fprintf(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
		flusher.Flush()
	}))
}

// staticTokenSource is a simple TokenSource for testing.
type staticTokenSource struct {
	token string
}

func (s *staticTokenSource) Token() (string, error) {
	return s.token, nil
}

// ---------------------------------------------------------------------------
// Setup helpers
// ---------------------------------------------------------------------------

// setupStreamEnv creates a full test environment with a mock Claude server,
// stream client, session store, hub with handler, and returns everything.
func setupStreamEnv(t *testing.T, tokens []string) (*ws.Hub, *session.Store, *httptest.Server, context.CancelFunc) {
	t.Helper()

	// Mock Claude API.
	claudeSrv := mockClaudeStreamServer(t, tokens)
	t.Cleanup(claudeSrv.Close)

	// Stream client pointing at mock server.
	sc := stream.NewClient(
		&staticTokenSource{token: "test-token"},
		stream.WithBaseURL(claudeSrv.URL),
	)

	// Session store.
	dbPath := filepath.Join(t.TempDir(), "stream-integration.db")
	store, err := session.Open(dbPath)
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	// Hub + handler with stream client.
	hub := ws.NewHub(ws.WithSessionStore(store))
	handler := ws.NewMessageHandler(hub, store, nil, ws.WithStreamClient(sc))
	hub.SetHandler(handler)

	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	return hub, store, claudeSrv, cancel
}

// wsStreamServer creates an httptest server for WebSocket upgrades.
func wsStreamServer(t *testing.T, hub *ws.Hub) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleUpgrade(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// ---------------------------------------------------------------------------
// Test 1: Full pipeline — chat.send → stream → chat.token events → chat.done
// ---------------------------------------------------------------------------

func TestStream_FullPipeline_TokensAndDone(t *testing.T) {
	tokens := []string{"Hello", " ", "world", "!"}
	hub, store, _, cancel := setupStreamEnv(t, tokens)
	defer cancel()

	sess, err := store.CreateSession("Stream Test")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.Background()
	srv := wsStreamServer(t, hub)

	conn := dial(t, ctx, srv)
	defer conn.Close(websocket.StatusNormalClosure, "")

	drainInitial(t, ctx, conn)

	// Send chat.send.
	sendJSON(t, ctx, conn, `{"type":"chat.send","sessionId":"`+sess.ID+`","content":"Hello Claude"}`)

	// Collect all messages until chat.done.
	var tokenMsgs []map[string]interface{}
	var doneMsg map[string]interface{}

	timeout := time.After(10 * time.Second)
	for {
		select {
		case <-timeout:
			t.Fatal("timed out waiting for chat.done")
		default:
		}

		msg := readJSON(t, ctx, conn)
		msgType := msg["type"].(string)

		switch msgType {
		case "chat.token":
			tokenMsgs = append(tokenMsgs, msg)
		case "chat.done":
			doneMsg = msg
		case "chat.error":
			data := msg["data"].(map[string]interface{})
			t.Fatalf("received chat.error: %v", data["error"])
		default:
			t.Fatalf("unexpected message type: %s", msgType)
		}

		if doneMsg != nil {
			break
		}
	}

	// Verify we received the correct number of token events.
	if len(tokenMsgs) != len(tokens) {
		t.Errorf("expected %d token events, got %d", len(tokens), len(tokenMsgs))
	}

	// Verify each token.
	for i, msg := range tokenMsgs {
		data := msg["data"].(map[string]interface{})
		if data["token"] != tokens[i] {
			t.Errorf("token[%d]: expected %q, got %q", i, tokens[i], data["token"])
		}
		if msg["sessionId"] != sess.ID {
			t.Errorf("token[%d]: sessionId = %v, want %s", i, msg["sessionId"], sess.ID)
		}
	}

	// Verify chat.done.
	if doneMsg["sessionId"] != sess.ID {
		t.Errorf("chat.done sessionId = %v, want %s", doneMsg["sessionId"], sess.ID)
	}
	doneData := doneMsg["data"].(map[string]interface{})
	if doneData["messageId"] == nil || doneData["messageId"] == "" {
		t.Error("chat.done should have a non-empty messageId")
	}
}

// ---------------------------------------------------------------------------
// Test 2: Token events include correct message IDs
// ---------------------------------------------------------------------------

func TestStream_TokensHaveMessageID(t *testing.T) {
	tokens := []string{"Hi", "!"}
	hub, store, _, cancel := setupStreamEnv(t, tokens)
	defer cancel()

	sess, err := store.CreateSession("MsgID Test")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.Background()
	srv := wsStreamServer(t, hub)
	conn := dial(t, ctx, srv)
	defer conn.Close(websocket.StatusNormalClosure, "")
	drainInitial(t, ctx, conn)

	sendJSON(t, ctx, conn, `{"type":"chat.send","sessionId":"`+sess.ID+`","content":"test"}`)

	var messageIDs []string
	timeout := time.After(10 * time.Second)
	done := false
	for !done {
		select {
		case <-timeout:
			t.Fatal("timed out")
		default:
		}
		msg := readJSON(t, ctx, conn)
		switch msg["type"].(string) {
		case "chat.token":
			data := msg["data"].(map[string]interface{})
			messageIDs = append(messageIDs, data["messageId"].(string))
		case "chat.done":
			done = true
		case "chat.error":
			data := msg["data"].(map[string]interface{})
			t.Fatalf("chat.error: %v", data["error"])
		}
	}

	// All tokens should have the same message ID (from the API response).
	if len(messageIDs) == 0 {
		t.Fatal("expected at least one token with a messageId")
	}

	firstID := messageIDs[0]
	if firstID == "" {
		t.Error("messageId should not be empty")
	}
	for i, id := range messageIDs {
		if id != firstID {
			t.Errorf("token[%d] messageId %q differs from token[0] %q", i, id, firstID)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 3: Assistant message is stored after streaming
// ---------------------------------------------------------------------------

func TestStream_AssistantMessageStored(t *testing.T) {
	tokens := []string{"The ", "answer ", "is ", "42."}
	hub, store, _, cancel := setupStreamEnv(t, tokens)
	defer cancel()

	sess, err := store.CreateSession("Storage Test")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.Background()
	srv := wsStreamServer(t, hub)
	conn := dial(t, ctx, srv)
	defer conn.Close(websocket.StatusNormalClosure, "")
	drainInitial(t, ctx, conn)

	sendJSON(t, ctx, conn, `{"type":"chat.send","sessionId":"`+sess.ID+`","content":"What is the answer?"}`)

	// Wait for chat.done.
	timeout := time.After(10 * time.Second)
	done := false
	for !done {
		select {
		case <-timeout:
			t.Fatal("timed out")
		default:
		}
		msg := readJSON(t, ctx, conn)
		switch msg["type"].(string) {
		case "chat.token":
			// consume
		case "chat.done":
			done = true
		case "chat.error":
			data := msg["data"].(map[string]interface{})
			t.Fatalf("chat.error: %v", data["error"])
		}
	}

	// Check stored messages.
	msgs, err := store.GetMessages(sess.ID)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 stored messages (user + assistant), got %d", len(msgs))
	}

	if msgs[0].Role != "user" || msgs[0].Content != "What is the answer?" {
		t.Errorf("user message: role=%q content=%q", msgs[0].Role, msgs[0].Content)
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "The answer is 42." {
		t.Errorf("assistant message: role=%q content=%q", msgs[1].Role, msgs[1].Content)
	}
}

// ---------------------------------------------------------------------------
// Test 4: No stream client falls back to immediate chat.done
// ---------------------------------------------------------------------------

func TestStream_NoStreamClient_FallbackBehavior(t *testing.T) {
	// Setup without stream client (nil).
	dbPath := filepath.Join(t.TempDir(), "no-stream.db")
	store, err := session.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	hub := ws.NewHub(ws.WithSessionStore(store))
	handler := ws.NewMessageHandler(hub, store, nil) // No WithStreamClient
	hub.SetHandler(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	sess, err := store.CreateSession("Fallback Test")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	srv := wsStreamServer(t, hub)
	conn := dial(t, ctx, srv)
	defer conn.Close(websocket.StatusNormalClosure, "")
	drainInitial(t, ctx, conn)

	sendJSON(t, ctx, conn, `{"type":"chat.send","sessionId":"`+sess.ID+`","content":"hello"}`)

	msg := readJSON(t, ctx, conn)
	if msg["type"] != "chat.done" {
		t.Errorf("expected immediate chat.done, got %v", msg["type"])
	}

	// Should only have 1 message (user), no assistant.
	msgs, err := store.GetMessages(sess.ID)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Errorf("expected user message, got role=%q", msgs[0].Role)
	}
}

// ---------------------------------------------------------------------------
// Test 5: Conversation history is sent to the API
// ---------------------------------------------------------------------------

func TestStream_ConversationHistorySent(t *testing.T) {
	var receivedMessages []stream.Message

	// Custom server that captures the request messages.
	captureSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req stream.Request
		json.NewDecoder(r.Body).Decode(&req)
		receivedMessages = req.Messages

		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		fmt.Fprintf(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_h\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"test\",\"content\":[]}}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"ok\"}}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":1}}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
		flusher.Flush()
	}))
	defer captureSrv.Close()

	sc := stream.NewClient(
		&staticTokenSource{token: "test"},
		stream.WithBaseURL(captureSrv.URL),
	)

	dbPath := filepath.Join(t.TempDir(), "history.db")
	store, err := session.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	hub := ws.NewHub(ws.WithSessionStore(store))
	handler := ws.NewMessageHandler(hub, store, nil, ws.WithStreamClient(sc))
	hub.SetHandler(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	sess, err := store.CreateSession("History Test")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Pre-populate conversation history.
	store.AddMessage(sess.ID, "user", "First question")
	store.AddMessage(sess.ID, "assistant", "First answer")

	srv := wsStreamServer(t, hub)
	conn := dial(t, ctx, srv)
	defer conn.Close(websocket.StatusNormalClosure, "")
	drainInitial(t, ctx, conn)

	// Send a follow-up message.
	sendJSON(t, ctx, conn, `{"type":"chat.send","sessionId":"`+sess.ID+`","content":"Second question"}`)

	// Wait for chat.done.
	timeout := time.After(10 * time.Second)
	done := false
	for !done {
		select {
		case <-timeout:
			t.Fatal("timed out")
		default:
		}
		msg := readJSON(t, ctx, conn)
		switch msg["type"].(string) {
		case "chat.token", "chat.done":
			if msg["type"].(string) == "chat.done" {
				done = true
			}
		case "chat.error":
			data := msg["data"].(map[string]interface{})
			t.Fatalf("chat.error: %v", data["error"])
		}
	}

	// Verify the API received the full conversation history.
	if len(receivedMessages) != 3 {
		t.Fatalf("expected 3 messages sent to API, got %d", len(receivedMessages))
	}

	if receivedMessages[0].Role != "user" || receivedMessages[0].TextContent() != "First question" {
		t.Errorf("msg[0]: role=%q text=%q", receivedMessages[0].Role, receivedMessages[0].TextContent())
	}
	if receivedMessages[1].Role != "assistant" || receivedMessages[1].TextContent() != "First answer" {
		t.Errorf("msg[1]: role=%q text=%q", receivedMessages[1].Role, receivedMessages[1].TextContent())
	}
	if receivedMessages[2].Role != "user" || receivedMessages[2].TextContent() != "Second question" {
		t.Errorf("msg[2]: role=%q text=%q", receivedMessages[2].Role, receivedMessages[2].TextContent())
	}
}

// ---------------------------------------------------------------------------
// Test 6: Multiple sequential messages build up conversation
// ---------------------------------------------------------------------------

func TestStream_MultiTurnConversation(t *testing.T) {
	tokens := []string{"Response"}
	hub, store, _, cancel := setupStreamEnv(t, tokens)
	defer cancel()

	sess, err := store.CreateSession("Multi-turn")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.Background()
	srv := wsStreamServer(t, hub)
	conn := dial(t, ctx, srv)
	defer conn.Close(websocket.StatusNormalClosure, "")
	drainInitial(t, ctx, conn)

	// Send two messages sequentially.
	for i := 0; i < 2; i++ {
		sendJSON(t, ctx, conn, fmt.Sprintf(`{"type":"chat.send","sessionId":"%s","content":"Turn %d"}`, sess.ID, i+1))

		// Drain until chat.done.
		timeout := time.After(10 * time.Second)
		done := false
		for !done {
			select {
			case <-timeout:
				t.Fatalf("turn %d: timed out", i+1)
			default:
			}
			msg := readJSON(t, ctx, conn)
			switch msg["type"].(string) {
			case "chat.token":
			case "chat.done":
				done = true
			case "chat.error":
				data := msg["data"].(map[string]interface{})
				t.Fatalf("turn %d: chat.error: %v", i+1, data["error"])
			}
		}
	}

	// After 2 turns, should have 4 messages: user, assistant, user, assistant.
	msgs, err := store.GetMessages(sess.ID)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}

	expectedRoles := []string{"user", "assistant", "user", "assistant"}
	for i, msg := range msgs {
		if msg.Role != expectedRoles[i] {
			t.Errorf("msg[%d]: role=%q, want %q", i, msg.Role, expectedRoles[i])
		}
	}
}
