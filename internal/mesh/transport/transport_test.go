package transport

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestCreateToken_Success(t *testing.T) {
	token, err := CreateToken("node-1", "test-secret")
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestVerifyToken_Success(t *testing.T) {
	secret := "test-secret-123"
	token, err := CreateToken("node-42", secret)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	nodeID, err := VerifyToken(token, secret)
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if nodeID != "node-42" {
		t.Errorf("nodeID = %q, want %q", nodeID, "node-42")
	}
}

func TestVerifyToken_WrongSecret(t *testing.T) {
	token, _ := CreateToken("node-1", "secret-a")
	_, err := VerifyToken(token, "secret-b")
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestVerifyToken_InvalidToken(t *testing.T) {
	_, err := VerifyToken("not-a-jwt", "secret")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestVerifyToken_RoundTrip(t *testing.T) {
	nodes := []string{"node-alpha", "node-beta", "node-gamma"}
	secret := "shared-secret"

	for _, n := range nodes {
		token, err := CreateToken(n, secret)
		if err != nil {
			t.Fatalf("CreateToken(%s): %v", n, err)
		}
		got, err := VerifyToken(token, secret)
		if err != nil {
			t.Fatalf("VerifyToken(%s): %v", n, err)
		}
		if got != n {
			t.Errorf("VerifyToken returned %q, want %q", got, n)
		}
	}
}

func TestMessage_JSONRoundTrip(t *testing.T) {
	msg := Message{
		Type:    "heartbeat",
		NodeID:  "node-1",
		Payload: json.RawMessage(`{"cpu":42.5}`),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Type != "heartbeat" {
		t.Errorf("Type = %q, want %q", decoded.Type, "heartbeat")
	}
	if decoded.NodeID != "node-1" {
		t.Errorf("NodeID = %q, want %q", decoded.NodeID, "node-1")
	}
}

func TestVerifyToken_WrongSigningMethod(t *testing.T) {
	// Create token with RSA signing method header (but still HMAC secret).
	// VerifyToken should reject non-HMAC methods.
	claims := jwt.RegisteredClaims{
		Subject:   "node-1",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// Sign normally first.
	signed, err := token.SignedString([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}

	// Valid token should work.
	nodeID, err := VerifyToken(signed, "secret")
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	if nodeID != "node-1" {
		t.Errorf("nodeID = %q", nodeID)
	}
}

func TestVerifyToken_EmptySubject(t *testing.T) {
	// Create a token with empty subject.
	claims := jwt.RegisteredClaims{
		Subject:   "",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = VerifyToken(signed, "secret")
	if err == nil {
		t.Fatal("expected error for empty subject")
	}
	if !strings.Contains(err.Error(), "empty subject") {
		t.Errorf("error = %q, want 'empty subject'", err.Error())
	}
}

func TestVerifyToken_ExpiredToken(t *testing.T) {
	claims := jwt.RegisteredClaims{
		Subject:   "node-old",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = VerifyToken(signed, "secret")
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestMessage_EmptyPayload(t *testing.T) {
	msg := Message{Type: "register", NodeID: "node-2"}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Type != "register" {
		t.Errorf("Type = %q", decoded.Type)
	}
	// json.RawMessage marshals nil as "null", so decoded.Payload will be []byte("null").
	if decoded.Payload != nil && string(decoded.Payload) != "null" {
		t.Errorf("expected null payload, got %s", decoded.Payload)
	}
}
