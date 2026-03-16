package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nhooyr.io/websocket"

	"github.com/rishav1305/soul-v2/internal/chat/session"
	"github.com/rishav1305/soul-v2/internal/chat/stream"
)

// setupTestEnv creates a Hub with a session store and MessageHandler,
// starts the hub event loop, and returns everything needed for testing.
// The returned cancel function should be deferred.
func setupTestEnv(t *testing.T) (*Hub, *session.Store, context.CancelFunc) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := session.Open(dbPath)
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	hub := NewHub(WithSessionStore(store))
	handler := NewMessageHandler(hub, store, nil)
	hub.SetHandler(handler)

	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	return hub, store, cancel
}

// connectClient creates a test HTTP server, dials a WebSocket connection,
// and returns the connection plus a cleanup function.
func connectClient(t *testing.T, ctx context.Context, hub *Hub) (*websocket.Conn, func()) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleUpgrade(w, r)
	}))

	wsURL := "ws" + srv.URL[len("http"):]
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		srv.Close()
		t.Fatalf("dial: %v", err)
	}

	// Wait for registration.
	time.Sleep(50 * time.Millisecond)

	cleanup := func() {
		conn.Close(websocket.StatusNormalClosure, "")
		srv.Close()
	}

	return conn, cleanup
}

// readMessage reads a single JSON message from the WebSocket connection
// with a timeout.
func readMessage(t *testing.T, ctx context.Context, conn *websocket.Conn) map[string]interface{} {
	t.Helper()

	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_, data, err := conn.Read(readCtx)
	if err != nil {
		t.Fatalf("read message: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal message: %v", err)
	}
	return result
}

// drainMessages reads all pending messages from the connection until timeout.
func drainMessages(t *testing.T, ctx context.Context, conn *websocket.Conn, timeout time.Duration) []map[string]interface{} {
	t.Helper()

	var msgs []map[string]interface{}
	for {
		readCtx, cancel := context.WithTimeout(ctx, timeout)
		_, data, err := conn.Read(readCtx)
		cancel()
		if err != nil {
			break
		}
		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("unmarshal message: %v", err)
		}
		msgs = append(msgs, result)
	}
	return msgs
}

func TestConnectionReady_SentOnConnect(t *testing.T) {
	hub, _, cancel := setupTestEnv(t)
	defer cancel()

	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	// First message should be connection.ready.
	msg := readMessage(t, ctx, conn)
	if msg["type"] != TypeConnectionReady {
		t.Errorf("expected connection.ready, got %v", msg["type"])
	}
	data := msg["data"].(map[string]interface{})
	if data["clientId"] == nil || data["clientId"] == "" {
		t.Error("expected non-empty clientId in connection.ready")
	}
}

func TestConnectionReady_FollowedBySessionList(t *testing.T) {
	hub, _, cancel := setupTestEnv(t)
	defer cancel()

	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	// First message: connection.ready.
	msg1 := readMessage(t, ctx, conn)
	if msg1["type"] != TypeConnectionReady {
		t.Fatalf("expected connection.ready, got %v", msg1["type"])
	}

	// Second message: session.list.
	msg2 := readMessage(t, ctx, conn)
	if msg2["type"] != TypeSessionList {
		t.Errorf("expected session.list, got %v", msg2["type"])
	}
}

func TestHandleChatSend_Success(t *testing.T) {
	hub, store, cancel := setupTestEnv(t)
	defer cancel()

	// Create a session first.
	sess, err := store.CreateSession("Test")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	// Drain connection.ready + session.list.
	_ = readMessage(t, ctx, conn) // connection.ready
	_ = readMessage(t, ctx, conn) // session.list

	// Send chat.send.
	chatMsg := `{"type":"chat.send","sessionId":"` + sess.ID + `","content":"hello"}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(chatMsg)); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Should receive chat.done (possibly preceded by session.updated broadcasts).
	var found bool
	for i := 0; i < 10; i++ {
		msg := readMessage(t, ctx, conn)
		if msg["type"] == TypeChatDone {
			if msg["sessionId"] != sess.ID {
				t.Errorf("expected sessionId %s, got %v", sess.ID, msg["sessionId"])
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("never received chat.done")
	}

	// Verify message was stored.
	messages, err := store.GetMessages(sess.ID)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].Content != "hello" {
		t.Errorf("expected content 'hello', got %q", messages[0].Content)
	}
	if messages[0].Role != "user" {
		t.Errorf("expected role 'user', got %q", messages[0].Role)
	}
}

func TestHandleChatSend_MissingSessionID(t *testing.T) {
	hub, _, cancel := setupTestEnv(t)
	defer cancel()

	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	// Drain connection.ready + session.list.
	_ = readMessage(t, ctx, conn)
	_ = readMessage(t, ctx, conn)

	// Send chat.send without sessionId.
	chatMsg := `{"type":"chat.send","content":"hello"}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(chatMsg)); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg := readMessage(t, ctx, conn)
	if msg["type"] != TypeChatError {
		t.Errorf("expected chat.error, got %v", msg["type"])
	}
	data := msg["data"].(map[string]interface{})
	if data["error"] != "session ID required" {
		t.Errorf("expected 'session ID required', got %v", data["error"])
	}
}

func TestHandleChatSend_EmptyContent(t *testing.T) {
	hub, store, cancel := setupTestEnv(t)
	defer cancel()

	sess, err := store.CreateSession("Test")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	// Drain connection.ready + session.list.
	_ = readMessage(t, ctx, conn)
	_ = readMessage(t, ctx, conn)

	// Send chat.send with empty content.
	chatMsg := `{"type":"chat.send","sessionId":"` + sess.ID + `","content":""}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(chatMsg)); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg := readMessage(t, ctx, conn)
	if msg["type"] != TypeChatError {
		t.Errorf("expected chat.error, got %v", msg["type"])
	}
	data := msg["data"].(map[string]interface{})
	if data["error"] != "message content required" {
		t.Errorf("expected 'message content required', got %v", data["error"])
	}
}

func TestHandleChatSend_SessionNotFound(t *testing.T) {
	hub, _, cancel := setupTestEnv(t)
	defer cancel()

	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	// Drain connection.ready + session.list.
	_ = readMessage(t, ctx, conn)
	_ = readMessage(t, ctx, conn)

	// Send chat.send with non-existent sessionId.
	chatMsg := `{"type":"chat.send","sessionId":"00000000-0000-0000-0000-000000000000","content":"hello"}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(chatMsg)); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg := readMessage(t, ctx, conn)
	if msg["type"] != TypeChatError {
		t.Errorf("expected chat.error, got %v", msg["type"])
	}
	data := msg["data"].(map[string]interface{})
	if data["error"] != "session not found" {
		t.Errorf("expected 'session not found', got %v", data["error"])
	}
}

func TestHandleSessionCreate_BroadcastsToAll(t *testing.T) {
	hub, _, cancel := setupTestEnv(t)
	defer cancel()

	ctx := context.Background()

	// Connect two clients.
	conn1, cleanup1 := connectClient(t, ctx, hub)
	defer cleanup1()
	conn2, cleanup2 := connectClient(t, ctx, hub)
	defer cleanup2()

	// Drain initial messages from both clients.
	_ = readMessage(t, ctx, conn1) // connection.ready
	_ = readMessage(t, ctx, conn1) // session.list
	_ = readMessage(t, ctx, conn2) // connection.ready
	_ = readMessage(t, ctx, conn2) // session.list

	// Send session.create from client 1.
	createMsg := `{"type":"session.create"}`
	if err := conn1.Write(ctx, websocket.MessageText, []byte(createMsg)); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Both clients should receive session.created.
	msg1 := readMessage(t, ctx, conn1)
	if msg1["type"] != TypeSessionCreated {
		t.Errorf("client1: expected session.created, got %v", msg1["type"])
	}

	msg2 := readMessage(t, ctx, conn2)
	if msg2["type"] != TypeSessionCreated {
		t.Errorf("client2: expected session.created, got %v", msg2["type"])
	}
}

func TestHandleSessionDelete_BroadcastsToAll(t *testing.T) {
	hub, store, cancel := setupTestEnv(t)
	defer cancel()

	sess, err := store.CreateSession("To Delete")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.Background()

	conn1, cleanup1 := connectClient(t, ctx, hub)
	defer cleanup1()
	conn2, cleanup2 := connectClient(t, ctx, hub)
	defer cleanup2()

	// Drain initial messages.
	_ = readMessage(t, ctx, conn1) // connection.ready
	_ = readMessage(t, ctx, conn1) // session.list
	_ = readMessage(t, ctx, conn2) // connection.ready
	_ = readMessage(t, ctx, conn2) // session.list

	// Send session.delete from client 1.
	deleteMsg := `{"type":"session.delete","sessionId":"` + sess.ID + `"}`
	if err := conn1.Write(ctx, websocket.MessageText, []byte(deleteMsg)); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Both clients should receive session.deleted.
	msg1 := readMessage(t, ctx, conn1)
	if msg1["type"] != TypeSessionDeleted {
		t.Errorf("client1: expected session.deleted, got %v", msg1["type"])
	}
	if msg1["sessionId"] != sess.ID {
		t.Errorf("expected sessionId %s, got %v", sess.ID, msg1["sessionId"])
	}

	msg2 := readMessage(t, ctx, conn2)
	if msg2["type"] != TypeSessionDeleted {
		t.Errorf("client2: expected session.deleted, got %v", msg2["type"])
	}
}

func TestHandleSessionDelete_MissingSessionID(t *testing.T) {
	hub, _, cancel := setupTestEnv(t)
	defer cancel()

	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	_ = readMessage(t, ctx, conn) // connection.ready
	_ = readMessage(t, ctx, conn) // session.list

	deleteMsg := `{"type":"session.delete"}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(deleteMsg)); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg := readMessage(t, ctx, conn)
	if msg["type"] != TypeChatError {
		t.Errorf("expected chat.error, got %v", msg["type"])
	}
	data := msg["data"].(map[string]interface{})
	if data["error"] != "session ID required" {
		t.Errorf("expected 'session ID required', got %v", data["error"])
	}
}

func TestHandleSessionDelete_NotFound(t *testing.T) {
	hub, _, cancel := setupTestEnv(t)
	defer cancel()

	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	_ = readMessage(t, ctx, conn) // connection.ready
	_ = readMessage(t, ctx, conn) // session.list

	deleteMsg := `{"type":"session.delete","sessionId":"00000000-0000-0000-0000-000000000000"}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(deleteMsg)); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg := readMessage(t, ctx, conn)
	if msg["type"] != TypeChatError {
		t.Errorf("expected chat.error, got %v", msg["type"])
	}
	data := msg["data"].(map[string]interface{})
	if data["error"] != "session not found" {
		t.Errorf("expected 'session not found', got %v", data["error"])
	}
}

func TestHandleSessionSwitch_Success(t *testing.T) {
	hub, store, cancel := setupTestEnv(t)
	defer cancel()

	sess, err := store.CreateSession("Test Switch")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	_ = readMessage(t, ctx, conn) // connection.ready
	_ = readMessage(t, ctx, conn) // session.list

	// Send session.switch.
	switchMsg := `{"type":"session.switch","sessionId":"` + sess.ID + `"}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(switchMsg)); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Should receive session.updated.
	msg1 := readMessage(t, ctx, conn)
	if msg1["type"] != TypeSessionUpdated {
		t.Errorf("expected session.updated, got %v", msg1["type"])
	}

	// Should receive session.list with sessions array.
	msg2 := readMessage(t, ctx, conn)
	if msg2["type"] != TypeSessionList {
		t.Errorf("expected session.list, got %v", msg2["type"])
	}
	// Verify session.list data.sessions is an array (not double-nested).
	if data, ok := msg2["data"].(map[string]interface{}); ok {
		if sessions, ok := data["sessions"].([]interface{}); ok {
			if len(sessions) == 0 {
				t.Error("expected at least one session in session.list")
			}
		} else {
			t.Errorf("expected data.sessions to be an array, got %T", data["sessions"])
		}
	}

	// Should receive session.history with messages.
	msg3 := readMessage(t, ctx, conn)
	if msg3["type"] != TypeSessionHistory {
		t.Errorf("expected session.history, got %v", msg3["type"])
	}

	// Verify client subscription was updated.
	clients := hub.Clients()
	if len(clients) == 0 {
		t.Fatal("expected at least one client")
	}
	if clients[0].SessionID() != sess.ID {
		t.Errorf("expected client subscribed to %s, got %s", sess.ID, clients[0].SessionID())
	}
}

func TestHandleSessionSwitch_MissingSessionID(t *testing.T) {
	hub, _, cancel := setupTestEnv(t)
	defer cancel()

	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	_ = readMessage(t, ctx, conn) // connection.ready
	_ = readMessage(t, ctx, conn) // session.list

	switchMsg := `{"type":"session.switch"}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(switchMsg)); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg := readMessage(t, ctx, conn)
	if msg["type"] != TypeChatError {
		t.Errorf("expected chat.error, got %v", msg["type"])
	}
	data := msg["data"].(map[string]interface{})
	if data["error"] != "session ID required" {
		t.Errorf("expected 'session ID required', got %v", data["error"])
	}
}

func TestHandleSessionSwitch_NotFound(t *testing.T) {
	hub, _, cancel := setupTestEnv(t)
	defer cancel()

	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	_ = readMessage(t, ctx, conn) // connection.ready
	_ = readMessage(t, ctx, conn) // session.list

	switchMsg := `{"type":"session.switch","sessionId":"00000000-0000-0000-0000-000000000000"}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(switchMsg)); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg := readMessage(t, ctx, conn)
	if msg["type"] != TypeChatError {
		t.Errorf("expected chat.error, got %v", msg["type"])
	}
	data := msg["data"].(map[string]interface{})
	if data["error"] != "session not found" {
		t.Errorf("expected 'session not found', got %v", data["error"])
	}
}

func TestHandleInvalidJSON(t *testing.T) {
	hub, _, cancel := setupTestEnv(t)
	defer cancel()

	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	_ = readMessage(t, ctx, conn) // connection.ready
	_ = readMessage(t, ctx, conn) // session.list

	// Send invalid JSON.
	if err := conn.Write(ctx, websocket.MessageText, []byte(`not json at all`)); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg := readMessage(t, ctx, conn)
	if msg["type"] != TypeChatError {
		t.Errorf("expected chat.error, got %v", msg["type"])
	}
}

func TestHandleUnknownType_NoResponse(t *testing.T) {
	hub, _, cancel := setupTestEnv(t)
	defer cancel()

	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	_ = readMessage(t, ctx, conn) // connection.ready
	_ = readMessage(t, ctx, conn) // session.list

	// Send unknown type.
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"unknown.thing"}`)); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Should not receive any response — verify with a short timeout.
	readCtx, readCancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer readCancel()

	_, _, err := conn.Read(readCtx)
	if err == nil {
		t.Error("expected no response for unknown message type, but got one")
	}
}

func TestBroadcast(t *testing.T) {
	hub, _, cancel := setupTestEnv(t)
	defer cancel()

	ctx := context.Background()

	conn1, cleanup1 := connectClient(t, ctx, hub)
	defer cleanup1()
	conn2, cleanup2 := connectClient(t, ctx, hub)
	defer cleanup2()

	// Drain initial messages.
	_ = readMessage(t, ctx, conn1) // connection.ready
	_ = readMessage(t, ctx, conn1) // session.list
	_ = readMessage(t, ctx, conn2) // connection.ready
	_ = readMessage(t, ctx, conn2) // session.list

	// Broadcast a message.
	testMsg := `{"type":"test.broadcast","data":"hello all"}`
	hub.Broadcast([]byte(testMsg))

	msg1 := readMessage(t, ctx, conn1)
	if msg1["type"] != "test.broadcast" {
		t.Errorf("conn1: expected test.broadcast, got %v", msg1["type"])
	}

	msg2 := readMessage(t, ctx, conn2)
	if msg2["type"] != "test.broadcast" {
		t.Errorf("conn2: expected test.broadcast, got %v", msg2["type"])
	}
}

func TestBroadcastToSession(t *testing.T) {
	hub, store, cancel := setupTestEnv(t)
	defer cancel()

	sess, err := store.CreateSession("Target Session")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.Background()

	conn1, cleanup1 := connectClient(t, ctx, hub)
	defer cleanup1()
	conn2, cleanup2 := connectClient(t, ctx, hub)
	defer cleanup2()

	// Drain initial messages.
	_ = readMessage(t, ctx, conn1) // connection.ready
	_ = readMessage(t, ctx, conn1) // session.list
	_ = readMessage(t, ctx, conn2) // connection.ready
	_ = readMessage(t, ctx, conn2) // session.list

	// Subscribe client 1 to the session.
	switchMsg := `{"type":"session.switch","sessionId":"` + sess.ID + `"}`
	if err := conn1.Write(ctx, websocket.MessageText, []byte(switchMsg)); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Drain session.switch response messages from conn1.
	_ = readMessage(t, ctx, conn1) // session.updated
	_ = readMessage(t, ctx, conn1) // session.list
	_ = readMessage(t, ctx, conn1) // session.history

	// Broadcast to session — only client 1 should receive.
	testMsg := `{"type":"test.session","data":"session only"}`
	hub.BroadcastToSession(sess.ID, []byte(testMsg))

	msg1 := readMessage(t, ctx, conn1)
	if msg1["type"] != "test.session" {
		t.Errorf("conn1: expected test.session, got %v", msg1["type"])
	}

	// Client 2 should NOT receive the session broadcast.
	readCtx, readCancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer readCancel()

	_, _, err = conn2.Read(readCtx)
	if err == nil {
		t.Error("expected conn2 to NOT receive session-scoped broadcast")
	}
}

func TestNewMessageHandler(t *testing.T) {
	hub := NewHub()
	handler := NewMessageHandler(hub, nil, nil)
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
	if handler.hub != hub {
		t.Error("expected handler.hub to match")
	}
}

func TestBuildAPIMessages_MergesConsecutiveRoles(t *testing.T) {
	sessionMsgs := []*session.Message{
		{Role: "user", Content: "first"},
		{Role: "user", Content: "second"},
		{Role: "assistant", Content: "reply one"},
		{Role: "assistant", Content: "reply two"},
	}

	got := buildAPIMessages(sessionMsgs)
	if len(got) != 2 {
		t.Fatalf("expected 2 API messages after normalization, got %d", len(got))
	}
	if got[0].Role != "user" {
		t.Fatalf("got[0].Role = %q, want %q", got[0].Role, "user")
	}
	if got[1].Role != "assistant" {
		t.Fatalf("got[1].Role = %q, want %q", got[1].Role, "assistant")
	}
	if len(got[0].Content) != 2 {
		t.Fatalf("user content blocks = %d, want 2", len(got[0].Content))
	}
	if len(got[1].Content) != 2 {
		t.Fatalf("assistant content blocks = %d, want 2", len(got[1].Content))
	}
	if got[0].Content[0].Text != "first" || got[0].Content[1].Text != "second" {
		t.Fatalf("unexpected user text blocks: %#v", got[0].Content)
	}
	if got[1].Content[0].Text != "reply one" || got[1].Content[1].Text != "reply two" {
		t.Fatalf("unexpected assistant text blocks: %#v", got[1].Content)
	}
}

func TestBuildAPIMessages_MergesToolAndTextByRole(t *testing.T) {
	toolUseBlocks := []stream.ContentBlock{
		{
			Type:  "tool_use",
			ID:    "tool-1",
			Name:  "demo_tool",
			Input: json.RawMessage(`{"k":"v"}`),
		},
	}
	toolUseJSON, err := json.Marshal(toolUseBlocks)
	if err != nil {
		t.Fatalf("marshal tool_use: %v", err)
	}

	toolResultJSON, err := json.Marshal(struct {
		ToolUseID string `json:"tool_use_id"`
		Content   string `json:"content"`
	}{
		ToolUseID: "tool-1",
		Content:   "done",
	})
	if err != nil {
		t.Fatalf("marshal tool_result: %v", err)
	}

	sessionMsgs := []*session.Message{
		{Role: "assistant", Content: "prelude"},
		{Role: "tool_use", Content: string(toolUseJSON)},
		{Role: "tool_result", Content: string(toolResultJSON)},
		{Role: "user", Content: "follow-up question"},
	}

	got := buildAPIMessages(sessionMsgs)
	if len(got) != 2 {
		t.Fatalf("expected 2 API messages after normalization, got %d", len(got))
	}

	assistant := got[0]
	if assistant.Role != "assistant" {
		t.Fatalf("assistant role = %q, want %q", assistant.Role, "assistant")
	}
	if len(assistant.Content) != 2 {
		t.Fatalf("assistant content blocks = %d, want 2", len(assistant.Content))
	}
	if assistant.Content[0].Type != "text" || assistant.Content[0].Text != "prelude" {
		t.Fatalf("unexpected first assistant block: %#v", assistant.Content[0])
	}
	if assistant.Content[1].Type != "tool_use" || assistant.Content[1].ID != "tool-1" {
		t.Fatalf("unexpected second assistant block: %#v", assistant.Content[1])
	}

	user := got[1]
	if user.Role != "user" {
		t.Fatalf("user role = %q, want %q", user.Role, "user")
	}
	if len(user.Content) != 2 {
		t.Fatalf("user content blocks = %d, want 2", len(user.Content))
	}
	if user.Content[0].Type != "tool_result" || user.Content[0].ToolUseID != "tool-1" {
		t.Fatalf("unexpected first user block: %#v", user.Content[0])
	}
	if user.Content[1].Type != "text" || user.Content[1].Text != "follow-up question" {
		t.Fatalf("unexpected second user block: %#v", user.Content[1])
	}
}

// --- Security: Input Validation Tests ---

func TestSecurity_ChatSend_OversizedContent(t *testing.T) {
	hub, store, cancel := setupTestEnv(t)
	defer cancel()

	sess, err := store.CreateSession("Test")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	_ = readMessage(t, ctx, conn) // connection.ready
	_ = readMessage(t, ctx, conn) // session.list

	// Create content larger than 32KB.
	bigContent := strings.Repeat("x", 33*1024)
	chatMsg := `{"type":"chat.send","sessionId":"` + sess.ID + `","content":"` + bigContent + `"}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(chatMsg)); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg := readMessage(t, ctx, conn)
	if msg["type"] != TypeChatError {
		t.Errorf("expected chat.error, got %v", msg["type"])
	}
	data := msg["data"].(map[string]interface{})
	if data["error"] != "message too large" {
		t.Errorf("expected 'message too large', got %v", data["error"])
	}

	// Verify message was NOT stored.
	messages, err := store.GetMessages(sess.ID)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("expected 0 messages (oversized content rejected), got %d", len(messages))
	}
}

func TestSecurity_ChatSend_WhitespaceOnlyContent(t *testing.T) {
	hub, store, cancel := setupTestEnv(t)
	defer cancel()

	sess, err := store.CreateSession("Test")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	_ = readMessage(t, ctx, conn) // connection.ready
	_ = readMessage(t, ctx, conn) // session.list

	// Send whitespace-only content.
	chatMsg := `{"type":"chat.send","sessionId":"` + sess.ID + `","content":"   \n\t  "}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(chatMsg)); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg := readMessage(t, ctx, conn)
	if msg["type"] != TypeChatError {
		t.Errorf("expected chat.error, got %v", msg["type"])
	}
	data := msg["data"].(map[string]interface{})
	if data["error"] != "message content required" {
		t.Errorf("expected 'message content required', got %v", data["error"])
	}
}

func TestSecurity_ChatSend_InvalidSessionID(t *testing.T) {
	hub, _, cancel := setupTestEnv(t)
	defer cancel()

	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	_ = readMessage(t, ctx, conn) // connection.ready
	_ = readMessage(t, ctx, conn) // session.list

	// Send chat.send with invalid UUID format.
	chatMsg := `{"type":"chat.send","sessionId":"not-a-valid-uuid","content":"hello"}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(chatMsg)); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg := readMessage(t, ctx, conn)
	if msg["type"] != TypeChatError {
		t.Errorf("expected chat.error, got %v", msg["type"])
	}
	data := msg["data"].(map[string]interface{})
	if data["error"] != "invalid session ID" {
		t.Errorf("expected 'invalid session ID', got %v", data["error"])
	}
}

func TestSecurity_SessionSwitch_InvalidSessionID(t *testing.T) {
	hub, _, cancel := setupTestEnv(t)
	defer cancel()

	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	_ = readMessage(t, ctx, conn) // connection.ready
	_ = readMessage(t, ctx, conn) // session.list

	switchMsg := `{"type":"session.switch","sessionId":"bad-uuid"}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(switchMsg)); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg := readMessage(t, ctx, conn)
	if msg["type"] != TypeChatError {
		t.Errorf("expected chat.error, got %v", msg["type"])
	}
	data := msg["data"].(map[string]interface{})
	if data["error"] != "invalid session ID" {
		t.Errorf("expected 'invalid session ID', got %v", data["error"])
	}
}

func TestSecurity_SessionDelete_InvalidSessionID(t *testing.T) {
	hub, _, cancel := setupTestEnv(t)
	defer cancel()

	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	_ = readMessage(t, ctx, conn) // connection.ready
	_ = readMessage(t, ctx, conn) // session.list

	deleteMsg := `{"type":"session.delete","sessionId":";;;DROP TABLE"}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(deleteMsg)); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg := readMessage(t, ctx, conn)
	if msg["type"] != TypeChatError {
		t.Errorf("expected chat.error, got %v", msg["type"])
	}
	data := msg["data"].(map[string]interface{})
	if data["error"] != "invalid session ID" {
		t.Errorf("expected 'invalid session ID', got %v", data["error"])
	}
}

func TestSecurity_MessageType_TooLong(t *testing.T) {
	hub, _, cancel := setupTestEnv(t)
	defer cancel()

	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	_ = readMessage(t, ctx, conn) // connection.ready
	_ = readMessage(t, ctx, conn) // session.list

	// Send message with type field longer than 50 chars.
	longType := strings.Repeat("a", 51)
	chatMsg := `{"type":"` + longType + `"}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(chatMsg)); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg := readMessage(t, ctx, conn)
	if msg["type"] != TypeChatError {
		t.Errorf("expected chat.error, got %v", msg["type"])
	}
	data := msg["data"].(map[string]interface{})
	if data["error"] != "invalid message type" {
		t.Errorf("expected 'invalid message type', got %v", data["error"])
	}
}

func TestHandleSessionResume_ReplaysMissedMessages(t *testing.T) {
	hub, store, cancel := setupTestEnv(t)
	defer cancel()
	_ = store

	// Give the hub a replay buffer.
	rb := NewReplayBuffer(100, 10)
	hub.replay = rb

	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	_ = readMessage(t, ctx, conn) // connection.ready
	_ = readMessage(t, ctx, conn) // session.list

	// Create a session.
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"session.create"}`)); err != nil {
		t.Fatalf("write session.create: %v", err)
	}
	created := readMessage(t, ctx, conn)
	if created["type"] != TypeSessionCreated {
		t.Fatalf("expected session.created, got %v", created["type"])
	}
	createdData := created["data"].(map[string]interface{})
	sess := createdData["session"].(map[string]interface{})
	sessionID := sess["id"].(string)

	// Manually store two replay entries (simulating missed messages).
	missedMsgID := sessionID + "-anchor"
	missed := []byte(`{"type":"chat.token","sessionId":"` + sessionID + `","messageId":"` + missedMsgID + `","data":{"token":"missed"}}`)
	rb.Store(sessionID, missedMsgID, missed)

	laterMsgID := sessionID + "-later"
	later := []byte(`{"type":"chat.done","sessionId":"` + sessionID + `","messageId":"` + laterMsgID + `","data":{}}`)
	rb.Store(sessionID, laterMsgID, later)

	// Send session.resume with anchor pointing to the first entry —
	// server should replay only entries after the anchor.
	resumeMsg := `{"type":"session.resume","sessionId":"` + sessionID + `","lastMessageId":"` + missedMsgID + `"}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(resumeMsg)); err != nil {
		t.Fatalf("write session.resume: %v", err)
	}

	// Expect the replayed later message.
	replayed := readMessage(t, ctx, conn)
	if replayed["type"] != "chat.done" {
		t.Errorf("expected replayed chat.done, got %v", replayed["type"])
	}
}

func TestHandleSessionResume_AnchorMissing_NoReplay(t *testing.T) {
	hub, _, cancel := setupTestEnv(t)
	defer cancel()

	hub.replay = NewReplayBuffer(100, 10)

	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	_ = readMessage(t, ctx, conn)
	_ = readMessage(t, ctx, conn)

	// Resume with an anchor that was never stored — server emits a metric
	// but sends nothing back to the client.
	resumeMsg := `{"type":"session.resume","sessionId":"nonexistent","lastMessageId":"ghost-anchor"}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(resumeMsg)); err != nil {
		t.Fatalf("write session.resume: %v", err)
	}

	// No message should arrive within a short window.
	msgs := drainMessages(t, ctx, conn, 150*time.Millisecond)
	if len(msgs) > 0 {
		t.Errorf("expected no replay messages, got %d: first type=%v", len(msgs), msgs[0]["type"])
	}
}

func TestHandleChatSend_DedupSendsDone(t *testing.T) {
	hub, store, cancel := setupTestEnv(t)
	defer cancel()

	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	_ = readMessage(t, ctx, conn) // connection.ready
	_ = readMessage(t, ctx, conn) // session.list

	// Create a session.
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"session.create"}`)); err != nil {
		t.Fatalf("write session.create: %v", err)
	}
	created := readMessage(t, ctx, conn)
	createdData := created["data"].(map[string]interface{})
	sess := createdData["session"].(map[string]interface{})
	sessionID := sess["id"].(string)

	// Pre-seed the handler's seenMessages with the target messageId.
	handler := NewMessageHandler(hub, store, nil)
	hub.SetHandler(handler)
	handler.seenMu.Lock()
	handler.seenMessages["dup-msg-id"] = time.Now()
	handler.seenMu.Unlock()

	// Send chat.send with the duplicate messageId — expect chat.done, not silence.
	chatMsg := `{"type":"chat.send","sessionId":"` + sessionID + `","content":"hello","messageId":"dup-msg-id"}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(chatMsg)); err != nil {
		t.Fatalf("write chat.send: %v", err)
	}

	msg := readMessage(t, ctx, conn)
	if msg["type"] != TypeChatDone {
		t.Errorf("expected chat.done on dedup, got %v", msg["type"])
	}
}

// --- runStream context.Canceled behaviour ---

// staticTokenSource satisfies stream.TokenSource for tests.
type staticTokenSource struct{ token string }

func (s *staticTokenSource) Token() (string, error) { return s.token, nil }

func TestHandleChatStop_CompletesSession(t *testing.T) {
	hub, store, cancel := setupTestEnv(t)
	defer cancel()
	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	// Drain the two initial messages (connection.ready + session.list).
	_ = readMessage(t, ctx, conn)
	_ = readMessage(t, ctx, conn)

	sess, _ := store.CreateSession("Test")
	_ = store.UpdateSessionStatus(sess.ID, session.StatusRunning)

	clients := hub.Clients()
	if len(clients) == 0 {
		t.Fatal("no clients")
	}
	client := clients[0]

	// Add a fake agent entry that completes immediately when cancelled.
	done := make(chan struct{})
	handler := hub.handler
	cs := handler.getOrCreateChatSession(client)
	cs.mu.Lock()
	cs.agents[sess.ID] = agentEntry{
		cancel: func() { close(done) },
		done:   done,
	}
	cs.mu.Unlock()

	// Send chat.stop.
	stopMsg, _ := json.Marshal(map[string]interface{}{
		"type":      TypeChatStop,
		"sessionId": sess.ID,
	})
	conn.Write(ctx, websocket.MessageText, stopMsg)

	// Wait for session status to change.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		updated, _ := store.GetSession(sess.ID)
		if updated.Status != session.StatusRunning {
			return // test passes
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("session still running after chat.stop, expected completed")
}

func TestHandleChatSend_SupersededStream_RestoresRunningStatus(t *testing.T) {
	hub, store, cancel := setupTestEnv(t)
	defer cancel()
	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	// Drain the two initial messages (connection.ready + session.list).
	_ = readMessage(t, ctx, conn)
	_ = readMessage(t, ctx, conn)

	sess, _ := store.CreateSession("Test")
	_ = store.UpdateSessionStatus(sess.ID, session.StatusRunning)
	// Add a user message so handleChatSend doesn't fail validation
	_, _ = store.AddMessage(sess.ID, "user", "hello")

	clients := hub.Clients()
	if len(clients) == 0 {
		t.Fatal("no clients")
	}
	client := clients[0]

	// Add a fake existing agent.
	done := make(chan struct{})
	handler := hub.handler
	cs := handler.getOrCreateChatSession(client)
	cs.mu.Lock()
	cs.agents[sess.ID] = agentEntry{
		cancel: func() {},
		done:   done,
	}
	cs.mu.Unlock()

	// Simulate the old stream completing after a delay.
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(done)
	}()

	// Send chat.send to trigger superseded path.
	sendMsg, _ := json.Marshal(map[string]interface{}{
		"type":      TypeChatSend,
		"sessionId": sess.ID,
		"content":   "new message",
	})
	conn.Write(ctx, websocket.MessageText, sendMsg)

	// Poll until session transitions back to running (or timeout).
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		updated, _ := store.GetSession(sess.ID)
		if updated.Status == session.StatusRunning {
			return // test passes
		}
		time.Sleep(10 * time.Millisecond)
	}
	final, _ := store.GetSession(sess.ID)
	t.Errorf("session status = %v, want running after superseded stream", final.Status)
}

// TestRunStream_CanceledContext_SilentReturn verifies that runStream returns
// silently when the agent context is already cancelled, without completing the
// session or sending a chat.error to the client.
//
// The stream.Client is pointed at an unreachable address. With a pre-cancelled
// context, http.Client.Do immediately returns a wrapped context.Canceled error,
// which is the exact path the fix guards.
func TestRunStream_CanceledContext_SilentReturn(t *testing.T) {
	hub, store, cancel := setupTestEnv(t)
	defer cancel()

	ctx := context.Background()
	conn, cleanup := connectClient(t, ctx, hub)
	defer cleanup()

	// Drain connection.ready + session.list.
	_ = readMessage(t, ctx, conn)
	_ = readMessage(t, ctx, conn)

	// Create a session and set it to StatusRunning so we can detect any
	// unintended status transition.
	sess, err := store.CreateSession("CancelTest")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := store.UpdateSessionStatus(sess.ID, session.StatusRunning); err != nil {
		t.Fatalf("set running: %v", err)
	}

	// Grab the hub's internal client handle.
	clients := hub.Clients()
	if len(clients) == 0 {
		t.Fatal("expected at least one connected client")
	}
	wsClient := clients[0]

	// Build a real stream.Client pointed at a test server that never responds.
	// We pre-cancel the agentCtx so http.Do returns context.Canceled immediately
	// — before the server even needs to reply.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block forever — we should never reach here because the context is
		// pre-cancelled and http.Do returns before the request is sent.
		select {}
	}))
	defer srv.Close()

	sc := stream.NewClient(
		&staticTokenSource{token: "test-token"},
		stream.WithBaseURL(srv.URL),
	)

	// Create a handler wired with the stream client.
	handler := NewMessageHandler(hub, store, nil, WithStreamClient(sc))

	// Build a minimal stream request.
	req := &stream.Request{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 64,
		Messages: []stream.Message{
			{Role: "user", Content: []stream.ContentBlock{{Type: "text", Text: "ping"}}},
		},
		SkipValidation: true,
	}

	// Pre-cancel the agent context — this is the key condition under test.
	agentCtx, agentCancel := context.WithCancel(context.Background())
	agentCancel()

	// runStream should return quickly and silently.
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.runStream(wsClient, sess.ID, req, agentCtx, "")
	}()

	select {
	case <-done:
		// Good: runStream returned.
	case <-time.After(3 * time.Second):
		t.Fatal("runStream did not return within 3s — likely blocked instead of returning on context.Canceled")
	}

	// Session must still be StatusRunning — runStream must NOT have completed it.
	updated, err := store.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if updated.Status != session.StatusRunning {
		t.Errorf("session status = %q, want %q — runStream incorrectly changed session lifecycle on context.Canceled",
			updated.Status, session.StatusRunning)
	}

	// No chat.error should have been sent to the client.
	msgs := drainMessages(t, ctx, conn, 300*time.Millisecond)
	for _, m := range msgs {
		if m["type"] == TypeChatError {
			t.Errorf("unexpected chat.error emitted on context.Canceled: %v", m)
		}
	}
}

