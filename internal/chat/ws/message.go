package ws

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// Inbound message types sent by WebSocket clients.
const (
	TypeChatSend          = "chat.send"
	TypeChatStop          = "chat.stop"
	TypeSessionSwitch     = "session.switch"
	TypeSessionCreate     = "session.create"
	TypeSessionDelete     = "session.delete"
	TypeSessionRename     = "session.rename"
	TypeSessionSetProduct = "session.setProduct"
)

// Input validation limits.
const (
	// maxTypeLength is the maximum allowed length for the message type field.
	maxTypeLength = 50

	// maxContentLength is the maximum allowed content length for chat.send (32KB).
	maxContentLength = 32 * 1024

	// maxTitleLength is the maximum allowed title length for session.create.
	maxTitleLength = 200
)

// Outbound message types sent to WebSocket clients.
const (
	TypeChatToken         = "chat.token"
	TypeChatDone          = "chat.done"
	TypeChatError         = "chat.error"
	TypeSessionCreated    = "session.created"
	TypeSessionDeleted    = "session.deleted"
	TypeSessionList       = "session.list"
	TypeSessionUpdated    = "session.updated"
	TypeSessionHistory    = "session.history"
	TypeConnectionReady   = "connection.ready"
	TypeSessionProductSet = "session.productSet"
	TypeToolCall          = "tool.call"
	TypeToolComplete      = "tool.complete"
)

// Attachment represents a file attached to a chat message.
type Attachment struct {
	Name      string `json:"name"`
	MediaType string `json:"mediaType"`
	Data      string `json:"data"` // base64 encoded
}

// ThinkingConfig configures Claude's extended thinking mode.
type ThinkingConfig struct {
	Type         string `json:"type"`                    // "disabled", "adaptive", "enabled"
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

// InboundMessage represents a message received from a WebSocket client.
type InboundMessage struct {
	Type        string          `json:"type"`
	SessionID   string          `json:"sessionId,omitempty"`
	Content     string          `json:"content,omitempty"`
	Model       string          `json:"model,omitempty"`
	Attachments []Attachment    `json:"attachments,omitempty"`
	Product     string          `json:"product,omitempty"`
	Thinking    *ThinkingConfig `json:"thinking,omitempty"`
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

	if len(msg.Type) > maxTypeLength {
		return nil, fmt.Errorf("invalid message type")
	}

	return &msg, nil
}

// ValidateChatContent validates and normalizes chat message content.
// Returns the trimmed content or an error if validation fails.
func ValidateChatContent(content string) (string, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", fmt.Errorf("message content required")
	}
	if len(content) > maxContentLength {
		return "", fmt.Errorf("message too large")
	}
	return trimmed, nil
}

// ValidateSessionTitle validates and sanitizes a session title.
// It strips control characters and enforces a length limit.
// An empty title is valid (the store assigns a default).
func ValidateSessionTitle(title string) string {
	// Strip control characters (except common whitespace).
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return -1
		}
		return r
	}, title)

	cleaned = strings.TrimSpace(cleaned)

	if len(cleaned) > maxTitleLength {
		cleaned = cleaned[:maxTitleLength]
	}

	return cleaned
}

// IsValidUUID checks if a string is a valid UUID format (8-4-4-4-12 hex).
func IsValidUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
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

// NewSessionProductSet creates a session.productSet outbound message.
func NewSessionProductSet(sessionID, product string) *OutboundMessage {
	return &OutboundMessage{
		Type:      TypeSessionProductSet,
		SessionID: sessionID,
		Data: map[string]string{
			"product": product,
		},
	}
}

// NewToolCall creates a tool.call outbound message.
func NewToolCall(sessionID, toolID, toolName string, input json.RawMessage) *OutboundMessage {
	return &OutboundMessage{
		Type:      TypeToolCall,
		SessionID: sessionID,
		Data: map[string]interface{}{
			"toolId":   toolID,
			"toolName": toolName,
			"input":    input,
		},
	}
}

// NewToolComplete creates a tool.complete outbound message.
func NewToolComplete(sessionID, toolID, toolName, result string) *OutboundMessage {
	return &OutboundMessage{
		Type:      TypeToolComplete,
		SessionID: sessionID,
		Data: map[string]interface{}{
			"toolId":   toolID,
			"toolName": toolName,
			"result":   result,
		},
	}
}
