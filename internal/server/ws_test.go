package server_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rishav1305/soul/internal/config"
	"github.com/rishav1305/soul/internal/server"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// wsMessage mirrors the WSMessage struct in ws.go for test decoding.
type wsMessage struct {
	Type      string          `json:"type"`
	SessionID string          `json:"session_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

func TestWebSocketConnect(t *testing.T) {
	cfg := config.Default()
	srv := server.New(cfg)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect to the WebSocket endpoint.
	wsURL := "ws" + ts.URL[len("http"):] + "/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect to WebSocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	// Send a chat.send message.
	sendMsg := wsMessage{
		Type:      "chat.send",
		SessionID: "test-session-1",
		Content:   "Hello, Soul!",
	}
	if err := wsjson.Write(ctx, conn, sendMsg); err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// We should receive at least one response with a "type" field.
	var resp wsMessage
	if err := wsjson.Read(ctx, conn, &resp); err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if resp.Type == "" {
		t.Fatal("expected response to have a non-empty 'type' field")
	}

	// Read remaining messages until we get chat.done.
	for resp.Type != "chat.done" {
		if err := wsjson.Read(ctx, conn, &resp); err != nil {
			t.Fatalf("failed to read follow-up response: %v", err)
		}
		if resp.Type == "" {
			t.Fatal("expected response to have a non-empty 'type' field")
		}
	}

	if resp.Type != "chat.done" {
		t.Fatalf("expected final message type 'chat.done', got %q", resp.Type)
	}
}

func TestWebSocketInvalidMessage(t *testing.T) {
	cfg := config.Default()
	srv := server.New(cfg)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect to the WebSocket endpoint.
	wsURL := "ws" + ts.URL[len("http"):] + "/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect to WebSocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	// Send a message with an unknown type.
	sendMsg := wsMessage{
		Type:    "unknown.type",
		Content: "this should fail",
	}
	if err := wsjson.Write(ctx, conn, sendMsg); err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// Should receive an error response.
	var resp wsMessage
	if err := wsjson.Read(ctx, conn, &resp); err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if resp.Type != "error" {
		t.Fatalf("expected response type 'error', got %q", resp.Type)
	}

	if resp.Content == "" {
		t.Fatal("expected error response to have non-empty content")
	}
}
