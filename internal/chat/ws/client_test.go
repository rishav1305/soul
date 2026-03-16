package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

func TestClientSubscribe(t *testing.T) {
	c := &Client{}
	if c.SessionID() != "" {
		t.Error("expected empty session ID initially")
	}

	c.Subscribe("session-123")
	if c.SessionID() != "session-123" {
		t.Errorf("expected session-123, got %s", c.SessionID())
	}

	c.Subscribe("session-456")
	if c.SessionID() != "session-456" {
		t.Errorf("expected session-456, got %s", c.SessionID())
	}
}

func TestClientSendChannelCapacity(t *testing.T) {
	c := &Client{
		id:   "test-client",
		send: make(chan []byte, sendChannelCap),
	}

	if cap(c.send) != 256 {
		t.Errorf("expected send channel capacity 256, got %d", cap(c.send))
	}

	// Fill the channel.
	for i := 0; i < sendChannelCap; i++ {
		c.send <- []byte("msg")
	}

	if len(c.send) != 256 {
		t.Errorf("expected send channel length 256, got %d", len(c.send))
	}
}

func TestClientSendClosesSlowClient(t *testing.T) {
	c := &Client{
		id:   "test-client",
		send: make(chan []byte, sendChannelCap),
	}

	// Fill the channel.
	for i := 0; i < sendChannelCap; i++ {
		c.send <- []byte{byte(i)}
	}

	// Send one more — channel is full, so Send should close and return false.
	ok := c.Send([]byte{255})
	if ok {
		t.Error("expected Send to return false when channel is full")
	}

	// Subsequent sends should also return false (sendDone is set).
	ok = c.Send([]byte{0})
	if ok {
		t.Error("expected Send to return false after slow-client close")
	}
}

func TestClientClose(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go h.Run(ctx)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.HandleUpgrade(w, r)
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):]
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = conn

	time.Sleep(100 * time.Millisecond)

	// Get the client from the hub via the safe Clients() method.
	clients := h.Clients()
	if len(clients) == 0 {
		t.Fatal("expected at least one client in the hub")
	}
	client := clients[0]

	// Close the client directly.
	client.Close()

	// Wait for cleanup — poll with backoff since ReadPump needs to detect
	// context cancel, exit, and send to unregister channel.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if h.ClientCount() == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("expected 0 clients after Close, got %d", h.ClientCount())
}

func TestClientID(t *testing.T) {
	c := &Client{id: "ws-test-001"}
	if c.ID() != "ws-test-001" {
		t.Errorf("expected ws-test-001, got %s", c.ID())
	}
}

func TestClientReadPumpRejectsBinary(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go h.Run(ctx)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.HandleUpgrade(w, r)
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):]
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Drain the connection.ready message (no session store, so no session.list).
	readCtx, readCancel := context.WithTimeout(ctx, 2*time.Second)
	_, _, _ = conn.Read(readCtx)
	readCancel()

	// Send a binary frame — should cause the server to close the connection.
	err = conn.Write(ctx, websocket.MessageBinary, []byte("binary data"))
	if err != nil {
		// Connection may already be closing, which is fine.
		return
	}

	// Try to read — should get a close error.
	time.Sleep(100 * time.Millisecond)
	_, _, err = conn.Read(ctx)
	if err == nil {
		t.Error("expected error after sending binary frame")
	}
}

func TestClientWritePump(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go h.Run(ctx)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.HandleUpgrade(w, r)
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):]
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	time.Sleep(50 * time.Millisecond)

	// Read the connection.ready message (no session store, so only one initial message).
	readCtx1, readCancel1 := context.WithTimeout(ctx, 2*time.Second)
	_, _, _ = conn.Read(readCtx1)
	readCancel1()

	// Get the client via the safe Clients() method and send a message.
	clients := h.Clients()
	if len(clients) == 0 {
		t.Fatal("expected at least one client in the hub")
	}
	client := clients[0]

	// Send a message via the client's send channel.
	testMsg := []byte(`{"type":"test","data":"hello"}`)
	client.Send(testMsg)

	// Read the message from the WebSocket connection.
	readCtx, readCancel := context.WithTimeout(ctx, 2*time.Second)
	defer readCancel()
	typ, data, err := conn.Read(readCtx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if typ != websocket.MessageText {
		t.Errorf("expected text message, got %v", typ)
	}
	if string(data) != string(testMsg) {
		t.Errorf("expected %q, got %q", testMsg, data)
	}
}

func TestClientReadPumpAcceptsText(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go h.Run(ctx)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.HandleUpgrade(w, r)
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):]
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	time.Sleep(50 * time.Millisecond)

	// Send a text message — should not cause disconnection.
	err = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"ping"}`))
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	// Give the read pump time to process.
	time.Sleep(50 * time.Millisecond)

	// Client should still be connected.
	if count := h.ClientCount(); count != 1 {
		t.Errorf("expected 1 client still connected, got %d", count)
	}
}

func TestClientContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Client{
		id:     "test-ctx",
		ctx:    ctx,
		cancel: cancel,
		send:   make(chan []byte, sendChannelCap),
	}

	// Context should be live.
	select {
	case <-c.Context().Done():
		t.Fatal("context should not be done yet")
	default:
	}

	// Cancel and verify context is done.
	cancel()
	select {
	case <-c.Context().Done():
		// expected
	case <-time.After(time.Second):
		t.Fatal("context should be done after cancel")
	}
}

func TestClientSendAfterClose(t *testing.T) {
	c := &Client{
		id:   "test-closed",
		send: make(chan []byte, sendChannelCap),
	}

	// Normal send should succeed.
	if ok := c.Send([]byte("hello")); !ok {
		t.Error("expected Send to return true on open channel")
	}

	// Close the send channel.
	c.closeSend()

	// Send after close should return false, not panic.
	if ok := c.Send([]byte("world")); ok {
		t.Error("expected Send to return false after closeSend")
	}

	// Double close should not panic.
	c.closeSend()
}

func TestMarshalBatch_SingleMessage_PlainObject(t *testing.T) {
	msgs := [][]byte{[]byte(`{"type":"chat.token"}`)}
	result, err := marshalBatch(msgs)
	if err != nil {
		t.Fatal(err)
	}
	// Single message: no array wrapper.
	if result[0] == '[' {
		t.Errorf("single message should not be wrapped in array, got: %s", result)
	}
}

func TestMarshalBatch_MultipleMessages_ArrayFrame(t *testing.T) {
	msgs := [][]byte{
		[]byte(`{"type":"chat.token"}`),
		[]byte(`{"type":"chat.token"}`),
	}
	result, err := marshalBatch(msgs)
	if err != nil {
		t.Fatal(err)
	}
	if result[0] != '[' {
		t.Errorf("multiple messages should be wrapped in array, got: %s", result)
	}
}

func TestClient_SlowClientQueueFull_SetsCloseReason(t *testing.T) {
	// Create a minimal client without a real connection.
	c := &Client{
		id:   "test-client",
		send: make(chan []byte, sendChannelCap),
	}

	// Fill the channel to capacity.
	for i := 0; i < sendChannelCap; i++ {
		c.send <- []byte(`{"type":"test"}`)
	}

	// Next send should trigger slow-client close and set closeReason.
	ok := c.Send([]byte(`{"type":"overflow"}`))
	if ok {
		t.Fatal("expected Send to return false for full channel")
	}
	// closeReason must be non-nil and equal to "slow_client_queue_full".
	v := c.closeReason.Load()
	if v == nil {
		t.Fatal("closeReason is nil, expected slow_client_queue_full")
	}
	if reason := v.(string); reason != "slow_client_queue_full" {
		t.Errorf("closeReason = %q, want %q", reason, "slow_client_queue_full")
	}
}
