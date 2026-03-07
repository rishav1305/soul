package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"

	"github.com/rishav1305/soul/internal/ai"
)

// chatSession tracks the cancellable context for an active chat stream per connection.
type chatSession struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{} // closed when the current chat goroutine finishes
}

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

	cs := &chatSession{}

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
			// Cancel any previous stream and wait for it to finish.
			cs.mu.Lock()
			if cs.cancel != nil {
				cs.cancel()
			}
			prevDone := cs.done
			cs.mu.Unlock()
			if prevDone != nil {
				<-prevDone
			}

			chatCtx, cancel := context.WithCancel(ctx)
			done := make(chan struct{})
			cs.mu.Lock()
			cs.cancel = cancel
			cs.done = done
			cs.mu.Unlock()

			// Capture msg for goroutine (loop var reuse).
			chatMsg := msg
			go func() {
				defer close(done)
				s.handleChatSend(chatCtx, conn, &chatMsg)
				cs.mu.Lock()
				if cs.done == done {
					cs.cancel = nil
					cs.done = nil
				}
				cs.mu.Unlock()
			}()

		case "chat.stop":
			cs.mu.Lock()
			if cs.cancel != nil {
				log.Printf("[ws] stopping active chat stream")
				cs.cancel()
				cs.cancel = nil
			}
			cs.mu.Unlock()

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

	// Capture the full assistant response with incremental DB persistence.
	// Insert a placeholder row up front so even a crash preserves partial output.
	var fullResponse strings.Builder
	var assistantMsgID int64
	var flushMu sync.Mutex
	lastFlushLen := 0

	if dbSessionID > 0 && s.planner != nil {
		if id, err := s.planner.InsertMessage(dbSessionID, "assistant", ""); err == nil {
			assistantMsgID = id
		} else {
			log.Printf("[ws] failed to insert assistant placeholder: %v", err)
		}
	}

	// Periodic flush: update the DB row every 5 seconds with accumulated content.
	flushTicker := time.NewTicker(5 * time.Second)
	flushDone := make(chan struct{})
	go func() {
		defer flushTicker.Stop()
		for {
			select {
			case <-flushTicker.C:
				flushMu.Lock()
				content := fullResponse.String()
				shouldFlush := len(content) > lastFlushLen && assistantMsgID > 0 && s.planner != nil
				if shouldFlush {
					lastFlushLen = len(content)
				}
				flushMu.Unlock()
				if shouldFlush {
					_ = s.planner.UpdateMessageContent(assistantMsgID, content)
				}
			case <-flushDone:
				return
			}
		}
	}()

	originalSendEvent := sendEvent
	sendEvent = func(wsMsg WSMessage) {
		if wsMsg.Type == "chat.token" {
			flushMu.Lock()
			fullResponse.WriteString(wsMsg.Content)
			flushMu.Unlock()
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
	if s.processor != nil {
		agent.startTask = func(id int64) { s.processor.StartTask(id) }
		agent.processor = s.processor
	}
	agent.pm = s.pm
	agent.RunWithHistory(ctx, inMemorySessionID, msg.Content, opts.ChatType, opts.DisabledTools, opts.Thinking, opts.SkillContent, priorMessages, sendEvent)

	// Stop the periodic flusher and do a final persist.
	close(flushDone)
	if assistantMsgID > 0 && s.planner != nil {
		content := fullResponse.String()
		if content == "" {
			content = "[tool calls executed]"
		}
		if err := s.planner.UpdateMessageContent(assistantMsgID, content); err != nil {
			log.Printf("[ws] failed to persist final assistant message: %v", err)
		}
	}
	// Always reset status to idle.
	if dbSessionID > 0 && s.planner != nil {
		_ = s.planner.UpdateSessionStatus(dbSessionID, "idle")
	}

	// Generate smart title + summary in the background after the first complete exchange.
	if dbSessionID > 0 && s.planner != nil && s.ai != nil {
		go s.generateSessionSummary(dbSessionID, model, ctx, conn)
	}

	log.Printf("[ws] chat.send complete session=%s dbSession=%d", inMemorySessionID, dbSessionID)
}

// generateSessionSummary calls a lightweight AI model to create a smart title
// and summary for the given session, then broadcasts the update to all WS clients.
func (s *Server) generateSessionSummary(sessionID int64, model string, parentCtx context.Context, conn *websocket.Conn) {
	// Use a fresh context so this doesn't get cancelled when the parent stream ends.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	records, err := s.planner.GetSessionMessages(sessionID)
	if err != nil || len(records) < 2 {
		return // need at least 1 user + 1 assistant message
	}

	// Build a condensed transcript — first 3 + last 3 messages (if long conversation).
	var transcript strings.Builder
	msgs := records
	if len(msgs) > 6 {
		for _, m := range msgs[:3] {
			fmt.Fprintf(&transcript, "%s: %s\n", m.Role, truncate(m.Content, 300))
		}
		transcript.WriteString("...\n")
		for _, m := range msgs[len(msgs)-3:] {
			fmt.Fprintf(&transcript, "%s: %s\n", m.Role, truncate(m.Content, 300))
		}
	} else {
		for _, m := range msgs {
			fmt.Fprintf(&transcript, "%s: %s\n", m.Role, truncate(m.Content, 300))
		}
	}

	prompt := fmt.Sprintf(`Analyze this conversation and return ONLY a JSON object with two fields:
- "title": a concise 3-7 word title capturing the main topic (no quotes, no period)
- "summary": a 1-sentence summary of what was discussed or accomplished (max 120 chars)

Conversation:
%s

Return ONLY the JSON object, nothing else.`, transcript.String())

	// Use haiku for speed and cost efficiency.
	summaryModel := "claude-haiku-4-5-20251001"
	result, err := s.ai.CompleteSimple(ctx, summaryModel, prompt)
	if err != nil {
		log.Printf("[ws] summary generation failed for session %d: %v", sessionID, err)
		return
	}

	// Parse the JSON response.
	var parsed struct {
		Title   string `json:"title"`
		Summary string `json:"summary"`
	}

	// Try to extract JSON from the response (handle potential markdown wrapping).
	jsonStr := strings.TrimSpace(result)
	if idx := strings.Index(jsonStr, "{"); idx >= 0 {
		if end := strings.LastIndex(jsonStr, "}"); end > idx {
			jsonStr = jsonStr[idx : end+1]
		}
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		log.Printf("[ws] failed to parse summary JSON for session %d: %v (raw: %s)", sessionID, err, result)
		return
	}

	if parsed.Title == "" {
		return
	}

	// Determine model short name for display.
	modelShort := model
	if i := strings.LastIndex(model, "-"); i > 0 {
		// e.g. "claude-sonnet-4-6" → "sonnet-4"
		parts := strings.Split(model, "-")
		if len(parts) >= 3 {
			modelShort = parts[1]
		}
	}

	if err := s.planner.UpdateSessionSummary(sessionID, parsed.Title, parsed.Summary, modelShort); err != nil {
		log.Printf("[ws] failed to save summary for session %d: %v", sessionID, err)
		return
	}

	log.Printf("[ws] generated summary for session %d: %q / %q", sessionID, parsed.Title, parsed.Summary)

	// Broadcast session.updated so all connected clients refresh the drawer.
	data, _ := json.Marshal(map[string]any{
		"session_id": sessionID,
		"title":      parsed.Title,
		"summary":    parsed.Summary,
		"model":      modelShort,
	})
	s.broadcast(WSMessage{Type: "session.updated", Data: data})
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
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
