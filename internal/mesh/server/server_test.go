package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul/internal/mesh/hub"
	"github.com/rishav1305/soul/internal/mesh/node"
	"github.com/rishav1305/soul/internal/mesh/store"
	"github.com/rishav1305/soul/internal/mesh/transport"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func testNodeInfo() node.NodeInfo {
	return node.NodeInfo{
		ID:       "node-test-001",
		Name:     "test-node",
		Host:     "127.0.0.1",
		Port:     3024,
		Platform: "linux",
		Arch:     "amd64",
		CPUCores: 4,
		IsHub:    true,
	}
}

func testServer(t *testing.T) *Server {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "mesh.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	h := hub.New(s)
	return New(testNodeInfo(), s, h, "test-secret")
}

func testServerMinimal() *Server {
	return &Server{
		nodeInfo: testNodeInfo(),
		secret:   "test-secret",
	}
}

// ---------------------------------------------------------------------------
// handleHealth
// ---------------------------------------------------------------------------

func TestHandleHealth(t *testing.T) {
	s := testServerMinimal()
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	s.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q, want ok", body["status"])
	}
}

// ---------------------------------------------------------------------------
// handleIdentity
// ---------------------------------------------------------------------------

func TestHandleIdentity(t *testing.T) {
	s := testServerMinimal()
	req := httptest.NewRequest("GET", "/api/mesh/identity", nil)
	w := httptest.NewRecorder()
	s.handleIdentity(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if body["id"] != "node-test-001" {
		t.Errorf("id = %v, want node-test-001", body["id"])
	}
	if body["name"] != "test-node" {
		t.Errorf("name = %v, want test-node", body["name"])
	}
	if body["platform"] != "linux" {
		t.Errorf("platform = %v, want linux", body["platform"])
	}
	if body["arch"] != "amd64" {
		t.Errorf("arch = %v, want amd64", body["arch"])
	}
	if body["isHub"] != true {
		t.Errorf("isHub = %v, want true", body["isHub"])
	}
	cpuCores, ok := body["cpuCores"].(float64)
	if !ok || cpuCores != 4 {
		t.Errorf("cpuCores = %v, want 4", body["cpuCores"])
	}
}

// ---------------------------------------------------------------------------
// handleListNodes
// ---------------------------------------------------------------------------

func TestHandleListNodes_Empty(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest("GET", "/api/mesh/nodes", nil)
	w := httptest.NewRecorder()
	s.handleListNodes(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var nodes []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &nodes); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected empty list, got %d nodes", len(nodes))
	}
}

func TestHandleListNodes_WithNode(t *testing.T) {
	s := testServer(t)

	// Register a node
	n := store.Node{
		ID:       "node-1",
		Name:     "alpha",
		Host:     "192.168.1.1",
		Port:     3024,
		Role:     "agent",
		Platform: "linux",
		Arch:     "amd64",
		CPUCores: 8,
		Status:   "online",
	}
	if err := s.store.RegisterNode(n); err != nil {
		t.Fatalf("RegisterNode: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/mesh/nodes", nil)
	w := httptest.NewRecorder()
	s.handleListNodes(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var nodes []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &nodes); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0]["id"] != "node-1" {
		t.Errorf("node id = %v, want node-1", nodes[0]["id"])
	}
}

// ---------------------------------------------------------------------------
// handleLink
// ---------------------------------------------------------------------------

func TestHandleLink_CreatesCode(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest("POST", "/api/mesh/link", nil)
	w := httptest.NewRecorder()
	s.handleLink(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	code := body["code"]
	if len(code) != 8 { // 4 bytes = 8 hex chars
		t.Errorf("code length = %d, want 8 hex chars", len(code))
	}
	if body["expiresAt"] == "" {
		t.Error("expiresAt should not be empty")
	}
}

// ---------------------------------------------------------------------------
// handleHeartbeats
// ---------------------------------------------------------------------------

func TestHandleHeartbeats_MissingNodeID(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest("GET", "/api/mesh/heartbeats", nil)
	w := httptest.NewRecorder()
	s.handleHeartbeats(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleHeartbeats_EmptyResult(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest("GET", "/api/mesh/heartbeats?node_id=nonexistent", nil)
	w := httptest.NewRecorder()
	s.handleHeartbeats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var hbs []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &hbs); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(hbs) != 0 {
		t.Errorf("expected empty list, got %d", len(hbs))
	}
}

func TestHandleHeartbeats_WithLimit(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest("GET", "/api/mesh/heartbeats?node_id=node-1&limit=5", nil)
	w := httptest.NewRecorder()
	s.handleHeartbeats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// ---------------------------------------------------------------------------
// handleToolExecute
// ---------------------------------------------------------------------------

func TestHandleToolExecute_MissingName(t *testing.T) {
	s := testServerMinimal()

	body := bytes.NewBufferString(`{"input": "test"}`)
	req := httptest.NewRequest("POST", "/api/tools//execute", body)
	// PathValue won't work without mux routing, so test directly
	w := httptest.NewRecorder()
	s.handleToolExecute(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleToolExecute_ValidTool(t *testing.T) {
	s := testServerMinimal()

	body := bytes.NewBufferString(`{"input": "test"}`)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/tools/{name}/execute", s.handleToolExecute)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/tools/list_nodes/execute", "application/json", body)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["tool"] != "list_nodes" {
		t.Errorf("tool = %v, want list_nodes", result["tool"])
	}
	if result["status"] != "not_implemented" {
		t.Errorf("status = %v, want not_implemented", result["status"])
	}
}

func TestHandleToolExecute_InvalidBody(t *testing.T) {
	s := testServerMinimal()

	body := bytes.NewBufferString(`not json`)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/tools/{name}/execute", s.handleToolExecute)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/tools/test/execute", "application/json", body)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// ---------------------------------------------------------------------------
// handleStatus
// ---------------------------------------------------------------------------

func TestHandleStatus(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest("GET", "/api/mesh/status", nil)
	w := httptest.NewRecorder()
	s.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["nodeId"] != "node-test-001" {
		t.Errorf("nodeId = %v, want node-test-001", body["nodeId"])
	}
	if body["isHub"] != true {
		t.Errorf("isHub = %v, want true", body["isHub"])
	}
}

// ---------------------------------------------------------------------------
// writeJSON
// ---------------------------------------------------------------------------

func TestWriteJSON_SetsContentType(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"key": "value"})

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestWriteJSON_CustomStatus(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, map[string]string{"id": "123"})

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}

// ---------------------------------------------------------------------------
// New constructor
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	info := testNodeInfo()
	s := New(info, nil, nil, "secret")
	if s.nodeInfo.ID != "node-test-001" {
		t.Errorf("nodeInfo.ID = %q, want node-test-001", s.nodeInfo.ID)
	}
	if s.secret != "secret" {
		t.Errorf("secret = %q, want secret", s.secret)
	}
}

// ---------------------------------------------------------------------------
// WebSocket auth (existing tests, preserved)
// ---------------------------------------------------------------------------

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

func TestHandleWebSocket_RejectsInvalidToken(t *testing.T) {
	s := &Server{secret: "test-secret"}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/ws", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-here")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid token, got %d", resp.StatusCode)
	}
}
