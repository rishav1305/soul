package transport

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Message is the wire format for hub-agent communication over WebSocket.
type Message struct {
	Type    string          `json:"type"`    // heartbeat, register, command, command_result
	NodeID  string          `json:"node_id"`
	Payload json.RawMessage `json:"payload"`
}

// CreateToken generates a JWT with the nodeID as subject, valid for 24 hours.
func CreateToken(nodeID, secret string) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   nodeID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("transport: sign token: %w", err)
	}
	return signed, nil
}

// VerifyToken validates a JWT and returns the nodeID from the subject claim.
func VerifyToken(tokenStr, secret string) (string, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("transport: unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return "", fmt.Errorf("transport: parse token: %w", err)
	}

	subject, err := token.Claims.GetSubject()
	if err != nil {
		return "", fmt.Errorf("transport: get subject: %w", err)
	}
	if subject == "" {
		return "", fmt.Errorf("transport: empty subject")
	}
	return subject, nil
}
