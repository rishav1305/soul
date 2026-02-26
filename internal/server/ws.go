package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// WSMessage is the envelope for all WebSocket messages.
type WSMessage struct {
	Type      string          `json:"type"`
	SessionID string          `json:"session_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// handleWebSocket upgrades the HTTP connection and processes messages.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		log.Printf("websocket accept: %v", err)
		return
	}
	defer conn.Close(websocket.StatusInternalError, "unexpected close")

	ctx := r.Context()

	for {
		var msg WSMessage
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			// Client disconnected or read error — exit cleanly.
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
				websocket.CloseStatus(err) == websocket.StatusGoingAway {
				return
			}
			log.Printf("websocket read: %v", err)
			return
		}

		switch msg.Type {
		case "chat.send":
			s.handleChatSend(ctx, conn, &msg)
		default:
			s.sendWSError(ctx, conn, fmt.Sprintf("unknown message type: %q", msg.Type))
		}
	}
}

// handleChatSend processes a chat.send message: stores the user message,
// then sends back a placeholder chat.token and chat.done response.
func (s *Server) handleChatSend(ctx context.Context, conn *websocket.Conn, msg *WSMessage) {
	sessionID := msg.SessionID
	if sessionID == "" {
		sessionID = "default"
	}

	sess := s.sessions.GetOrCreate(sessionID)
	sess.AddMessage("user", msg.Content)

	// Placeholder response — real AI streaming comes in Task 8.
	placeholder := fmt.Sprintf("Echo: %s", msg.Content)

	// Send a chat.token with the content.
	tokenMsg := WSMessage{
		Type:      "chat.token",
		SessionID: sessionID,
		Content:   placeholder,
	}
	if err := wsjson.Write(ctx, conn, tokenMsg); err != nil {
		log.Printf("websocket write chat.token: %v", err)
		return
	}

	// Send chat.done to signal completion.
	doneMsg := WSMessage{
		Type:      "chat.done",
		SessionID: sessionID,
	}
	if err := wsjson.Write(ctx, conn, doneMsg); err != nil {
		log.Printf("websocket write chat.done: %v", err)
		return
	}

	// Store the assistant response in the session.
	sess.AddMessage("assistant", placeholder)
}

// sendWSError sends an error message back to the client.
func (s *Server) sendWSError(ctx context.Context, conn *websocket.Conn, errMsg string) {
	resp := WSMessage{
		Type:    "error",
		Content: errMsg,
	}
	if err := wsjson.Write(ctx, conn, resp); err != nil {
		log.Printf("websocket write error: %v", err)
	}
}
