package ws

import (
	"encoding/json"
	"testing"
)

func TestParseInboundMessage_ChatSend(t *testing.T) {
	raw := []byte(`{"type":"chat.send","sessionId":"abc-123","content":"hello world"}`)
	msg, err := ParseInboundMessage(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != TypeChatSend {
		t.Errorf("expected type %q, got %q", TypeChatSend, msg.Type)
	}
	if msg.SessionID != "abc-123" {
		t.Errorf("expected sessionId abc-123, got %q", msg.SessionID)
	}
	if msg.Content != "hello world" {
		t.Errorf("expected content 'hello world', got %q", msg.Content)
	}
}

func TestParseInboundMessage_SessionSwitch(t *testing.T) {
	raw := []byte(`{"type":"session.switch","sessionId":"sess-456"}`)
	msg, err := ParseInboundMessage(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != TypeSessionSwitch {
		t.Errorf("expected type %q, got %q", TypeSessionSwitch, msg.Type)
	}
	if msg.SessionID != "sess-456" {
		t.Errorf("expected sessionId sess-456, got %q", msg.SessionID)
	}
}

func TestParseInboundMessage_SessionCreate(t *testing.T) {
	raw := []byte(`{"type":"session.create"}`)
	msg, err := ParseInboundMessage(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != TypeSessionCreate {
		t.Errorf("expected type %q, got %q", TypeSessionCreate, msg.Type)
	}
}

func TestParseInboundMessage_SessionDelete(t *testing.T) {
	raw := []byte(`{"type":"session.delete","sessionId":"sess-789"}`)
	msg, err := ParseInboundMessage(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != TypeSessionDelete {
		t.Errorf("expected type %q, got %q", TypeSessionDelete, msg.Type)
	}
	if msg.SessionID != "sess-789" {
		t.Errorf("expected sessionId sess-789, got %q", msg.SessionID)
	}
}

func TestParseInboundMessage_EmptyData(t *testing.T) {
	_, err := ParseInboundMessage([]byte{})
	if err == nil {
		t.Fatal("expected error for empty message")
	}
}

func TestParseInboundMessage_InvalidJSON(t *testing.T) {
	_, err := ParseInboundMessage([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseInboundMessage_MissingType(t *testing.T) {
	_, err := ParseInboundMessage([]byte(`{"sessionId":"abc"}`))
	if err == nil {
		t.Fatal("expected error for missing type")
	}
}

func TestParseInboundMessage_EmptyType(t *testing.T) {
	_, err := ParseInboundMessage([]byte(`{"type":""}`))
	if err == nil {
		t.Fatal("expected error for empty type")
	}
}

func TestParseInboundMessage_EmptyObject(t *testing.T) {
	_, err := ParseInboundMessage([]byte(`{}`))
	if err == nil {
		t.Fatal("expected error for empty object (missing type)")
	}
}

func TestParseInboundMessage_ExtraFields(t *testing.T) {
	raw := []byte(`{"type":"chat.send","sessionId":"abc","content":"hi","extra":"ignored"}`)
	msg, err := ParseInboundMessage(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != TypeChatSend {
		t.Errorf("expected type %q, got %q", TypeChatSend, msg.Type)
	}
}

func TestParseInboundMessage_UnknownType(t *testing.T) {
	raw := []byte(`{"type":"unknown.type"}`)
	msg, err := ParseInboundMessage(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != "unknown.type" {
		t.Errorf("expected type unknown.type, got %q", msg.Type)
	}
}

// --- Outbound message tests ---

func TestMarshalOutbound_ChatToken(t *testing.T) {
	msg := NewChatToken("sess-1", "Hello", "msg-1")
	data, err := MarshalOutbound(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if result["type"] != TypeChatToken {
		t.Errorf("expected type %q, got %v", TypeChatToken, result["type"])
	}
	if result["sessionId"] != "sess-1" {
		t.Errorf("expected sessionId sess-1, got %v", result["sessionId"])
	}
	dataMap := result["data"].(map[string]interface{})
	if dataMap["token"] != "Hello" {
		t.Errorf("expected token Hello, got %v", dataMap["token"])
	}
	if dataMap["messageId"] != "msg-1" {
		t.Errorf("expected messageId msg-1, got %v", dataMap["messageId"])
	}
}

func TestMarshalOutbound_ChatDone(t *testing.T) {
	msg := NewChatDone("sess-1", "msg-1")
	data, err := MarshalOutbound(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if result["type"] != TypeChatDone {
		t.Errorf("expected type %q, got %v", TypeChatDone, result["type"])
	}
	if result["sessionId"] != "sess-1" {
		t.Errorf("expected sessionId sess-1, got %v", result["sessionId"])
	}
}

func TestMarshalOutbound_ChatError(t *testing.T) {
	msg := NewChatError("sess-1", "something went wrong")
	data, err := MarshalOutbound(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if result["type"] != TypeChatError {
		t.Errorf("expected type %q, got %v", TypeChatError, result["type"])
	}
	dataMap := result["data"].(map[string]interface{})
	if dataMap["error"] != "something went wrong" {
		t.Errorf("expected error message, got %v", dataMap["error"])
	}
}

func TestMarshalOutbound_SessionCreated(t *testing.T) {
	sess := map[string]string{"id": "sess-new", "title": "New Session"}
	msg := NewSessionCreated(sess)
	data, err := MarshalOutbound(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if result["type"] != TypeSessionCreated {
		t.Errorf("expected type %q, got %v", TypeSessionCreated, result["type"])
	}
}

func TestMarshalOutbound_SessionDeleted(t *testing.T) {
	msg := NewSessionDeleted("sess-del")
	data, err := MarshalOutbound(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if result["type"] != TypeSessionDeleted {
		t.Errorf("expected type %q, got %v", TypeSessionDeleted, result["type"])
	}
	if result["sessionId"] != "sess-del" {
		t.Errorf("expected sessionId sess-del, got %v", result["sessionId"])
	}
}

func TestMarshalOutbound_SessionList(t *testing.T) {
	sessions := []map[string]string{
		{"id": "sess-1", "title": "Session 1"},
		{"id": "sess-2", "title": "Session 2"},
	}
	msg := NewSessionList(sessions)
	data, err := MarshalOutbound(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if result["type"] != TypeSessionList {
		t.Errorf("expected type %q, got %v", TypeSessionList, result["type"])
	}
}

func TestMarshalOutbound_SessionUpdated(t *testing.T) {
	sess := map[string]string{"id": "sess-upd", "title": "Updated"}
	msg := NewSessionUpdated(sess)
	data, err := MarshalOutbound(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if result["type"] != TypeSessionUpdated {
		t.Errorf("expected type %q, got %v", TypeSessionUpdated, result["type"])
	}
}

func TestMarshalOutbound_ConnectionReady(t *testing.T) {
	msg := NewConnectionReady("ws-00000001-abcd")
	data, err := MarshalOutbound(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if result["type"] != TypeConnectionReady {
		t.Errorf("expected type %q, got %v", TypeConnectionReady, result["type"])
	}
	dataMap := result["data"].(map[string]interface{})
	if dataMap["clientId"] != "ws-00000001-abcd" {
		t.Errorf("expected clientId ws-00000001-abcd, got %v", dataMap["clientId"])
	}
}

func TestMarshalOutbound_EmptySessionID_Omitted(t *testing.T) {
	msg := NewConnectionReady("ws-1")
	data, err := MarshalOutbound(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if _, ok := result["sessionId"]; ok {
		t.Error("expected sessionId to be omitted when empty")
	}
}

func TestMarshalOutbound_RoundTrip(t *testing.T) {
	original := &OutboundMessage{
		Type:      TypeChatToken,
		SessionID: "sess-rt",
		Data: map[string]string{
			"token":     "world",
			"messageId": "msg-rt",
		},
	}

	data, err := MarshalOutbound(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded OutboundMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Type != original.Type {
		t.Errorf("type mismatch: got %q, want %q", decoded.Type, original.Type)
	}
	if decoded.SessionID != original.SessionID {
		t.Errorf("sessionId mismatch: got %q, want %q", decoded.SessionID, original.SessionID)
	}
}
