package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	prodctx "github.com/rishav1305/soul-v2/internal/chat/context"
	"github.com/rishav1305/soul-v2/internal/chat/metrics"
	"github.com/rishav1305/soul-v2/internal/chat/session"
	"github.com/rishav1305/soul-v2/internal/chat/stream"
)

// maxToolRounds is the maximum number of tool-use/result round-trips per message.
const maxToolRounds = 5

// agentEntry tracks a running stream agent for a single session.
type agentEntry struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// chatSession tracks all running agents for a single WebSocket client.
type chatSession struct {
	mu     sync.Mutex
	agents map[string]agentEntry // keyed by session ID
}

// MessageHandler processes inbound WebSocket messages and dispatches them
// to the appropriate session and hub operations. It is safe for concurrent
// use from multiple ReadPump goroutines because it only reads its own fields
// (hub, sessionStore, streamClient, metrics, dispatcher) and delegates state
// mutations to the hub event loop or the thread-safe session store.
type MessageHandler struct {
	hub          *Hub
	sessionStore session.StoreInterface
	streamClient *stream.Client
	metrics      *metrics.EventLogger
	dispatcher   *prodctx.Dispatcher
	builtin      *BuiltinExecutor

	sessionsMu  sync.Mutex
	sessions    map[*Client]*chatSession
	seenMessages map[string]time.Time // messageId → first seen time
	seenMu       sync.Mutex
}

// NewMessageHandler creates a new MessageHandler with the given dependencies.
// The streamClient parameter may be nil — if so, chat.send will store the user
// message and immediately return chat.done without streaming (Phase 3 behavior).
func NewMessageHandler(hub *Hub, store session.StoreInterface, mel *metrics.EventLogger, opts ...MessageHandlerOption) *MessageHandler {
	h := &MessageHandler{
		hub:          hub,
		sessionStore: store,
		metrics:      mel,
		sessions:     make(map[*Client]*chatSession),
		seenMessages: make(map[string]time.Time),
	}
	for _, opt := range opts {
		opt(h)
	}
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			h.seenMu.Lock()
			cutoff := time.Now().Add(-5 * time.Minute)
			for id, ts := range h.seenMessages {
				if ts.Before(cutoff) {
					delete(h.seenMessages, id)
				}
			}
			h.seenMu.Unlock()
		}
	}()
	return h
}

// MessageHandlerOption configures a MessageHandler.
type MessageHandlerOption func(*MessageHandler)

// WithStreamClient sets the Claude API streaming client on the handler.
func WithStreamClient(sc *stream.Client) MessageHandlerOption {
	return func(h *MessageHandler) {
		h.streamClient = sc
	}
}

// WithDispatcher sets the product tool call dispatcher on the handler.
func WithDispatcher(d *prodctx.Dispatcher) MessageHandlerOption {
	return func(h *MessageHandler) {
		h.dispatcher = d
	}
}

// WithBuiltinExecutor sets the built-in tool executor on the handler.
func WithBuiltinExecutor(be *BuiltinExecutor) MessageHandlerOption {
	return func(h *MessageHandler) {
		h.builtin = be
	}
}

// HandleMessage parses a raw inbound message and routes it to the correct
// handler method. Invalid JSON or unknown types are handled gracefully
// without crashing.
func (h *MessageHandler) HandleMessage(client *Client, raw []byte) {
	msg, err := ParseInboundMessage(raw)
	if err != nil {
		h.sendError(client, "", err.Error())
		return
	}

	switch msg.Type {
	case TypeChatSend:
		h.handleChatSend(client, msg)
	case TypeChatStop:
		h.handleChatStop(client, msg)
	case TypeSessionSwitch:
		h.handleSessionSwitch(client, msg)
	case TypeSessionCreate:
		h.handleSessionCreate(client, msg)
	case TypeSessionDelete:
		h.handleSessionDelete(client, msg)
	case TypeSessionRename:
		h.handleSessionRename(client, msg)
	case TypeSessionSetProduct:
		h.handleSessionSetProduct(client, msg)
	case TypeSessionResume:
		h.handleSessionResume(client, msg)
	default:
		log.Printf("ws: unknown message type %q from client %s", msg.Type, client.ID())
	}
}

// handleSessionSetProduct processes a session.setProduct message. It persists
// the product on the session and broadcasts the update to all clients.
func (h *MessageHandler) handleSessionSetProduct(client *Client, msg *InboundMessage) {
	if msg.SessionID == "" {
		h.sendError(client, "", "session ID required")
		return
	}
	if !IsValidUUID(msg.SessionID) {
		h.sendError(client, "", "invalid session ID")
		return
	}

	if err := h.sessionStore.SetProduct(msg.SessionID, msg.Product); err != nil {
		log.Printf("ws: failed to set product for session %s: %v", msg.SessionID, err)
		h.sendError(client, msg.SessionID, "failed to set product")
		return
	}

	h.broadcast(NewSessionProductSet(msg.SessionID, msg.Product))

	if h.metrics != nil {
		_ = h.metrics.Log("session.setProduct", map[string]interface{}{
			"session_id": msg.SessionID,
			"product":    msg.Product,
			"client_id":  client.ID(),
		})
	}
}

// handleChatSend processes a chat.send message. It validates the session exists,
// stores the user message, builds conversation history, and streams the response
// from the Claude API back to the client as chat.token events.
func (h *MessageHandler) handleChatSend(client *Client, msg *InboundMessage) {
	if msg.MessageID != "" {
		h.seenMu.Lock()
		if _, seen := h.seenMessages[msg.MessageID]; seen {
			h.seenMu.Unlock()
			log.Printf("ws: dedup — skipping already-seen message %s", msg.MessageID)
			return
		}
		h.seenMessages[msg.MessageID] = time.Now()
		h.seenMu.Unlock()
	}

	if msg.SessionID == "" {
		h.sendError(client, "", "session ID required")
		return
	}
	if !IsValidUUID(msg.SessionID) {
		h.sendError(client, "", "invalid session ID")
		return
	}

	content, contentErr := ValidateChatContent(msg.Content)
	if contentErr != nil {
		h.sendError(client, msg.SessionID, contentErr.Error())
		return
	}
	msg.Content = content

	// Validate session exists.
	_, err := h.sessionStore.GetSession(msg.SessionID)
	if err != nil {
		h.sendError(client, msg.SessionID, "session not found")
		return
	}

	// Auto-title from first user message (if session has no messages yet).
	sess, _ := h.sessionStore.GetSession(msg.SessionID)
	if sess != nil && sess.MessageCount == 0 {
		title := msg.Content
		if len(title) > 50 {
			title = title[:50]
			if i := strings.LastIndex(title, " "); i > 25 {
				title = title[:i]
			}
			title += "..."
		}
		if updated, err := h.sessionStore.UpdateSessionTitle(msg.SessionID, title); err == nil {
			h.broadcast(NewSessionUpdated(updated))
		}
	}

	// Transition session to running.
	if sess != nil && sess.Status == session.StatusIdle {
		if err := h.sessionStore.UpdateSessionStatus(msg.SessionID, session.StatusRunning); err == nil {
			if updated, err := h.sessionStore.GetSession(msg.SessionID); err == nil {
				h.broadcast(NewSessionUpdated(updated))
			}
		}
	}

	// Store the user message.
	stored, err := h.sessionStore.AddMessage(msg.SessionID, "user", msg.Content)
	if err != nil {
		log.Printf("ws: failed to store message: %v", err)
		h.sendError(client, msg.SessionID, "failed to store message")
		return
	}

	// User is active in this session — reset unread count.
	if err := h.sessionStore.ResetUnreadCount(msg.SessionID); err != nil {
		log.Printf("ws: failed to reset unread count for session %s: %v", msg.SessionID, err)
	}

	// If no stream client is configured, fall back to immediate chat.done.
	if h.streamClient == nil {
		h.completeSession(client, msg.SessionID)
		out := NewChatDone(msg.SessionID, stored.ID)
		h.sendToClient(client, out)
		return
	}

	// Build conversation history from session messages.
	sessionMsgs, err := h.sessionStore.GetMessages(msg.SessionID)
	if err != nil {
		log.Printf("ws: failed to get session messages: %v", err)
		h.sendError(client, msg.SessionID, "failed to load conversation history")
		return
	}

	apiMessages := buildAPIMessages(sessionMsgs)

	// Append image attachments to the last user message (current message).
	if len(msg.Attachments) > 0 && len(apiMessages) > 0 {
		last := &apiMessages[len(apiMessages)-1]
		if last.Role == "user" {
			for _, att := range msg.Attachments {
				if strings.HasPrefix(att.MediaType, "image/") && len(att.Data) > 0 {
					last.Content = append(last.Content, stream.ContentBlock{
						Type: "image",
						Source: &stream.ImageSource{
							Type:      "base64",
							MediaType: att.MediaType,
							Data:      att.Data,
						},
					})
				}
			}
		}
	}

	req := &stream.Request{
		MaxTokens: 4096,
		Messages:  apiMessages,
		Model:     msg.Model,
	}

	if msg.Thinking != nil && msg.Thinking.Type != "" && msg.Thinking.Type != "disabled" {
		req.Thinking = &stream.ThinkingParam{
			Type:         msg.Thinking.Type,
			BudgetTokens: msg.Thinking.BudgetTokens,
		}
		if msg.Thinking.Type == "enabled" && msg.Thinking.BudgetTokens > 0 {
			needed := msg.Thinking.BudgetTokens + 1024
			if needed > req.MaxTokens {
				req.MaxTokens = needed
			}
		}
		if msg.Thinking.Type == "adaptive" && req.MaxTokens < 64000 {
			req.MaxTokens = 64000
		}
	}

	// Inject product context (or default context for general mode).
	product := ""
	if sess != nil {
		product = sess.Product
	}
	pctx := prodctx.ForProduct(product)
	req.System = pctx.System
	if len(pctx.Tools) > 0 {
		req.Tools = pctx.Tools
	}

	// Generate a message ID for the assistant response ahead of time.
	// We'll use the stored user message ID as a reference, and create
	// the assistant message after streaming completes.
	sessionID := msg.SessionID

	// Get or create the per-client chat session tracker.
	cs := h.getOrCreateChatSession(client)

	cs.mu.Lock()
	// Cancel any existing agent for this session before starting a new one.
	if existing, ok := cs.agents[sessionID]; ok {
		existing.cancel()
		cs.mu.Unlock()
		<-existing.done
		cs.mu.Lock()
	}

	// Create a new context from Background — NOT from client.Context().
	// This allows multiple sessions to run concurrently on the same connection
	// and prevents a session switch from cancelling an in-flight stream.
	agentCtx, agentCancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	cs.agents[sessionID] = agentEntry{cancel: agentCancel, done: done}
	cs.mu.Unlock()

	// Stream in a goroutine so we don't block the ReadPump.
	go func() {
		defer func() {
			close(done)
			cs.mu.Lock()
			// Only delete if this is still our entry (not replaced by a newer one).
			if entry, ok := cs.agents[sessionID]; ok && entry.done == done {
				delete(cs.agents, sessionID)
			}
			cs.mu.Unlock()
		}()
		h.runStream(client, sessionID, req, agentCtx)
	}()
}

// buildAPIMessages converts stored session messages into Claude API messages,
// handling text, tool_use, and tool_result roles correctly.
func buildAPIMessages(sessionMsgs []*session.Message) []stream.Message {
	apiMessages := make([]stream.Message, 0, len(sessionMsgs))
	for _, sm := range sessionMsgs {
		switch sm.Role {
		case "user":
			apiMessages = append(apiMessages, stream.Message{
				Role: "user",
				Content: []stream.ContentBlock{
					{Type: "text", Text: sm.Content},
				},
			})
		case "assistant":
			apiMessages = append(apiMessages, stream.Message{
				Role: "assistant",
				Content: []stream.ContentBlock{
					{Type: "text", Text: sm.Content},
				},
			})
		case "tool_use":
			// Content is a JSON array of ContentBlock structs (tool_use blocks).
			var blocks []stream.ContentBlock
			if err := json.Unmarshal([]byte(sm.Content), &blocks); err != nil {
				log.Printf("ws: failed to unmarshal tool_use content: %v", err)
				continue
			}
			apiMessages = append(apiMessages, stream.Message{
				Role:    "assistant",
				Content: blocks,
			})
		case "tool_result":
			// Content is a JSON object: {"tool_use_id":"...","content":"..."}
			var tr struct {
				ToolUseID string `json:"tool_use_id"`
				Content   string `json:"content"`
			}
			if err := json.Unmarshal([]byte(sm.Content), &tr); err != nil {
				log.Printf("ws: failed to unmarshal tool_result content: %v", err)
				continue
			}
			apiMessages = append(apiMessages, stream.Message{
				Role: "user",
				Content: []stream.ContentBlock{
					{
						Type:      "tool_result",
						ToolUseID: tr.ToolUseID,
						Content:   tr.Content,
					},
				},
			})
		}
	}
	// Fix orphaned tool_use: if a tool_use message (assistant) is not followed
	// by tool_result messages (user), add synthetic tool_results so Claude doesn't
	// reject the conversation. This happens when a stream is cancelled mid-tool-loop.
	apiMessages = fixOrphanedToolUse(apiMessages)

	return normalizeAPIMessages(apiMessages)
}

// fixOrphanedToolUse ensures every tool_use block has a corresponding tool_result.
func fixOrphanedToolUse(messages []stream.Message) []stream.Message {
	fixed := make([]stream.Message, 0, len(messages))
	for i, msg := range messages {
		fixed = append(fixed, msg)

		// Check if this is an assistant message with tool_use blocks
		if msg.Role != "assistant" {
			continue
		}
		var toolUseIDs []string
		for _, cb := range msg.Content {
			if cb.Type == "tool_use" {
				toolUseIDs = append(toolUseIDs, cb.ID)
			}
		}
		if len(toolUseIDs) == 0 {
			continue
		}

		// Check if the next message(s) provide tool_results for all tool_use IDs
		provided := make(map[string]bool)
		for j := i + 1; j < len(messages); j++ {
			if messages[j].Role != "user" {
				break
			}
			for _, cb := range messages[j].Content {
				if cb.Type == "tool_result" {
					provided[cb.ToolUseID] = true
				}
			}
		}

		// Add synthetic tool_results for any missing IDs
		var missing []stream.ContentBlock
		for _, id := range toolUseIDs {
			if !provided[id] {
				missing = append(missing, stream.ContentBlock{
					Type:      "tool_result",
					ToolUseID: id,
					Content:   "[tool execution was interrupted]",
				})
			}
		}
		if len(missing) > 0 {
			fixed = append(fixed, stream.Message{
				Role:    "user",
				Content: missing,
			})
		}
	}
	return fixed
}

// normalizeAPIMessages merges consecutive messages with the same role into one
// message by appending content blocks in order. This keeps Claude payloads
// valid when a turn ends with a user message (for example after a stream error).
func normalizeAPIMessages(messages []stream.Message) []stream.Message {
	if len(messages) == 0 {
		return messages
	}

	normalized := make([]stream.Message, 0, len(messages))
	for _, msg := range messages {
		if len(normalized) == 0 || normalized[len(normalized)-1].Role != msg.Role {
			cloned := stream.Message{
				Role: msg.Role,
			}
			if len(msg.Content) > 0 {
				cloned.Content = append([]stream.ContentBlock(nil), msg.Content...)
			}
			normalized = append(normalized, cloned)
			continue
		}

		last := &normalized[len(normalized)-1]
		last.Content = append(last.Content, msg.Content...)
	}

	return normalized
}

// toolCall holds accumulated data for a single tool_use block during streaming.
type toolCall struct {
	ID    string
	Name  string
	Input strings.Builder
}

// runStream executes the Claude API streaming call and forwards events to the client.
// It handles tool-use loops: if Claude responds with tool_use blocks, it dispatches
// them to product servers and sends the results back for up to maxToolRounds.
func (h *MessageHandler) runStream(client *Client, sessionID string, req *stream.Request, agentCtx context.Context) {
	startTime := time.Now()

	// Use the provided agent context with a 5-minute deadline.
	// The agent context is derived from context.Background(), not client.Context(),
	// so multiple sessions can run concurrently on the same connection.
	ctx, cancel := context.WithTimeout(agentCtx, 5*time.Minute)
	defer cancel()

	// Log stream start.
	if h.metrics != nil {
		_ = h.metrics.Log(metrics.EventWSStreamStart, map[string]interface{}{
			"session_id": sessionID,
			"client_id":  client.ID(),
		})
	}

	var messageID string
	var model string
	var totalInputTokens int
	var totalOutputTokens int
	var firstTokenLogged bool

	// Tool loop: stream, check for tool_use, dispatch, repeat.
	for round := 0; round <= maxToolRounds; round++ {
		roundStart := time.Now()
		log.Printf("ws: stream round %d for session %s (model=%s, system=%d chars, messages=%d, tools=%d, thinking=%v)",
			round, sessionID, req.Model, len(req.System), len(req.Messages), len(req.Tools),
			req.Thinking != nil)

		ch, err := h.streamClient.Stream(ctx, req)
		if err != nil {
			log.Printf("ws: stream error after %v for session %s round %d: %v",
				time.Since(roundStart).Round(time.Millisecond), sessionID, round, err)
			var authErr *stream.AuthError
			if errors.As(err, &authErr) {
				log.Printf("ws: AUTH FAILURE for session %s: %v (check OAuth beta header and token expiry)", sessionID, err)
			} else {
				log.Printf("ws: stream error for session %s: %v", sessionID, err)
			}
			h.logAPIError(sessionID, err)
			h.sendClassifiedError(client, sessionID, err)
			return
		}

		var fullText strings.Builder
		var toolCalls []toolCall
		var currentToolIdx int = -1
		var stopReason string
		var gotMessageStop bool

		for evt := range ch {
			switch evt.Type {
			case "message_start":
				if evt.Message != nil {
					messageID = evt.Message.ID
					model = evt.Message.Model
					if evt.Message.Usage != nil {
						totalInputTokens += evt.Message.Usage.InputTokens
					}
				}

			case "content_block_start":
				if evt.ContentBlock != nil && evt.ContentBlock.Type == "tool_use" {
					tc := toolCall{
						ID:   evt.ContentBlock.ID,
						Name: evt.ContentBlock.Name,
					}
					toolCalls = append(toolCalls, tc)
					currentToolIdx = len(toolCalls) - 1

					// Send tool.call WS event to client.
					h.sendToClient(client, NewToolCall(sessionID, tc.ID, tc.Name, nil))
				}

			case "content_block_delta":
				if evt.Delta != nil {
					if evt.Delta.Text != "" {
						token := evt.Delta.Text
						fullText.WriteString(token)
						tokenMsg := NewChatToken(sessionID, token, messageID)
						h.sendToClient(client, tokenMsg)

						// Log first token only (per spec: ws.stream.token).
						if !firstTokenLogged {
							firstTokenLogged = true
							log.Printf("ws: first token for session %s after %v (round %d)",
								sessionID, time.Since(startTime).Round(time.Millisecond), round)
							if h.metrics != nil {
								_ = h.metrics.Log(metrics.EventWSStreamToken, map[string]interface{}{
									"session_id":     sessionID,
									"client_id":      client.ID(),
									"message_id":     messageID,
									"first_token_ms": time.Since(startTime).Milliseconds(),
								})
							}
						}
					}
					if evt.Delta.PartialJSON != "" && currentToolIdx >= 0 {
						toolCalls[currentToolIdx].Input.WriteString(evt.Delta.PartialJSON)
					}
				}

			case "content_block_stop":
				// Nothing special needed; currentToolIdx stays valid for the
				// next content_block_start to overwrite it.

			case "message_delta":
				if evt.Usage != nil {
					totalOutputTokens += evt.Usage.OutputTokens
				}
				if evt.StopReason != "" {
					stopReason = evt.StopReason
				}

			case "error":
				errMsg := "stream error"
				statusCode := 0
				if evt.Error != nil {
					errMsg = evt.Error.Message
					statusCode = evt.Error.StatusCode
				}
				log.Printf("ws: stream error event for session %s: %s", sessionID, errMsg)
				if h.metrics != nil {
					_ = h.metrics.Log(metrics.EventAPIError, map[string]interface{}{
						"session_id":    sessionID,
						"error_type":    "stream",
						"status_code":   statusCode,
						"error_message": errMsg,
					})
				}
				h.sendError(client, sessionID, "stream interrupted — please try again")
				return

			case "message_stop":
				gotMessageStop = true
			}
		}

		// If stream ended without message_stop, it was truncated.
		if !gotMessageStop {
			log.Printf("ws: stream ended without message_stop for session %s", sessionID)
			if h.metrics != nil {
				_ = h.metrics.Log(metrics.EventAPIError, map[string]interface{}{
					"session_id":    sessionID,
					"error_type":    "incomplete_stream",
					"status_code":   0,
					"error_message": "stream ended without message_stop",
				})
			}

			// Persist partial response so the user keeps what they received.
			if fullText.Len() > 0 {
				partial := fullText.String() + "\n\n[incomplete — stream ended unexpectedly]"
				if stored, err := h.sessionStore.AddMessage(sessionID, "assistant", partial); err != nil {
					log.Printf("ws: failed to store partial message for session %s: %v", sessionID, err)
				} else if messageID == "" {
					messageID = stored.ID
				}
			}

			h.completeSession(client, sessionID)
			doneMsg := NewChatDone(sessionID, messageID)
			h.sendToClient(client, doneMsg)
			return
		}

		// If stop reason is tool_use, handle tool dispatch loop.
		if stopReason == "tool_use" && len(toolCalls) > 0 && (h.dispatcher != nil || h.builtin != nil) {
			// Build and store the assistant tool_use message.
			toolBlocks := make([]stream.ContentBlock, 0, len(toolCalls))

			// Include any text that preceded the tool calls.
			if fullText.Len() > 0 {
				toolBlocks = append(toolBlocks, stream.ContentBlock{
					Type: "text",
					Text: fullText.String(),
				})
			}

			for i := range toolCalls {
				tc := &toolCalls[i]
				inputJSON := json.RawMessage(tc.Input.String())
				if len(inputJSON) == 0 {
					inputJSON = json.RawMessage("{}")
				}
				toolBlocks = append(toolBlocks, stream.ContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: inputJSON,
				})
			}

			// Store as tool_use message (JSON array of content blocks).
			toolUseJSON, err := json.Marshal(toolBlocks)
			if err != nil {
				log.Printf("ws: failed to marshal tool_use blocks: %v", err)
				h.sendError(client, sessionID, "internal error during tool processing")
				return
			}
			if _, err := h.sessionStore.AddMessage(sessionID, "tool_use", string(toolUseJSON)); err != nil {
				log.Printf("ws: failed to store tool_use message: %v", err)
			}

			// Dispatch each tool call and collect results.
			toolResultMessages := make([]stream.Message, 0, len(toolCalls))
			for i := range toolCalls {
				tc := &toolCalls[i]
				inputJSON := json.RawMessage(tc.Input.String())
				if len(inputJSON) == 0 {
					inputJSON = json.RawMessage("{}")
				}

				var result string
				var execErr error
				if h.builtin != nil && h.builtin.CanHandle(tc.Name) {
					result, execErr = h.builtin.Execute(ctx, tc.Name, inputJSON)
				} else if h.dispatcher != nil {
					result, execErr = h.dispatcher.Execute(ctx, tc.Name, inputJSON)
				} else {
					execErr = fmt.Errorf("no handler for tool: %s", tc.Name)
				}
				if execErr != nil {
					result = fmt.Sprintf("Error: %v", execErr)
				}

				// Store tool_result message.
				trJSON, _ := json.Marshal(struct {
					ToolUseID string `json:"tool_use_id"`
					Content   string `json:"content"`
				}{
					ToolUseID: tc.ID,
					Content:   result,
				})
				if _, err := h.sessionStore.AddMessage(sessionID, "tool_result", string(trJSON)); err != nil {
					log.Printf("ws: failed to store tool_result message: %v", err)
				}

				// Send tool.complete WS event.
				h.sendToClient(client, NewToolComplete(sessionID, tc.ID, tc.Name, result))

				// Build the user message with tool_result for the next API call.
				toolResultMessages = append(toolResultMessages, stream.Message{
					Role: "user",
					Content: []stream.ContentBlock{
						{
							Type:      "tool_result",
							ToolUseID: tc.ID,
							Content:   result,
						},
					},
				})
			}

			// Build the follow-up request: original messages + assistant tool_use + tool results.
			followUpMessages := make([]stream.Message, len(req.Messages))
			copy(followUpMessages, req.Messages)

			// Append the assistant message with tool blocks.
			followUpMessages = append(followUpMessages, stream.Message{
				Role:    "assistant",
				Content: toolBlocks,
			})

			// Append each tool result as a user message.
			// Claude API expects all tool_results in a single user message.
			var allToolResultBlocks []stream.ContentBlock
			for _, trMsg := range toolResultMessages {
				allToolResultBlocks = append(allToolResultBlocks, trMsg.Content...)
			}
			followUpMessages = append(followUpMessages, stream.Message{
				Role:    "user",
				Content: allToolResultBlocks,
			})

			// Update request for next round.
			req = &stream.Request{
				MaxTokens:      req.MaxTokens,
				Messages:       followUpMessages,
				Model:          req.Model,
				System:         req.System,
				Tools:          req.Tools,
				Thinking:       req.Thinking,
				SkipValidation: true,
			}

			continue // next round of the tool loop
		}

		// Normal text response — store and finish.
		if fullText.Len() > 0 {
			assistantMsg, err := h.sessionStore.AddMessage(sessionID, "assistant", fullText.String())
			if err != nil {
				log.Printf("ws: failed to store assistant message: %v", err)
			} else if messageID == "" {
				messageID = assistantMsg.ID
			}
		}

		// Transition session to completed.
		h.completeSession(client, sessionID)

		// Send chat.done.
		doneMsg := NewChatDone(sessionID, messageID)
		h.sendToClient(client, doneMsg)

		// Log stream end + API request metrics.
		if h.metrics != nil {
			duration := time.Since(startTime).Milliseconds()
			_ = h.metrics.Log(metrics.EventWSStreamEnd, map[string]interface{}{
				"session_id":   sessionID,
				"client_id":    client.ID(),
				"message_id":   messageID,
				"total_tokens": totalOutputTokens,
				"duration_ms":  duration,
			})
			_ = h.metrics.Log(metrics.EventAPIRequest, map[string]interface{}{
				"session_id":    sessionID,
				"model":         model,
				"input_tokens":  totalInputTokens,
				"output_tokens": totalOutputTokens,
				"duration_ms":   duration,
			})
		}
		return
	}

	// Exceeded maxToolRounds — store what we have and finish.
	log.Printf("ws: exceeded max tool rounds (%d) for session %s", maxToolRounds, sessionID)
	h.completeSession(client, sessionID)
	doneMsg := NewChatDone(sessionID, messageID)
	h.sendToClient(client, doneMsg)
}

// handleSessionSwitch processes a session.switch message. It validates the
// session exists, subscribes the client, and sends the session list with history.
func (h *MessageHandler) handleSessionSwitch(client *Client, msg *InboundMessage) {
	if msg.SessionID == "" {
		h.sendError(client, "", "session ID required")
		return
	}
	if !IsValidUUID(msg.SessionID) {
		h.sendError(client, "", "invalid session ID")
		return
	}

	// Validate session exists.
	sess, err := h.sessionStore.GetSession(msg.SessionID)
	if err != nil {
		h.sendError(client, msg.SessionID, "session not found")
		return
	}

	// Subscribe client to the session.
	client.Subscribe(msg.SessionID)

	// Reset unread count for the session being switched to.
	if err := h.sessionStore.ResetUnreadCount(msg.SessionID); err != nil {
		log.Printf("ws: failed to reset unread count for session %s: %v", msg.SessionID, err)
	}

	// Mark session as read if it was completed_unread.
	if sess.Status == session.StatusCompletedUnread {
		if err := h.sessionStore.UpdateSessionStatus(msg.SessionID, session.StatusIdle); err == nil {
			sess, _ = h.sessionStore.GetSession(msg.SessionID)
		}
	} else if sess.Status == session.StatusCompleted {
		if err := h.sessionStore.UpdateSessionStatus(msg.SessionID, session.StatusIdle); err == nil {
			sess, _ = h.sessionStore.GetSession(msg.SessionID)
		}
	}

	// Send session.updated with the full session object.
	updated := NewSessionUpdated(sess)
	h.sendToClient(client, updated)

	// Send full session list (excluding empty sessions) to the client.
	allSessions, err := h.sessionStore.ListSessions()
	if err != nil {
		log.Printf("ws: failed to list sessions: %v", err)
		h.sendError(client, msg.SessionID, "failed to list sessions")
		return
	}
	nonEmpty := make([]*session.Session, 0, len(allSessions))
	for _, s := range allSessions {
		if s.MessageCount > 0 {
			nonEmpty = append(nonEmpty, s)
		}
	}
	listMsg := NewSessionList(nonEmpty)
	h.sendToClient(client, listMsg)

	// Send message history for the switched-to session.
	messages, err := h.sessionStore.GetMessages(msg.SessionID)
	if err != nil {
		log.Printf("ws: failed to get messages for session %s: %v", msg.SessionID, err)
		h.sendError(client, msg.SessionID, "failed to load messages")
		return
	}
	historyMsg := NewSessionHistory(msg.SessionID, messages)
	h.sendToClient(client, historyMsg)
}

// handleSessionCreate processes a session.create message. It creates a new
// session and broadcasts the result to all connected clients.
func (h *MessageHandler) handleSessionCreate(client *Client, msg *InboundMessage) {
	title := ValidateSessionTitle(msg.Content)
	sess, err := h.sessionStore.CreateSession(title)
	if err != nil {
		log.Printf("ws: failed to create session: %v", err)
		h.sendError(client, "", "failed to create session")
		return
	}

	out := NewSessionCreated(sess)
	h.broadcast(out)
}

// handleSessionDelete processes a session.delete message. It validates the
// session exists, deletes it, and broadcasts the deletion to all clients.
func (h *MessageHandler) handleSessionDelete(client *Client, msg *InboundMessage) {
	if msg.SessionID == "" {
		h.sendError(client, "", "session ID required")
		return
	}
	if !IsValidUUID(msg.SessionID) {
		h.sendError(client, "", "invalid session ID")
		return
	}

	if err := h.sessionStore.DeleteSession(msg.SessionID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.sendError(client, msg.SessionID, "session not found")
		} else {
			log.Printf("ws: failed to delete session %s: %v", msg.SessionID, err)
			h.sendError(client, msg.SessionID, "failed to delete session")
		}
		return
	}

	out := NewSessionDeleted(msg.SessionID)
	h.broadcast(out)
}

// handleSessionRename processes a session.rename message. It validates the
// session ID and title, updates the session title, and broadcasts the update.
func (h *MessageHandler) handleSessionRename(client *Client, msg *InboundMessage) {
	if msg.SessionID == "" {
		h.sendError(client, "", "session ID required")
		return
	}
	if !IsValidUUID(msg.SessionID) {
		h.sendError(client, "", "invalid session ID")
		return
	}

	title := ValidateSessionTitle(msg.Content)
	if title == "" {
		h.sendError(client, msg.SessionID, "title cannot be empty")
		return
	}

	updated, err := h.sessionStore.UpdateSessionTitle(msg.SessionID, title)
	if err != nil {
		log.Printf("ws: failed to rename session %s: %v", msg.SessionID, err)
		h.sendError(client, msg.SessionID, "failed to rename session")
		return
	}

	h.broadcast(NewSessionUpdated(updated))
}

// getOrCreateChatSession returns the chatSession for the given client,
// creating one if it doesn't exist yet.
func (h *MessageHandler) getOrCreateChatSession(client *Client) *chatSession {
	h.sessionsMu.Lock()
	defer h.sessionsMu.Unlock()

	cs, ok := h.sessions[client]
	if !ok {
		cs = &chatSession{agents: make(map[string]agentEntry)}
		h.sessions[client] = cs
	}
	return cs
}

// handleChatStop cancels a running agent for a specific session.
func (h *MessageHandler) handleChatStop(client *Client, msg *InboundMessage) {
	if msg.SessionID == "" {
		h.sendError(client, "", "session ID required")
		return
	}
	if !IsValidUUID(msg.SessionID) {
		h.sendError(client, "", "invalid session ID")
		return
	}

	h.sessionsMu.Lock()
	cs, ok := h.sessions[client]
	h.sessionsMu.Unlock()

	if !ok {
		return
	}

	cs.mu.Lock()
	entry, ok := cs.agents[msg.SessionID]
	if ok {
		delete(cs.agents, msg.SessionID)
	}
	cs.mu.Unlock()

	if ok {
		entry.cancel()
		<-entry.done
	}
}

// OnClientDisconnect cancels all running agents for the given client and
// removes its chatSession entry. Called by the hub when a client unregisters.
func (h *MessageHandler) OnClientDisconnect(client *Client) {
	h.sessionsMu.Lock()
	cs, ok := h.sessions[client]
	if ok {
		delete(h.sessions, client)
	}
	h.sessionsMu.Unlock()

	if !ok {
		return
	}

	cs.mu.Lock()
	agents := make(map[string]agentEntry, len(cs.agents))
	for k, v := range cs.agents {
		agents[k] = v
	}
	cs.agents = nil
	cs.mu.Unlock()

	for _, entry := range agents {
		entry.cancel()
		<-entry.done
	}
}

// completeSession transitions a session from running to completed/completed_unread
// and broadcasts the update. If the client is currently viewing this session,
// it transitions to completed; otherwise to completed_unread (unread badge).
func (h *MessageHandler) completeSession(client *Client, sessionID string) {
	sess, err := h.sessionStore.GetSession(sessionID)
	if err != nil || sess.Status != session.StatusRunning {
		return
	}

	// If client is subscribed to this session, mark completed; otherwise unread.
	newStatus := session.StatusCompletedUnread
	if client.SessionID() == sessionID {
		newStatus = session.StatusCompleted
	}

	if err := h.sessionStore.UpdateSessionStatus(sessionID, newStatus); err != nil {
		log.Printf("ws: failed to complete session %s: %v", sessionID, err)
		return
	}
	if updated, err := h.sessionStore.GetSession(sessionID); err == nil {
		h.broadcast(NewSessionUpdated(updated))

		// Background smart title: after first assistant response (MessageCount == 2).
		if updated.MessageCount == 2 && h.streamClient != nil {
			go h.generateSmartTitle(sessionID)
		}
	}
}

// generateSmartTitle generates a smart title for a session using the Claude API.
// It runs in a background goroutine after the first assistant response.
func (h *MessageHandler) generateSmartTitle(sessionID string) {
	messages, err := h.sessionStore.GetMessages(sessionID)
	if err != nil || len(messages) < 2 {
		return
	}

	var userMsg, assistantMsg string
	for _, m := range messages {
		if m.Role == "user" && userMsg == "" {
			userMsg = m.Content
			if len(userMsg) > 500 {
				userMsg = userMsg[:500]
			}
		}
		if m.Role == "assistant" && assistantMsg == "" {
			assistantMsg = m.Content
			if len(assistantMsg) > 500 {
				assistantMsg = assistantMsg[:500]
			}
		}
		if userMsg != "" && assistantMsg != "" {
			break
		}
	}

	if userMsg == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &stream.Request{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 30,
		System:    "Generate a 3-5 word title for this conversation. Reply with ONLY the title, no quotes, no punctuation at the end.",
		Messages: []stream.Message{
			{Role: "user", Content: []stream.ContentBlock{{Type: "text", Text: fmt.Sprintf("User: %s\n\nAssistant: %s", userMsg, assistantMsg)}}},
		},
	}

	ch, err := h.streamClient.Stream(ctx, req)
	if err != nil {
		log.Printf("ws: smart title stream error for session %s: %v", sessionID, err)
		return
	}

	var title strings.Builder
	for evt := range ch {
		if evt.Type == "content_block_delta" && evt.Delta != nil && evt.Delta.Text != "" {
			title.WriteString(evt.Delta.Text)
		}
	}

	generated := strings.TrimSpace(title.String())
	if generated == "" || len(generated) > 100 {
		return
	}

	if updated, err := h.sessionStore.UpdateSessionTitle(sessionID, generated); err == nil {
		h.broadcast(NewSessionUpdated(updated))
		log.Printf("ws: smart title for session %s: %q", sessionID, generated)
	}
}

// handleSessionResume processes a session.resume message. It replays any
// buffered messages that the client missed since lastMessageId.
func (h *MessageHandler) handleSessionResume(client *Client, msg *InboundMessage) {
	sessionID := msg.SessionID
	lastMsgID := msg.LastMessageID
	if sessionID == "" || lastMsgID == "" {
		return
	}

	var replayed int
	replayOK := false
	if h.hub.replay != nil {
		msgs, found := h.hub.replay.Replay(sessionID, lastMsgID)
		replayOK = found
		replayed = len(msgs)
		for _, m := range msgs {
			client.Send(m)
		}
	}

	if h.metrics != nil {
		if replayOK {
			_ = h.metrics.Log(metrics.EventWSReconnectSuccess, map[string]interface{}{
				"session_id": sessionID,
				"client_id":  client.ID(),
				"replayed":   replayed,
			})
		} else {
			_ = h.metrics.Log(metrics.EventWSReconnectFail, map[string]interface{}{
				"session_id": sessionID,
				"client_id":  client.ID(),
				"reason":     "replay_buffer_miss",
			})
		}
	}
	if h.hub.connHealth != nil {
		h.hub.connHealth.RecordReconnectAttempt(replayOK)
	}

	if h.metrics != nil && replayed > 0 {
		_ = h.metrics.Log(metrics.EventWSStreamResume, map[string]interface{}{
			"session_id":  sessionID,
			"last_msg_id": lastMsgID,
			"replayed":    replayed,
		})
	}
}

// sendError sends a chat.error message to a single client.
func (h *MessageHandler) sendError(client *Client, sessionID, errMsg string) {
	out := NewChatError(sessionID, errMsg)
	h.sendToClient(client, out)
}

// sendToClient marshals and sends a message to a single client.
func (h *MessageHandler) sendToClient(client *Client, msg *OutboundMessage) {
	// Assign replay anchor BEFORE marshaling so it's included in the JSON the client receives
	if h.hub.replay != nil && msg.SessionID != "" && msg.MessageID == "" {
		msg.MessageID = msg.SessionID + "-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}

	data, err := MarshalOutbound(msg)
	if err != nil {
		log.Printf("ws: failed to marshal outbound message: %v", err)
		return
	}
	client.Send(data)

	// Store in replay buffer using the same ID that was just serialized to the client
	if h.hub.replay != nil && msg.SessionID != "" && msg.MessageID != "" {
		h.hub.replay.Store(msg.SessionID, msg.MessageID, data)
	}
}

// broadcast marshals and sends a message to all connected clients via the hub.
func (h *MessageHandler) broadcast(msg *OutboundMessage) {
	data, err := MarshalOutbound(msg)
	if err != nil {
		log.Printf("ws: failed to marshal broadcast message: %v", err)
		return
	}
	h.hub.Broadcast(data)
}

// logAPIError logs an api.error metric event with error classification.
func (h *MessageHandler) logAPIError(sessionID string, err error) {
	if h.metrics == nil {
		return
	}

	data := map[string]interface{}{
		"session_id": sessionID,
	}

	var authErr *stream.AuthError
	var rateLimitErr *stream.RateLimitError
	var serverErr *stream.ServerError
	var apiErr *stream.APIError

	switch {
	case errors.As(err, &authErr):
		data["error_type"] = "auth"
		data["status_code"] = 401
		data["error_message"] = authErr.Error()
	case errors.As(err, &rateLimitErr):
		data["error_type"] = "rate_limit"
		data["status_code"] = 429
		data["error_message"] = rateLimitErr.Error()
		data["retry_after_ms"] = rateLimitErr.RetryAfter.Milliseconds()
	case errors.As(err, &serverErr):
		data["error_type"] = "server"
		data["status_code"] = serverErr.StatusCode
		data["error_message"] = serverErr.Error()
	case errors.Is(err, context.DeadlineExceeded):
		data["error_type"] = "timeout"
		data["status_code"] = 0
		data["error_message"] = err.Error()
	case errors.As(err, &apiErr):
		data["error_type"] = "unknown"
		data["status_code"] = apiErr.StatusCode
		data["error_message"] = apiErr.Error()
	default:
		data["error_type"] = "unknown"
		data["status_code"] = 0
		data["error_message"] = err.Error()
	}

	_ = h.metrics.Log(metrics.EventAPIError, data)
}

// sendClassifiedError sends a user-friendly chat.error message based on the error type.
func (h *MessageHandler) sendClassifiedError(client *Client, sessionID string, err error) {
	var authErr *stream.AuthError
	var rateLimitErr *stream.RateLimitError
	var serverErr *stream.ServerError

	switch {
	case errors.As(err, &authErr):
		h.sendError(client, sessionID, "authentication failed — please re-authenticate")
	case errors.As(err, &rateLimitErr):
		msg := "rate limited — please wait"
		if rateLimitErr.RetryAfter > 0 {
			msg = fmt.Sprintf("rate limited — please wait %v", rateLimitErr.RetryAfter)
		}
		h.sendError(client, sessionID, msg)
	case errors.As(err, &serverErr):
		h.sendError(client, sessionID, "Claude API temporarily unavailable")
	case errors.Is(err, context.DeadlineExceeded):
		h.sendError(client, sessionID, "request timed out — please try again")
	default:
		h.sendError(client, sessionID, "failed to start stream")
	}
}
