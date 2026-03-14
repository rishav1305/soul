package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"nhooyr.io/websocket"

	"github.com/rishav1305/soul-v2/internal/chat/metrics"
	"github.com/rishav1305/soul-v2/internal/chat/server"
	"github.com/rishav1305/soul-v2/internal/chat/session"
	"github.com/rishav1305/soul-v2/internal/chat/ws"
)

// ---------------------------------------------------------------------------
// Invariant 1: Session store round-trip
// Create a session, add messages, retrieve, verify all fields survived.
// ---------------------------------------------------------------------------

func TestInvariant_SessionStoreRoundTrip(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "invariant.db")
	store, err := session.Open(dbPath)
	if err != nil {
		t.Fatalf("session.Open: %v", err)
	}
	defer store.Close()

	// Create session.
	sess, err := store.CreateSession("Invariant Test")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.ID == "" {
		t.Fatal("session ID must not be empty")
	}
	if sess.Title != "Invariant Test" {
		t.Errorf("Title = %q, want %q", sess.Title, "Invariant Test")
	}

	// Add messages.
	msg1, err := store.AddMessage(sess.ID, "user", "What is 2+2?")
	if err != nil {
		t.Fatalf("AddMessage user: %v", err)
	}
	msg2, err := store.AddMessage(sess.ID, "assistant", "4")
	if err != nil {
		t.Fatalf("AddMessage assistant: %v", err)
	}

	// Retrieve and verify session.
	got, err := store.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.ID != sess.ID {
		t.Errorf("ID mismatch: %q != %q", got.ID, sess.ID)
	}
	if got.Title != "Invariant Test" {
		t.Errorf("Title = %q, want %q", got.Title, "Invariant Test")
	}
	if got.MessageCount != 2 {
		t.Errorf("MessageCount = %d, want 2", got.MessageCount)
	}

	// Retrieve and verify messages.
	msgs, err := store.GetMessages(sess.ID)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	if msgs[0].ID != msg1.ID {
		t.Errorf("msg1 ID mismatch")
	}
	if msgs[0].Role != "user" {
		t.Errorf("msg1 Role = %q, want user", msgs[0].Role)
	}
	if msgs[0].Content != "What is 2+2?" {
		t.Errorf("msg1 Content = %q, want %q", msgs[0].Content, "What is 2+2?")
	}

	if msgs[1].ID != msg2.ID {
		t.Errorf("msg2 ID mismatch")
	}
	if msgs[1].Role != "assistant" {
		t.Errorf("msg2 Role = %q, want assistant", msgs[1].Role)
	}
	if msgs[1].Content != "4" {
		t.Errorf("msg2 Content = %q, want 4", msgs[1].Content)
	}

	// Verify ordering: msg1 comes before msg2.
	if msgs[0].CreatedAt.After(msgs[1].CreatedAt) {
		t.Error("msg1 should be created before msg2")
	}

	// Verify session appears in list.
	sessions, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	found := false
	for _, s := range sessions {
		if s.ID == sess.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("session not found in ListSessions")
	}

	// Delete and verify cascade.
	if err := store.DeleteSession(sess.ID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	_, err = store.GetSession(sess.ID)
	if err == nil {
		t.Error("GetSession should fail after delete")
	}
	msgs, err = store.GetMessages(sess.ID)
	if err != nil {
		t.Fatalf("GetMessages after delete: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages after cascade delete, got %d", len(msgs))
	}
}

// ---------------------------------------------------------------------------
// Invariant 2: Metrics logger round-trip
// Log an event, read it back, verify all fields match.
// ---------------------------------------------------------------------------

func TestInvariant_MetricsLoggerRoundTrip(t *testing.T) {
	dir := t.TempDir()

	logger, err := metrics.NewEventLogger(dir)
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}

	// Log an event with data.
	err = logger.Log("test.invariant", map[string]interface{}{
		"key":     "value",
		"count":   42,
		"nested":  true,
	})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}

	// Close to flush.
	if err := logger.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Read back.
	events, err := metrics.ReadEvents(dir)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	ev := events[0]

	// Verify event type.
	if ev.EventType != "test.invariant" {
		t.Errorf("EventType = %q, want test.invariant", ev.EventType)
	}

	// Verify timestamp is recent (within last 10 seconds).
	if time.Since(ev.Timestamp) > 10*time.Second {
		t.Errorf("Timestamp too old: %v", ev.Timestamp)
	}

	// Verify data fields.
	if ev.Data["key"] != "value" {
		t.Errorf("Data[key] = %v, want value", ev.Data["key"])
	}
	// JSON numbers decode as float64.
	if count, ok := ev.Data["count"].(float64); !ok || count != 42 {
		t.Errorf("Data[count] = %v, want 42", ev.Data["count"])
	}
	if nested, ok := ev.Data["nested"].(bool); !ok || !nested {
		t.Errorf("Data[nested] = %v, want true", ev.Data["nested"])
	}

	// Verify MarshalJSON round-trip produces valid JSON.
	buf, err := ev.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf, &parsed); err != nil {
		t.Fatalf("round-trip produced invalid JSON: %v", err)
	}
	if parsed["event"] != "test.invariant" {
		t.Errorf("round-trip event = %v, want test.invariant", parsed["event"])
	}
}

// ---------------------------------------------------------------------------
// Invariant 3: WebSocket protocol
// Connect, receive connection.ready, send chat.send, get chat.done.
// ---------------------------------------------------------------------------

func TestInvariant_WebSocketProtocol(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ws-invariant.db")
	store, err := session.Open(dbPath)
	if err != nil {
		t.Fatalf("session.Open: %v", err)
	}
	defer store.Close()

	hub := ws.NewHub(ws.WithSessionStore(store))
	handler := ws.NewMessageHandler(hub, store, nil)
	hub.SetHandler(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Create an HTTP test server with WS upgrade + health endpoint.
	srv := server.New(
		server.WithSessionStore(store),
		server.WithHub(hub),
		server.WithPort(0),
	)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Invariant 3a: Health endpoint returns {"status":"ok"}.
	resp, err := ts.Client().Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d, want 200", resp.StatusCode)
	}
	var health struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if health.Status != "ok" {
		t.Errorf("health.status = %q, want ok", health.Status)
	}

	// Create a session for the chat test.
	sess, err := store.CreateSession("WS Invariant")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Invariant 3b: WebSocket connect and protocol sequence.
	wsURL := "ws" + ts.URL[len("http"):] + "/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Must receive connection.ready first.
	readyMsg := readWSJSON(t, ctx, conn)
	if readyMsg["type"] != "connection.ready" {
		t.Fatalf("expected connection.ready, got %v", readyMsg["type"])
	}
	readyData, ok := readyMsg["data"].(map[string]interface{})
	if !ok {
		t.Fatal("connection.ready must have data object")
	}
	if readyData["clientId"] == nil || readyData["clientId"] == "" {
		t.Error("connection.ready must include clientId")
	}

	// Must receive session.list.
	sessionList := readWSJSON(t, ctx, conn)
	if sessionList["type"] != "session.list" {
		t.Fatalf("expected session.list, got %v", sessionList["type"])
	}

	// Invariant 3c: Send chat message and receive chat.done.
	chatMsg := `{"type":"chat.send","sessionId":"` + sess.ID + `","content":"invariant check"}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(chatMsg)); err != nil {
		t.Fatalf("write chat.send: %v", err)
	}

	doneMsg := readWSJSON(t, ctx, conn)
	if doneMsg["type"] != "chat.done" {
		t.Fatalf("expected chat.done, got %v", doneMsg["type"])
	}
	if doneMsg["sessionId"] != sess.ID {
		t.Errorf("chat.done sessionId = %v, want %s", doneMsg["sessionId"], sess.ID)
	}

	// Verify message was persisted.
	msgs, err := store.GetMessages(sess.ID)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content != "invariant check" {
		t.Errorf("Content = %q, want %q", msgs[0].Content, "invariant check")
	}
}

// readWSJSON reads a single JSON message from a WebSocket connection with timeout.
func readWSJSON(t *testing.T, ctx context.Context, conn *websocket.Conn) map[string]interface{} {
	t.Helper()
	rCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, data, err := conn.Read(rCtx)
	if err != nil {
		t.Fatalf("readWSJSON: %v", err)
	}

	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("readWSJSON unmarshal: %v\nraw: %s", err, string(data))
	}

	// Verify it has a type field (protocol invariant).
	if _, ok := msg["type"]; !ok {
		t.Fatalf("WS message missing 'type' field: %s", string(data))
	}

	// Verify type is a string.
	if _, ok := msg["type"].(string); !ok {
		t.Fatalf("WS message 'type' is not a string: %v", msg["type"])
	}

	return msg
}

