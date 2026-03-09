package ws

import (
	"encoding/json"
	"fmt"
)

// Inbound message types sent by WebSocket clients.
const (
	TypeChatSend      = "chat.send"
	TypeSessionSwitch = "session.switch"
	TypeSessionCreate = "session.create"
	TypeSessionDelete = "session.delete"
)

// Outbound message types sent to WebSocket clients.
const (
	TypeChatToken       = "chat.token"
	TypeChatDone        = "chat.done"
	TypeChatError       = "chat.error"
	TypeSessionCreated  = "session.created"
	TypeSessionDeleted  = "session.deleted"
	TypeSessionList     = "session.list"
	TypeSessionUpdated  = "session.updated"
	TypeSessionHistory  = "session.history"
	TypeConnectionReady = "connection.ready"
)

// InboundMessage represents a message received from a WebSocket client.
type InboundMessage struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionId,omitempty"`
	Content   string `json:"content,omitempty"`
}

// OutboundMessage represents a message sent to a WebSocket client.
type OutboundMessage struct {
	Type      string      `json:"type"`
	SessionID string      `json:"sessionId,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

// ParseInboundMessage parses raw JSON bytes into an InboundMessage.
// It returns an error if the JSON is malformed or the type field is empty.
func ParseInboundMessage(raw []byte) (*InboundMessage, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty message")
	}

	var msg InboundMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	if msg.Type == "" {
		return nil, fmt.Errorf("missing message type")
	}

	return &msg, nil
}

// MarshalOutbound serializes an OutboundMessage to JSON bytes.
func MarshalOutbound(msg *OutboundMessage) ([]byte, error) {
	return json.Marshal(msg)
}

// NewChatToken creates a chat.token outbound message.
func NewChatToken(sessionID, token, messageID string) *OutboundMessage {
	return &OutboundMessage{
		Type:      TypeChatToken,
		SessionID: sessionID,
		Data: map[string]string{
			"token":     token,
			"messageId": messageID,
		},
	}
}

// NewChatDone creates a chat.done outbound message.
func NewChatDone(sessionID, messageID string) *OutboundMessage {
	return &OutboundMessage{
		Type:      TypeChatDone,
		SessionID: sessionID,
		Data: map[string]string{
			"messageId": messageID,
		},
	}
}

// NewChatError creates a chat.error outbound message.
func NewChatError(sessionID, errMsg string) *OutboundMessage {
	return &OutboundMessage{
		Type:      TypeChatError,
		SessionID: sessionID,
		Data: map[string]string{
			"error": errMsg,
		},
	}
}

// NewSessionCreated creates a session.created outbound message.
func NewSessionCreated(session interface{}) *OutboundMessage {
	return &OutboundMessage{
		Type: TypeSessionCreated,
		Data: map[string]interface{}{
			"session": session,
		},
	}
}

// NewSessionDeleted creates a session.deleted outbound message.
func NewSessionDeleted(sessionID string) *OutboundMessage {
	return &OutboundMessage{
		Type:      TypeSessionDeleted,
		SessionID: sessionID,
	}
}

// NewSessionList creates a session.list outbound message.
func NewSessionList(sessions interface{}) *OutboundMessage {
	return &OutboundMessage{
		Type: TypeSessionList,
		Data: map[string]interface{}{
			"sessions": sessions,
		},
	}
}

// NewSessionHistory creates a session.history outbound message containing
// the message history for a specific session.
func NewSessionHistory(sessionID string, messages interface{}) *OutboundMessage {
	return &OutboundMessage{
		Type:      TypeSessionHistory,
		SessionID: sessionID,
		Data: map[string]interface{}{
			"messages": messages,
		},
	}
}

// NewSessionUpdated creates a session.updated outbound message.
func NewSessionUpdated(session interface{}) *OutboundMessage {
	return &OutboundMessage{
		Type: TypeSessionUpdated,
		Data: map[string]interface{}{
			"session": session,
		},
	}
}

// NewConnectionReady creates a connection.ready outbound message.
func NewConnectionReady(clientID string) *OutboundMessage {
	return &OutboundMessage{
		Type: TypeConnectionReady,
		Data: map[string]string{
			"clientId": clientID,
		},
	}
}
