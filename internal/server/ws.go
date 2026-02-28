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
		log.Printf("[ws] accept error: %v", err)
		return
	}
	defer conn.Close(websocket.StatusInternalError, "unexpected close")

	log.Printf("[ws] client connected from %s", r.RemoteAddr)
	ctx := r.Context()

	// Register this client for broadcast messages.
	s.registerWSClient(conn, ctx)
	defer s.unregisterWSClient(conn)

	for {
		var msg WSMessage
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			// Client disconnected or read error — exit cleanly.
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
				websocket.CloseStatus(err) == websocket.StatusGoingAway {
				log.Printf("[ws] client disconnected (clean)")
				return
			}
			log.Printf("[ws] read error: %v", err)
			return
		}

		log.Printf("[ws] recv type=%s session=%s content_len=%d", msg.Type, msg.SessionID, len(msg.Content))

		switch msg.Type {
		case "chat.send", "chat.message":
			s.handleChatSend(ctx, conn, &msg)
		default:
			log.Printf("[ws] unknown message type: %q", msg.Type)
			s.sendWSError(ctx, conn, fmt.Sprintf("unknown message type: %q", msg.Type))
		}
	}
}

// chatOptions holds the per-message options the frontend sends in msg.Data.
type chatOptions struct {
	Model         string   `json:"model"`
	ChatType      string   `json:"chatType"`
	DisabledTools []string `json:"disabledTools"`
	Thinking      bool     `json:"thinking"`
}

// handleChatSend processes a chat.send message by running the AI agent loop.
// It streams tokens, tool calls, and progress events back to the browser.
func (s *Server) handleChatSend(ctx context.Context, conn *websocket.Conn, msg *WSMessage) {
	sessionID := msg.SessionID
	if sessionID == "" {
		sessionID = "default"
	}

	log.Printf("[ws] chat.send session=%s content=%q", sessionID, msg.Content)

	// Parse optional chat options from msg.Data.
	var opts chatOptions
	if len(msg.Data) > 0 {
		if err := json.Unmarshal(msg.Data, &opts); err != nil {
			log.Printf("[ws] failed to parse chat options: %v", err)
		}
	}

	// Use the model from options if provided, otherwise fall back to config default.
	model := s.cfg.Model
	if opts.Model != "" {
		model = opts.Model
	}

	log.Printf("[ws] chat options model=%s chatType=%s disabledTools=%v thinking=%v",
		model, opts.ChatType, opts.DisabledTools, opts.Thinking)

	// Create a sendEvent callback that writes to the WebSocket.
	sendEvent := func(wsMsg WSMessage) {
		if err := wsjson.Write(ctx, conn, wsMsg); err != nil {
			log.Printf("[ws] write error type=%s: %v", wsMsg.Type, err)
		}
	}

	// Run the AI agent loop.
	agent := NewAgentLoop(s.ai, s.products, s.sessions, s.planner, s.broadcast, model)
	agent.Run(ctx, sessionID, msg.Content, opts.ChatType, opts.DisabledTools, opts.Thinking, sendEvent)
	log.Printf("[ws] chat.send complete session=%s", sessionID)
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
