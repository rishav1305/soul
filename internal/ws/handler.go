package ws

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/rishav1305/soul-v2/internal/metrics"
	"github.com/rishav1305/soul-v2/internal/session"
	"github.com/rishav1305/soul-v2/internal/stream"
)

// MessageHandler processes inbound WebSocket messages and dispatches them
// to the appropriate session and hub operations. It is safe for concurrent
// use from multiple ReadPump goroutines because it only reads its own fields
// (hub, sessionStore, streamClient, metrics) and delegates state mutations
// to the hub event loop or the thread-safe session store.
type MessageHandler struct {
	hub          *Hub
	sessionStore *session.Store
	streamClient *stream.Client
	metrics      *metrics.EventLogger
}

// NewMessageHandler creates a new MessageHandler with the given dependencies.
// The streamClient parameter may be nil — if so, chat.send will store the user
// message and immediately return chat.done without streaming (Phase 3 behavior).
func NewMessageHandler(hub *Hub, store *session.Store, mel *metrics.EventLogger, opts ...MessageHandlerOption) *MessageHandler {
	h := &MessageHandler{
		hub:          hub,
		sessionStore: store,
		metrics:      mel,
	}
	for _, opt := range opts {
		opt(h)
	}
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
	case TypeSessionSwitch:
		h.handleSessionSwitch(client, msg)
	case TypeSessionCreate:
		h.handleSessionCreate(client, msg)
	case TypeSessionDelete:
		h.handleSessionDelete(client, msg)
	default:
		log.Printf("ws: unknown message type %q from client %s", msg.Type, client.ID())
	}
}

// handleChatSend processes a chat.send message. It validates the session exists,
// stores the user message, builds conversation history, and streams the response
// from the Claude API back to the client as chat.token events.
func (h *MessageHandler) handleChatSend(client *Client, msg *InboundMessage) {
	if msg.SessionID == "" {
		h.sendError(client, "", "session ID required")
		return
	}
	if msg.Content == "" {
		h.sendError(client, msg.SessionID, "message content required")
		return
	}

	// Validate session exists.
	_, err := h.sessionStore.GetSession(msg.SessionID)
	if err != nil {
		h.sendError(client, msg.SessionID, "session not found")
		return
	}

	// Store the user message.
	stored, err := h.sessionStore.AddMessage(msg.SessionID, "user", msg.Content)
	if err != nil {
		log.Printf("ws: failed to store message: %v", err)
		h.sendError(client, msg.SessionID, "failed to store message")
		return
	}

	// If no stream client is configured, fall back to immediate chat.done.
	if h.streamClient == nil {
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

	apiMessages := make([]stream.Message, 0, len(sessionMsgs))
	for _, sm := range sessionMsgs {
		if sm.Role != "user" && sm.Role != "assistant" {
			continue
		}
		apiMessages = append(apiMessages, stream.Message{
			Role: sm.Role,
			Content: []stream.ContentBlock{
				{Type: "text", Text: sm.Content},
			},
		})
	}

	req := &stream.Request{
		MaxTokens: 4096,
		Messages:  apiMessages,
	}

	// Generate a message ID for the assistant response ahead of time.
	// We'll use the stored user message ID as a reference, and create
	// the assistant message after streaming completes.
	sessionID := msg.SessionID

	// Stream in a goroutine so we don't block the ReadPump.
	go h.runStream(client, sessionID, req)
}

// runStream executes the Claude API streaming call and forwards events to the client.
func (h *MessageHandler) runStream(client *Client, sessionID string, req *stream.Request) {
	startTime := time.Now()

	// Use the client's context so the stream is cancelled on disconnect.
	ctx := client.Context()

	ch, err := h.streamClient.Stream(ctx, req)
	if err != nil {
		log.Printf("ws: stream error for session %s: %v", sessionID, err)
		h.logAPIError(sessionID, err)
		h.sendClassifiedError(client, sessionID, err)
		return
	}

	// Log stream start.
	if h.metrics != nil {
		_ = h.metrics.Log(metrics.EventWSStreamStart, map[string]interface{}{
			"session_id": sessionID,
			"client_id":  client.ID(),
		})
	}

	var fullText strings.Builder
	var messageID string
	var model string
	var totalInputTokens int
	var totalOutputTokens int
	var firstTokenLogged bool
	var gotMessageStop bool

	for evt := range ch {
		switch evt.Type {
		case "message_start":
			if evt.Message != nil {
				messageID = evt.Message.ID
				model = evt.Message.Model
				if evt.Message.Usage != nil {
					totalInputTokens = evt.Message.Usage.InputTokens
				}
			}

		case "content_block_delta":
			if evt.Delta != nil && evt.Delta.Text != "" {
				token := evt.Delta.Text
				fullText.WriteString(token)
				tokenMsg := NewChatToken(sessionID, token, messageID)
				h.sendToClient(client, tokenMsg)

				// Log first token only (per spec: ws.stream.token).
				if !firstTokenLogged && h.metrics != nil {
					firstTokenLogged = true
					_ = h.metrics.Log(metrics.EventWSStreamToken, map[string]interface{}{
						"session_id":     sessionID,
						"client_id":      client.ID(),
						"message_id":     messageID,
						"first_token_ms": time.Since(startTime).Milliseconds(),
					})
				}
			}

		case "message_delta":
			if evt.Usage != nil {
				totalOutputTokens = evt.Usage.OutputTokens
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
		h.sendError(client, sessionID, "stream ended unexpectedly")
		return
	}

	// Store the assistant response.
	if fullText.Len() > 0 {
		assistantMsg, err := h.sessionStore.AddMessage(sessionID, "assistant", fullText.String())
		if err != nil {
			log.Printf("ws: failed to store assistant message: %v", err)
		} else if messageID == "" {
			messageID = assistantMsg.ID
		}
	}

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
}

// handleSessionSwitch processes a session.switch message. It validates the
// session exists, subscribes the client, and sends the session list with history.
func (h *MessageHandler) handleSessionSwitch(client *Client, msg *InboundMessage) {
	if msg.SessionID == "" {
		h.sendError(client, "", "session ID required")
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

	// Send session.updated with the full session object.
	updated := NewSessionUpdated(sess)
	h.sendToClient(client, updated)

	// Send full session list to the client.
	sessions, err := h.sessionStore.ListSessions()
	if err != nil {
		log.Printf("ws: failed to list sessions: %v", err)
		h.sendError(client, msg.SessionID, "failed to list sessions")
		return
	}
	listMsg := NewSessionList(sessions)
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
	sess, err := h.sessionStore.CreateSession("")
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

// sendError sends a chat.error message to a single client.
func (h *MessageHandler) sendError(client *Client, sessionID, errMsg string) {
	out := NewChatError(sessionID, errMsg)
	h.sendToClient(client, out)
}

// sendToClient marshals and sends a message to a single client.
func (h *MessageHandler) sendToClient(client *Client, msg *OutboundMessage) {
	data, err := MarshalOutbound(msg)
	if err != nil {
		log.Printf("ws: failed to marshal outbound message: %v", err)
		return
	}
	client.Send(data)
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
