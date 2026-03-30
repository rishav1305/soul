package transport

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestVerifyToken_NonHMACSigningMethod(t *testing.T) {
	// Construct a JWT with RS256 alg header to trigger non-HMAC rejection.
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"node-1","exp":9999999999}`))
	fakeToken := header + "." + payload + ".fakesignature"

	_, err := VerifyToken(fakeToken, "secret")
	if err == nil {
		t.Fatal("expected error for non-HMAC signing method")
	}
}

func TestCreateToken_EmptySecret(t *testing.T) {
	// Empty secret should still work (HMAC with empty key).
	token, err := CreateToken("node-1", "")
	if err != nil {
		t.Fatalf("CreateToken with empty secret: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
	nodeID, err := VerifyToken(token, "")
	if err != nil {
		t.Fatalf("VerifyToken with empty secret: %v", err)
	}
	if nodeID != "node-1" {
		t.Errorf("nodeID = %q, want node-1", nodeID)
	}
}

func TestMessage_AllFields(t *testing.T) {
	msg := Message{
		Type:    "command",
		NodeID:  "node-99",
		Payload: json.RawMessage(`{"action":"restart","target":"soul-v2"}`),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Type != "command" {
		t.Errorf("Type = %q, want command", decoded.Type)
	}
	if decoded.NodeID != "node-99" {
		t.Errorf("NodeID = %q, want node-99", decoded.NodeID)
	}
	var payload map[string]string
	if err := json.Unmarshal(decoded.Payload, &payload); err != nil {
		t.Fatalf("Unmarshal payload: %v", err)
	}
	if payload["action"] != "restart" {
		t.Errorf("action = %q, want restart", payload["action"])
	}
}
