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

func TestClientSendDropsOldest(t *testing.T) {
	c := &Client{
		id:   "test-client",
		send: make(chan []byte, sendChannelCap),
	}

	// Fill the channel with numbered messages.
	for i := 0; i < sendChannelCap; i++ {
		c.send <- []byte{byte(i)}
	}

	// Send one more — should drop oldest (0) and add new.
	c.Send([]byte{255})

	// The first message should now be 1 (0 was dropped).
	first := <-c.send
	if first[0] != 1 {
		t.Errorf("expected first message to be 1 (after dropping 0), got %d", first[0])
	}

	// Verify the last message is 255.
	// Drain 254 messages (indices 2..255 + the new 255 at the end).
	var last []byte
	for i := 0; i < sendChannelCap-1; i++ {
		last = <-c.send
	}
	if last[0] != 255 {
		t.Errorf("expected last message to be 255, got %d", last[0])
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

	time.Sleep(50 * time.Millisecond)

	// Get the client from the hub via the safe Clients() method.
	clients := h.Clients()
	if len(clients) == 0 {
		t.Fatal("expected at least one client in the hub")
	}
	client := clients[0]

	// Close the client directly.
	client.Close()

	// Wait for cleanup.
	time.Sleep(100 * time.Millisecond)
	if count := h.ClientCount(); count != 0 {
		t.Errorf("expected 0 clients after Close, got %d", count)
	}
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
