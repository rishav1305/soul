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
