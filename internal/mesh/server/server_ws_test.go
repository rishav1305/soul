package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"nhooyr.io/websocket"

	"github.com/rishav1305/soul/internal/mesh/hub"
	"github.com/rishav1305/soul/internal/mesh/store"
	"github.com/rishav1305/soul/internal/mesh/transport"
)

func testServerWithHTTPTest(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "mesh.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	h := hub.New(s)
	srv := New(testNodeInfo(), s, h, "test-secret")

	// Register the node so heartbeats don't fail with FK constraint.
	s.RegisterNode(store.Node{
		ID: "node-ws-test", Name: "ws-test", Host: "127.0.0.1", Port: 3024,
		Role: "agent", Platform: "linux", Arch: "amd64", CPUCores: 4, Status: "online",
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", srv.handleHealth)
	mux.HandleFunc("GET /api/mesh/identity", srv.handleIdentity)
	mux.HandleFunc("GET /api/mesh/nodes", srv.handleListNodes)
	mux.HandleFunc("GET /api/mesh/status", srv.handleStatus)
	mux.HandleFunc("POST /api/mesh/link", srv.handleLink)
	mux.HandleFunc("GET /api/mesh/heartbeats", srv.handleHeartbeats)
	mux.HandleFunc("/ws/mesh", srv.handleWebSocket)
	mux.HandleFunc("POST /api/tools/{name}/execute", srv.handleToolExecute)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return srv, ts
}

func TestHandleWebSocket_Heartbeat(t *testing.T) {
	_, ts := testServerWithHTTPTest(t)

	token, err := transport.CreateToken("node-ws-test", "test-secret")
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + ts.URL[4:] + "/ws/mesh"
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": []string{"Bearer " + token},
		},
	})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	// Send heartbeat message.
	hbPayload, _ := json.Marshal(store.Heartbeat{
		CPUUsagePercent: 42.0,
		RAMAvailableMB:  4096,
	})
	msg := transport.Message{
		Type:    "heartbeat",
		NodeID:  "node-ws-test",
		Payload: hbPayload,
	}
	data, _ := json.Marshal(msg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("Write heartbeat: %v", err)
	}

	// Send register message.
	regPayload, _ := json.Marshal(map[string]interface{}{
		"id": "node-ws-test", "name": "ws-test", "host": "127.0.0.1", "port": 3024,
		"platform": "linux", "arch": "amd64", "cpuCores": 4,
	})
	regMsg := transport.Message{
		Type:    "register",
		NodeID:  "node-ws-test",
		Payload: regPayload,
	}
	regData, _ := json.Marshal(regMsg)
	if err := conn.Write(ctx, websocket.MessageText, regData); err != nil {
		t.Fatalf("Write register: %v", err)
	}

	// Send command_result message.
	cmdMsg := transport.Message{
		Type:    "command_result",
		NodeID:  "node-ws-test",
		Payload: json.RawMessage(`{"result":"ok"}`),
	}
	cmdData, _ := json.Marshal(cmdMsg)
	if err := conn.Write(ctx, websocket.MessageText, cmdData); err != nil {
		t.Fatalf("Write command_result: %v", err)
	}

	// Send unknown message type.
	unkMsg := transport.Message{
		Type:   "unknown_type",
		NodeID: "node-ws-test",
	}
	unkData, _ := json.Marshal(unkMsg)
	if err := conn.Write(ctx, websocket.MessageText, unkData); err != nil {
		t.Fatalf("Write unknown: %v", err)
	}

	// Send invalid JSON (non-parseable).
	if err := conn.Write(ctx, websocket.MessageText, []byte("not json")); err != nil {
		t.Fatalf("Write invalid: %v", err)
	}

	// Give server time to process.
	time.Sleep(100 * time.Millisecond)

	// Close connection to exit the server's read loop.
	conn.Close(websocket.StatusNormalClosure, "test done")
}

func TestHandleWebSocket_RegisterInvalidPayload(t *testing.T) {
	_, ts := testServerWithHTTPTest(t)

	token, err := transport.CreateToken("node-ws-test", "test-secret")
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + ts.URL[4:] + "/ws/mesh"
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": []string{"Bearer " + token},
		},
	})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	// Send register with invalid payload.
	regMsg := transport.Message{
		Type:    "register",
		NodeID:  "node-ws-test",
		Payload: json.RawMessage(`"not an object"`),
	}
	regData, _ := json.Marshal(regMsg)
	if err := conn.Write(ctx, websocket.MessageText, regData); err != nil {
		t.Fatalf("Write: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	conn.Close(websocket.StatusNormalClosure, "done")
}
