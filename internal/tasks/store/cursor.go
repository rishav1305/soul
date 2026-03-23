package store

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

type cursor struct {
	Seq int64 `json:"seq"`
	Ts  int64 `json:"ts"`
}

// EncodeCursor produces an opaque base64-encoded cursor token.
func EncodeCursor(seq, ts int64) string {
	b, _ := json.Marshal(cursor{Seq: seq, Ts: ts})
	return base64.RawURLEncoding.EncodeToString(b)
}

// DecodeCursor parses an opaque cursor token.
// Empty string returns (0, 0, nil) — meaning "full sync".
func DecodeCursor(token string) (seq, ts int64, err error) {
	if token == "" {
		return 0, 0, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, 0, fmt.Errorf("cursor: bad base64: %w", err)
	}
	var c cursor
	if err := json.Unmarshal(b, &c); err != nil {
		return 0, 0, fmt.Errorf("cursor: bad json: %w", err)
	}
	return c.Seq, c.Ts, nil
}
