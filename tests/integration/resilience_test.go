package integration_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// connectWS connects to the test server's WebSocket endpoint.
func connectWS(t *testing.T, url, token string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, url+"?token="+token, nil)
	if err != nil {
		t.Fatalf("ws connect: %v", err)
	}
	return conn
}

// readUntilType reads WS messages until one with the given type is found.
func readUntilType(t *testing.T, conn *websocket.Conn, msgType string, timeout time.Duration) map[string]interface{} {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		var msg map[string]interface{}
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			t.Fatalf("reading for %s: %v", msgType, err)
		}
		if msg["type"] == msgType {
			return msg
		}
	}
}

// sendJSONMap sends a JSON map over WebSocket to a live server.
func sendJSONMap(t *testing.T, conn *websocket.Conn, msg interface{}) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := wsjson.Write(ctx, conn, msg); err != nil {
		t.Fatalf("ws write: %v", err)
	}
}

func TestResilience_MidStreamDisconnect(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	baseURL := "ws://localhost:3002/ws"
	token := os.Getenv("SOUL_V2_TOKEN")
	if token == "" {
		t.Skip("SOUL_V2_TOKEN not set")
	}

	conn := connectWS(t, baseURL, token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	readUntilType(t, conn, "connection.ready", 5*time.Second)

	sendJSONMap(t, conn, map[string]string{"type": "session.create"})
	created := readUntilType(t, conn, "session.created", 5*time.Second)
	createdData := created["data"].(map[string]interface{})
	session := createdData["session"].(map[string]interface{})
	sessionID := session["id"].(string)

	sendJSONMap(t, conn, map[string]interface{}{
		"type":      "chat.send",
		"sessionId": sessionID,
		"content":   "Count from 1 to 10",
	})

	readUntilType(t, conn, "chat.token", 10*time.Second)

	// Abruptly close mid-stream
	conn.Close(websocket.StatusGoingAway, "test disconnect")

	// Reconnect
	conn2 := connectWS(t, baseURL, token)
	defer conn2.Close(websocket.StatusNormalClosure, "")

	readUntilType(t, conn2, "connection.ready", 5*time.Second)

	sendJSONMap(t, conn2, map[string]interface{}{
		"type":      "session.switch",
		"sessionId": sessionID,
	})

	history := readUntilType(t, conn2, "session.history", 5*time.Second)
	historyData := history["data"].(map[string]interface{})
	messages := historyData["messages"].([]interface{})
	if len(messages) == 0 {
		t.Error("expected messages after mid-stream disconnect")
	}
}

func TestResilience_DisconnectStorm(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	baseURL := "ws://localhost:3002/ws"
	token := os.Getenv("SOUL_V2_TOKEN")
	if token == "" {
		t.Skip("SOUL_V2_TOKEN not set")
	}

	// Rapid connect/disconnect 20 times
	for i := 0; i < 20; i++ {
		conn := connectWS(t, baseURL, token)
		conn.Close(websocket.StatusNormalClosure, "")
	}

	// Server should still accept connections
	conn := connectWS(t, baseURL, token)
	defer conn.Close(websocket.StatusNormalClosure, "")
	readUntilType(t, conn, "connection.ready", 5*time.Second)
}

func TestResilience_AuthFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, resp, err := websocket.Dial(ctx, "ws://localhost:3002/ws?token=invalid-token", nil)
	if err == nil {
		t.Fatal("expected connection to fail with invalid token")
	}
	// nhooyr.io/websocket returns the HTTP response on failure
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}
