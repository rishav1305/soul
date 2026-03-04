package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"

	"github.com/rishav1305/soul/internal/ai"
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
	SkillContent  string   `json:"skillContent"`
}

// handleChatSend processes a chat.send message by running the AI agent loop.
// It streams tokens, tool calls, and progress events back to the browser.
func (s *Server) handleChatSend(ctx context.Context, conn *websocket.Conn, msg *WSMessage) {
	log.Printf("[ws] chat.send session=%s content=%q", msg.SessionID, msg.Content)

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

	// If client didn't provide skill content, auto-load from chatType name.
	if opts.SkillContent == "" && s.skillStore != nil && opts.ChatType != "" {
		chatTypeLower := strings.ToLower(opts.ChatType)
		if content, ok := s.skillStore.Get(chatTypeLower); ok {
			opts.SkillContent = content
		}
	}

	log.Printf("[ws] chat options model=%s chatType=%s disabledTools=%v thinking=%v skillContent=%d bytes",
		model, opts.ChatType, opts.DisabledTools, opts.Thinking, len(opts.SkillContent))

	// Create a mutable sendEvent callback that writes to the WebSocket.
	// We use a var so we can wrap it later to capture the full response.
	var sendEvent func(WSMessage)
	sendEvent = func(wsMsg WSMessage) {
		if err := wsjson.Write(ctx, conn, wsMsg); err != nil {
			log.Printf("[ws] write error type=%s: %v", wsMsg.Type, err)
		}
	}

	// Resolve DB session ID — two-step: look up existing first, then create if needed.
	// "new" and "" are explicit sentinels meaning "start a fresh session".
	var dbSessionID int64
	if msg.SessionID != "" && msg.SessionID != "new" {
		if id, err := strconv.ParseInt(msg.SessionID, 10, 64); err == nil && id > 0 {
			// Client provided an existing DB session ID.
			dbSessionID = id
			if s.planner != nil {
				_ = s.planner.UpdateSessionStatus(dbSessionID, "running")
			}
		}
	}
	if dbSessionID == 0 && s.planner != nil {
		// New session — create it in the DB.
		title := msg.Content
		if len(title) > 100 {
			title = title[:100]
		}
		if sess, err := s.planner.CreateSession(title); err == nil {
			dbSessionID = sess.ID
			// Tell client the new DB session ID so it can resume later.
			idData, _ := json.Marshal(map[string]any{"session_id": dbSessionID})
			sendEvent(WSMessage{Type: "session.created", Data: idData})
		} else {
			log.Printf("[ws] failed to create DB session: %v", err)
		}
	}

	// Load prior history from DB to resume context.
	var priorMessages []ai.Message
	if dbSessionID > 0 && s.planner != nil {
		records, err := s.planner.GetSessionMessages(dbSessionID)
		if err != nil {
			log.Printf("[ws] failed to load session messages: %v", err)
		} else {
			for _, r := range records {
				priorMessages = append(priorMessages, ai.Message{Role: r.Role, Content: r.Content})
			}
			// Cap history at 50 messages to avoid exceeding context limits.
			if len(priorMessages) > 50 {
				priorMessages = priorMessages[len(priorMessages)-50:]
			}
			log.Printf("[ws] loaded %d prior messages for session %d", len(priorMessages), dbSessionID)
		}
	}

	// Persist user message to DB before running agent.
	if dbSessionID > 0 && s.planner != nil {
		if err := s.planner.AddMessage(dbSessionID, "user", msg.Content); err != nil {
			log.Printf("[ws] failed to persist user message: %v", err)
		}
	}

	// Wrap sendEvent to capture the full assistant response for DB persistence.
	var fullResponse strings.Builder
	originalSendEvent := sendEvent
	sendEvent = func(wsMsg WSMessage) {
		if wsMsg.Type == "chat.token" {
			fullResponse.WriteString(wsMsg.Content)
		}
		originalSendEvent(wsMsg)
	}

	// Determine the in-memory session ID to use.
	// Use "db-<id>" for DB-backed sessions so the in-memory store stays isolated.
	inMemorySessionID := msg.SessionID
	if inMemorySessionID == "" {
		inMemorySessionID = "default"
	}
	if dbSessionID > 0 {
		inMemorySessionID = fmt.Sprintf("db-%d", dbSessionID)
	}

	// Run the AI agent loop with prior history for context.
	agent := NewAgentLoop(s.ai, s.products, s.sessions, s.planner, s.broadcast, model, s.projectRoot)
	agent.RunWithHistory(ctx, inMemorySessionID, msg.Content, opts.ChatType, opts.DisabledTools, opts.Thinking, opts.SkillContent, priorMessages, sendEvent)

	// Persist assistant response to DB (always, even for tool-only turns).
	if dbSessionID > 0 && s.planner != nil {
		content := fullResponse.String()
		if content == "" {
			content = "[tool calls executed]"
		}
		if err := s.planner.AddMessage(dbSessionID, "assistant", content); err != nil {
			log.Printf("[ws] failed to persist assistant message: %v", err)
		}
	}
	// Always reset status to idle.
	if dbSessionID > 0 && s.planner != nil {
		_ = s.planner.UpdateSessionStatus(dbSessionID, "idle")
	}

	log.Printf("[ws] chat.send complete session=%s dbSession=%d", inMemorySessionID, dbSessionID)
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
