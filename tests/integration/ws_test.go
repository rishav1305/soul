package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"nhooyr.io/websocket"

	"github.com/rishav1305/soul-v2/internal/chat/session"
	"github.com/rishav1305/soul-v2/internal/chat/ws"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// wsReadBuf holds messages buffered from array-batched WS frames.
// Key: *websocket.Conn — safe because each test uses its own connection.
var (
	wsReadBufMu  sync.Mutex
	wsReadBufMap = make(map[*websocket.Conn][]map[string]interface{})
)

// setupWSEnv creates a Hub with a session store and MessageHandler, starts the
// hub event loop, and returns everything needed for integration testing.
func setupWSEnv(t *testing.T) (*ws.Hub, *session.Store, context.CancelFunc) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "ws-integration.db")
	store, err := session.Open(dbPath)
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	hub := ws.NewHub(ws.WithSessionStore(store))
	handler := ws.NewMessageHandler(hub, store, nil)
	hub.SetHandler(handler)

	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	return hub, store, cancel
}

// wsServer creates an httptest server that upgrades HTTP to WebSocket via the hub.
func wsServer(t *testing.T, hub *ws.Hub) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleUpgrade(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// dial opens a WebSocket connection to the test server.
func dial(t *testing.T, ctx context.Context, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + srv.URL[len("http"):]
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return conn
}

// readJSON reads one JSON message from the connection with a timeout.
// The WS writePump coalesces rapid messages into JSON array frames
// ([msg1,msg2,...]). readJSON transparently handles this: when an array frame
// arrives, it returns the first element and buffers the rest so subsequent
// calls return them in order without an additional Read.
func readJSON(t *testing.T, ctx context.Context, conn *websocket.Conn) map[string]interface{} {
	t.Helper()

	// Return buffered messages from a previous array frame first.
	wsReadBufMu.Lock()
	if buf := wsReadBufMap[conn]; len(buf) > 0 {
		m := buf[0]
		wsReadBufMap[conn] = buf[1:]
		wsReadBufMu.Unlock()
		return m
	}
	wsReadBufMu.Unlock()

	rCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, data, err := conn.Read(rCtx)
	if err != nil {
		t.Fatalf("readJSON: %v", err)
	}

	// Detect JSON array frame (WS batch coalescing).
	if len(data) > 0 && data[0] == '[' {
		var arr []map[string]interface{}
		if err := json.Unmarshal(data, &arr); err != nil {
			t.Fatalf("readJSON unmarshal array: %v", err)
		}
		if len(arr) == 0 {
			t.Fatal("readJSON: empty batch array from server")
		}
		if len(arr) > 1 {
			wsReadBufMu.Lock()
			wsReadBufMap[conn] = append(wsReadBufMap[conn], arr[1:]...)
			wsReadBufMu.Unlock()
		}
		return arr[0]
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("readJSON unmarshal: %v", err)
	}
	return m
}

// drainInitial drains connection.ready + session.list that every new client receives.
func drainInitial(t *testing.T, ctx context.Context, conn *websocket.Conn) {
	t.Helper()
	msg1 := readJSON(t, ctx, conn)
	if msg1["type"] != "connection.ready" {
		t.Fatalf("expected connection.ready, got %v", msg1["type"])
	}
	msg2 := readJSON(t, ctx, conn)
	if msg2["type"] != "session.list" {
		t.Fatalf("expected session.list, got %v", msg2["type"])
	}
}

// waitForClientCount polls the hub until the expected count is reached or the
// deadline expires.
func waitForClientCount(t *testing.T, hub *ws.Hub, want int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if hub.ClientCount() == want {
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Errorf("client count: want %d, got %d (after %v)", want, hub.ClientCount(), timeout)
}

// waitForGoroutines waits until runtime.NumGoroutine() drops to the target
// (or below), allowing time for goroutines to drain after a test.
func waitForGoroutines(t *testing.T, target int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= target {
			return
		}
		runtime.GC()
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("goroutine leak: expected <= %d, got %d", target, runtime.NumGoroutine())
}

// sendJSON writes a JSON text frame to the connection.
func sendJSON(t *testing.T, ctx context.Context, conn *websocket.Conn, msg string) {
	t.Helper()
	if err := conn.Write(ctx, websocket.MessageText, []byte(msg)); err != nil {
		t.Fatalf("sendJSON: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Test 1: Full connect -> send message -> receive tokens -> done cycle
// ---------------------------------------------------------------------------

func TestWS_FullChatCycle(t *testing.T) {
	hub, store, cancel := setupWSEnv(t)
	defer cancel()

	sess, err := store.CreateSession("Chat Cycle")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.Background()
	srv := wsServer(t, hub)
	conn := dial(t, ctx, srv)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// 1. Receive connection.ready.
	ready := readJSON(t, ctx, conn)
	if ready["type"] != "connection.ready" {
		t.Fatalf("expected connection.ready, got %v", ready["type"])
	}
	data := ready["data"].(map[string]interface{})
	if data["clientId"] == nil || data["clientId"] == "" {
		t.Fatal("missing clientId in connection.ready")
	}

	// 2. Receive session.list.
	sList := readJSON(t, ctx, conn)
	if sList["type"] != "session.list" {
		t.Fatalf("expected session.list, got %v", sList["type"])
	}

	// 3. Send chat.send.
	sendJSON(t, ctx, conn, `{"type":"chat.send","sessionId":"`+sess.ID+`","content":"integration test"}`)

	// 4. Receive chat.done — collect 4 messages (3 session.updated + 1 chat.done).
	// The no-Claude path broadcasts session.updated for auto-title, idle→running,
	// and running→completed. These are async hub broadcasts and may interleave
	// with the direct-sent chat.done in any order.
	const chatSendMsgs = 4
	var done map[string]interface{}
	for i := 0; i < chatSendMsgs; i++ {
		msg := readJSON(t, ctx, conn)
		switch msg["type"] {
		case "chat.done":
			done = msg
		case "session.updated":
			// expected async broadcasts — discard
		default:
			t.Fatalf("unexpected message type after chat.send: %v", msg["type"])
		}
	}
	if done == nil {
		t.Fatal("chat.done not received among the 4 messages after chat.send")
	}
	if done["sessionId"] != sess.ID {
		t.Errorf("expected sessionId %s, got %v", sess.ID, done["sessionId"])
	}

	// 5. Verify message was persisted.
	msgs, err := store.GetMessages(sess.ID)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 stored message, got %d", len(msgs))
	}
	if msgs[0].Content != "integration test" {
		t.Errorf("stored content = %q, want %q", msgs[0].Content, "integration test")
	}
}

// ---------------------------------------------------------------------------
// Test 2: Session switch cancels client context (placeholder for stream cancel)
// ---------------------------------------------------------------------------

func TestWS_SessionSwitchUpdatesContext(t *testing.T) {
	hub, store, cancel := setupWSEnv(t)
	defer cancel()

	sess1, err := store.CreateSession("Session A")
	if err != nil {
		t.Fatalf("create session A: %v", err)
	}
	sess2, err := store.CreateSession("Session B")
	if err != nil {
		t.Fatalf("create session B: %v", err)
	}

	ctx := context.Background()
	srv := wsServer(t, hub)
	conn := dial(t, ctx, srv)
	defer conn.Close(websocket.StatusNormalClosure, "")

	drainInitial(t, ctx, conn)

	// Switch to session A.
	sendJSON(t, ctx, conn, `{"type":"session.switch","sessionId":"`+sess1.ID+`"}`)
	_ = readJSON(t, ctx, conn) // session.updated
	_ = readJSON(t, ctx, conn) // session.list
	_ = readJSON(t, ctx, conn) // session.history

	// Verify subscription.
	clients := hub.Clients()
	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}
	if clients[0].SessionID() != sess1.ID {
		t.Errorf("client subscribed to %s, want %s", clients[0].SessionID(), sess1.ID)
	}

	// Switch to session B — subscription updates atomically.
	sendJSON(t, ctx, conn, `{"type":"session.switch","sessionId":"`+sess2.ID+`"}`)
	_ = readJSON(t, ctx, conn) // session.updated
	_ = readJSON(t, ctx, conn) // session.list
	_ = readJSON(t, ctx, conn) // session.history

	clients = hub.Clients()
	if clients[0].SessionID() != sess2.ID {
		t.Errorf("client subscribed to %s after switch, want %s", clients[0].SessionID(), sess2.ID)
	}
}

// ---------------------------------------------------------------------------
// Test 3: Multiple clients on same session receive broadcast
// ---------------------------------------------------------------------------

func TestWS_BroadcastToSession_MultipleClients(t *testing.T) {
	hub, store, cancel := setupWSEnv(t)
	defer cancel()

	sess, err := store.CreateSession("Shared Session")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.Background()
	srv := wsServer(t, hub)

	// Connect two clients.
	conn1 := dial(t, ctx, srv)
	defer conn1.Close(websocket.StatusNormalClosure, "")
	conn2 := dial(t, ctx, srv)
	defer conn2.Close(websocket.StatusNormalClosure, "")

	drainInitial(t, ctx, conn1)
	drainInitial(t, ctx, conn2)

	// Subscribe both to the same session.
	sendJSON(t, ctx, conn1, `{"type":"session.switch","sessionId":"`+sess.ID+`"}`)
	_ = readJSON(t, ctx, conn1) // session.updated
	_ = readJSON(t, ctx, conn1) // session.list
	_ = readJSON(t, ctx, conn1) // session.history

	sendJSON(t, ctx, conn2, `{"type":"session.switch","sessionId":"`+sess.ID+`"}`)
	_ = readJSON(t, ctx, conn2) // session.updated
	_ = readJSON(t, ctx, conn2) // session.list
	_ = readJSON(t, ctx, conn2) // session.history

	// Broadcast a session-scoped message from the hub.
	testMsg := `{"type":"test.session.broadcast","sessionId":"` + sess.ID + `","data":"shared"}`
	hub.BroadcastToSession(sess.ID, []byte(testMsg))

	msg1 := readJSON(t, ctx, conn1)
	if msg1["type"] != "test.session.broadcast" {
		t.Errorf("conn1: expected test.session.broadcast, got %v", msg1["type"])
	}
	msg2 := readJSON(t, ctx, conn2)
	if msg2["type"] != "test.session.broadcast" {
		t.Errorf("conn2: expected test.session.broadcast, got %v", msg2["type"])
	}
}

// ---------------------------------------------------------------------------
// Test 4: Client disconnect triggers context cancellation
// ---------------------------------------------------------------------------

func TestWS_DisconnectCancelsContext(t *testing.T) {
	hub, _, cancel := setupWSEnv(t)
	defer cancel()

	ctx := context.Background()
	srv := wsServer(t, hub)
	conn := dial(t, ctx, srv)

	drainInitial(t, ctx, conn)

	// Grab the server-side client and its context.
	clients := hub.Clients()
	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}
	clientCtx := clients[0].Context()

	// Verify context is still active.
	select {
	case <-clientCtx.Done():
		t.Fatal("client context should NOT be cancelled before disconnect")
	default:
	}

	// Disconnect the WebSocket.
	conn.Close(websocket.StatusNormalClosure, "bye")

	// Context should be cancelled within 2 seconds.
	select {
	case <-clientCtx.Done():
		// Success — context was cancelled.
	case <-time.After(2 * time.Second):
		t.Fatal("client context was not cancelled within 2s after disconnect")
	}

	// Hub should also have 0 clients.
	waitForClientCount(t, hub, 0, 2*time.Second)
}

// ---------------------------------------------------------------------------
// Test 5: Session create via WebSocket returns session object
// ---------------------------------------------------------------------------

func TestWS_SessionCreateReturnsObject(t *testing.T) {
	hub, _, cancel := setupWSEnv(t)
	defer cancel()

	ctx := context.Background()
	srv := wsServer(t, hub)

	conn1 := dial(t, ctx, srv)
	defer conn1.Close(websocket.StatusNormalClosure, "")
	conn2 := dial(t, ctx, srv)
	defer conn2.Close(websocket.StatusNormalClosure, "")

	drainInitial(t, ctx, conn1)
	drainInitial(t, ctx, conn2)

	// Send session.create from client 1.
	sendJSON(t, ctx, conn1, `{"type":"session.create"}`)

	// Both clients should receive session.created.
	msg1 := readJSON(t, ctx, conn1)
	if msg1["type"] != "session.created" {
		t.Errorf("conn1: expected session.created, got %v", msg1["type"])
	}

	// Verify session object structure.
	data1, ok := msg1["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be object, got %T", msg1["data"])
	}
	sessObj, ok := data1["session"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data.session to be object, got %T", data1["session"])
	}
	if sessObj["id"] == nil || sessObj["id"] == "" {
		t.Error("expected non-empty session.id")
	}
	if sessObj["title"] == nil {
		t.Error("expected session.title to be present")
	}

	msg2 := readJSON(t, ctx, conn2)
	if msg2["type"] != "session.created" {
		t.Errorf("conn2: expected session.created, got %v", msg2["type"])
	}
}

// ---------------------------------------------------------------------------
// Test 6: Session delete sends deleted event to all clients
// ---------------------------------------------------------------------------

func TestWS_SessionDeleteBroadcasts(t *testing.T) {
	hub, store, cancel := setupWSEnv(t)
	defer cancel()

	sess, err := store.CreateSession("To Delete")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.Background()
	srv := wsServer(t, hub)

	conn1 := dial(t, ctx, srv)
	defer conn1.Close(websocket.StatusNormalClosure, "")
	conn2 := dial(t, ctx, srv)
	defer conn2.Close(websocket.StatusNormalClosure, "")

	drainInitial(t, ctx, conn1)
	drainInitial(t, ctx, conn2)

	// Delete from client 1.
	sendJSON(t, ctx, conn1, `{"type":"session.delete","sessionId":"`+sess.ID+`"}`)

	msg1 := readJSON(t, ctx, conn1)
	if msg1["type"] != "session.deleted" {
		t.Errorf("conn1: expected session.deleted, got %v", msg1["type"])
	}
	if msg1["sessionId"] != sess.ID {
		t.Errorf("conn1: sessionId = %v, want %s", msg1["sessionId"], sess.ID)
	}

	msg2 := readJSON(t, ctx, conn2)
	if msg2["type"] != "session.deleted" {
		t.Errorf("conn2: expected session.deleted, got %v", msg2["type"])
	}
	if msg2["sessionId"] != sess.ID {
		t.Errorf("conn2: sessionId = %v, want %s", msg2["sessionId"], sess.ID)
	}

	// Verify session is actually gone from the store.
	_, err = store.GetSession(sess.ID)
	if err == nil {
		t.Error("expected error for deleted session")
	}
}

// ---------------------------------------------------------------------------
// Test 7: Concurrent connect/disconnect stress test (100 clients)
// ---------------------------------------------------------------------------

func TestWS_StressTest_100Clients(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	hub, _, cancel := setupWSEnv(t)
	defer cancel()

	srv := wsServer(t, hub)

	const numClients = 100
	var wg sync.WaitGroup
	wg.Add(numClients)

	// Track panics.
	panics := make(chan interface{}, numClients)

	for i := 0; i < numClients; i++ {
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panics <- r
				}
			}()

			ctx, ctxCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer ctxCancel()

			conn := dial(t, ctx, srv)
			// Read at least the connection.ready message.
			rCtx, rCancel := context.WithTimeout(ctx, 3*time.Second)
			_, _, _ = conn.Read(rCtx)
			rCancel()

			// Small random-ish work: send a text message.
			_ = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"session.create"}`))

			// Brief pause then disconnect.
			time.Sleep(10 * time.Millisecond)
			conn.Close(websocket.StatusNormalClosure, "done")
		}()
	}

	wg.Wait()
	close(panics)

	// Check for panics.
	for p := range panics {
		t.Errorf("goroutine panicked: %v", p)
	}

	// Hub should return to 0 clients.
	waitForClientCount(t, hub, 0, 5*time.Second)
}

// ---------------------------------------------------------------------------
// Test 8: Server shutdown sends close frames to all clients
// ---------------------------------------------------------------------------

func TestWS_ServerShutdownClosesClients(t *testing.T) {
	hub, _, cancel := setupWSEnv(t)

	ctx := context.Background()
	srv := wsServer(t, hub)

	// Connect 3 clients.
	conns := make([]*websocket.Conn, 3)
	for i := range conns {
		conns[i] = dial(t, ctx, srv)
		drainInitial(t, ctx, conns[i])
	}

	waitForClientCount(t, hub, 3, 2*time.Second)

	// Cancel the hub context (server shutdown).
	cancel()

	// All clients should receive an error or close frame when reading.
	var wg sync.WaitGroup
	wg.Add(len(conns))
	for _, c := range conns {
		go func(conn *websocket.Conn) {
			defer wg.Done()
			// Read will eventually fail with a close or error.
			rCtx, rCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer rCancel()
			for {
				_, _, err := conn.Read(rCtx)
				if err != nil {
					return // Got the expected close/error.
				}
			}
		}(c)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All clients detected the close.
	case <-time.After(5 * time.Second):
		t.Fatal("not all clients received close frames within 5s")
	}

	// Cleanup: close our side of the connections.
	for _, c := range conns {
		c.Close(websocket.StatusNormalClosure, "")
	}
}

// ---------------------------------------------------------------------------
// Test 9: Goroutine leak detection
// ---------------------------------------------------------------------------

func TestWS_GoroutineLeak(t *testing.T) {
	// Baseline goroutine count.
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	// Run a full lifecycle.
	hub, store, cancel := setupWSEnv(t)

	sess, err := store.CreateSession("Goroutine Test")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.Background()
	srv := wsServer(t, hub)

	// Connect 5 clients, do work, disconnect.
	conns := make([]*websocket.Conn, 5)
	for i := range conns {
		conns[i] = dial(t, ctx, srv)
		drainInitial(t, ctx, conns[i])

		// Send a chat message.
		sendJSON(t, ctx, conns[i], `{"type":"chat.send","sessionId":"`+sess.ID+`","content":"leak test"}`)
		_ = readJSON(t, ctx, conns[i]) // chat.done
	}

	// Disconnect all clients.
	for _, c := range conns {
		c.Close(websocket.StatusNormalClosure, "")
	}

	waitForClientCount(t, hub, 0, 3*time.Second)

	// Cancel hub.
	cancel()

	// Goroutine count should return close to baseline.
	// Allow a generous margin (+5) for runtime goroutines (GC, finalizers, etc.).
	waitForGoroutines(t, baseline+5, 5*time.Second)
}

// ---------------------------------------------------------------------------
// Test 10: Full round-trip with real session store + message handler
// ---------------------------------------------------------------------------

func TestWS_FullRoundTrip(t *testing.T) {
	hub, store, cancel := setupWSEnv(t)
	defer cancel()

	ctx := context.Background()
	srv := wsServer(t, hub)
	conn := dial(t, ctx, srv)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// 1. connection.ready
	ready := readJSON(t, ctx, conn)
	if ready["type"] != "connection.ready" {
		t.Fatalf("expected connection.ready, got %v", ready["type"])
	}

	// 2. session.list (initially empty)
	sList := readJSON(t, ctx, conn)
	if sList["type"] != "session.list" {
		t.Fatalf("expected session.list, got %v", sList["type"])
	}

	// 3. Create session via WebSocket.
	sendJSON(t, ctx, conn, `{"type":"session.create"}`)
	created := readJSON(t, ctx, conn)
	if created["type"] != "session.created" {
		t.Fatalf("expected session.created, got %v", created["type"])
	}

	// Extract session ID from the response.
	createdData := created["data"].(map[string]interface{})
	sessObj := createdData["session"].(map[string]interface{})
	sessionID := sessObj["id"].(string)
	if sessionID == "" {
		t.Fatal("empty session ID in session.created")
	}

	// 4. Switch to the new session.
	sendJSON(t, ctx, conn, `{"type":"session.switch","sessionId":"`+sessionID+`"}`)

	updated := readJSON(t, ctx, conn) // session.updated
	if updated["type"] != "session.updated" {
		t.Fatalf("expected session.updated, got %v", updated["type"])
	}

	sList2 := readJSON(t, ctx, conn) // session.list
	if sList2["type"] != "session.list" {
		t.Fatalf("expected session.list, got %v", sList2["type"])
	}

	history := readJSON(t, ctx, conn) // session.history
	if history["type"] != "session.history" {
		t.Fatalf("expected session.history, got %v", history["type"])
	}

	// 5. Send a chat message.
	sendJSON(t, ctx, conn, `{"type":"chat.send","sessionId":"`+sessionID+`","content":"round trip message"}`)

	// The no-Claude chat.send path for a new session (MessageCount==0, Status==Idle)
	// emits exactly 4 messages — in unpredictable order due to the async hub
	// broadcast (direct-send is faster than broadcastCh delivery):
	//   - session.updated × 3 (auto-title, idle→running, running→completed)
	//   - chat.done × 1 (direct send)
	// Read all 4 and verify chat.done is among them.
	const chatSendMsgCount = 4
	var chatDone map[string]interface{}
	for i := 0; i < chatSendMsgCount; i++ {
		msg := readJSON(t, ctx, conn)
		switch msg["type"] {
		case "chat.done":
			chatDone = msg
		case "session.updated":
			// expected — discard
		default:
			t.Fatalf("unexpected message type after chat.send: %v", msg["type"])
		}
	}
	if chatDone == nil {
		t.Fatal("expected chat.done among the 4 messages after chat.send")
	}
	if chatDone["sessionId"] != sessionID {
		t.Errorf("chat.done sessionId = %v, want %s", chatDone["sessionId"], sessionID)
	}

	// 6. Verify message was stored.
	msgs, err := store.GetMessages(sessionID)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content != "round trip message" {
		t.Errorf("message content = %q, want %q", msgs[0].Content, "round trip message")
	}
	if msgs[0].Role != "user" {
		t.Errorf("message role = %q, want %q", msgs[0].Role, "user")
	}

	// 7. Delete the session via WebSocket.
	sendJSON(t, ctx, conn, `{"type":"session.delete","sessionId":"`+sessionID+`"}`)

	deleted := readJSON(t, ctx, conn)
	if deleted["type"] != "session.deleted" {
		t.Fatalf("expected session.deleted, got %v", deleted["type"])
	}
	if deleted["sessionId"] != sessionID {
		t.Errorf("session.deleted sessionId = %v, want %s", deleted["sessionId"], sessionID)
	}

	// 8. Verify session is gone.
	_, err = store.GetSession(sessionID)
	if err == nil {
		t.Error("expected error when getting deleted session")
	}

	// 9. Verify messages are also gone (CASCADE).
	msgs, err = store.GetMessages(sessionID)
	if err != nil {
		t.Fatalf("get messages after delete: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages after session delete, got %d", len(msgs))
	}
}

// ---------------------------------------------------------------------------
// Test: Disconnect handling — ReadPump cleanup sequence
// ---------------------------------------------------------------------------

func TestWS_DisconnectHandling_CleanupSequence(t *testing.T) {
	hub, _, cancel := setupWSEnv(t)
	defer cancel()

	ctx := context.Background()
	srv := wsServer(t, hub)
	conn := dial(t, ctx, srv)

	drainInitial(t, ctx, conn)
	waitForClientCount(t, hub, 1, 2*time.Second)

	clients := hub.Clients()
	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}
	client := clients[0]
	clientCtx := client.Context()

	// Verify pre-conditions.
	if clientCtx.Err() != nil {
		t.Fatal("client context should not be cancelled before disconnect")
	}

	// Disconnect.
	conn.Close(websocket.StatusNormalClosure, "testing cleanup")

	// 1. Hub unregisters the client.
	waitForClientCount(t, hub, 0, 2*time.Second)

	// 2. Client context is cancelled (would cancel any agent).
	select {
	case <-clientCtx.Done():
		// Expected.
	case <-time.After(2 * time.Second):
		t.Fatal("client context not cancelled within 2s of disconnect")
	}
}

// ---------------------------------------------------------------------------
// Test: Server-side Close() triggers full cleanup
// ---------------------------------------------------------------------------

func TestWS_ServerSideClose(t *testing.T) {
	hub, _, cancel := setupWSEnv(t)
	defer cancel()

	ctx := context.Background()
	srv := wsServer(t, hub)
	conn := dial(t, ctx, srv)

	drainInitial(t, ctx, conn)

	clients := hub.Clients()
	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}
	client := clients[0]
	clientCtx := client.Context()

	// Server-side close.
	client.Close()

	// Context should be cancelled.
	select {
	case <-clientCtx.Done():
		// Good.
	case <-time.After(2 * time.Second):
		t.Fatal("context not cancelled after server-side Close()")
	}

	// Hub should unregister.
	waitForClientCount(t, hub, 0, 2*time.Second)

	// Client-side read should fail.
	rCtx, rCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer rCancel()
	_, _, err := conn.Read(rCtx)
	if err == nil {
		t.Error("expected read error after server-side Close()")
	}
}
