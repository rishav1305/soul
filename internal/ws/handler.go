package ws

import (
	"log"
	"strings"

	"github.com/rishav1305/soul-v2/internal/metrics"
	"github.com/rishav1305/soul-v2/internal/session"
)

// MessageHandler processes inbound WebSocket messages and dispatches them
// to the appropriate session and hub operations. It is safe for concurrent
// use from multiple ReadPump goroutines because it only reads its own fields
// (hub, sessionStore, metrics) and delegates state mutations to the hub
// event loop or the thread-safe session store.
type MessageHandler struct {
	hub          *Hub
	sessionStore *session.Store
	metrics      *metrics.EventLogger
}

// NewMessageHandler creates a new MessageHandler with the given dependencies.
func NewMessageHandler(hub *Hub, store *session.Store, mel *metrics.EventLogger) *MessageHandler {
	return &MessageHandler{
		hub:          hub,
		sessionStore: store,
		metrics:      mel,
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
// stores the user message, and sends a chat.done acknowledgement. Claude streaming
// integration comes in Phase 4.
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

	// For now, echo back a chat.done. Claude streaming comes in Phase 4.
	out := NewChatDone(msg.SessionID, stored.ID)
	h.sendToClient(client, out)
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
