package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rishav1305/soul/internal/mesh/transport"
)

func TestHandleWebSocket_RejectsQueryToken(t *testing.T) {
	s := &Server{secret: "test-secret"}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Create valid JWT token
	token, err := transport.CreateToken("node-1", "test-secret")
	if err != nil {
		t.Fatal(err)
	}

	// Query param should be rejected
	req, _ := http.NewRequest("GET", ts.URL+"/ws?token="+token, nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Sec-WebSocket-Version", "13")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for query token, got %d", resp.StatusCode)
	}
}

func TestHandleWebSocket_AcceptsAuthorizationHeader(t *testing.T) {
	s := &Server{secret: "test-secret"}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	token, err := transport.CreateToken("node-1", "test-secret")
	if err != nil {
		t.Fatal(err)
	}

	// Authorization header should be accepted (non-401 response)
	req, _ := http.NewRequest("GET", ts.URL+"/ws", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Sec-WebSocket-Version", "13")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		t.Error("expected non-401 for valid Authorization header")
	}
}

func TestHandleWebSocket_RejectsNoAuth(t *testing.T) {
	s := &Server{secret: "test-secret"}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/ws", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 with no auth, got %d", resp.StatusCode)
	}
}
