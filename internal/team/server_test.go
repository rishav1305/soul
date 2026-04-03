package team

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testServer creates a Server with temp dirs for testing.
func testServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()

	hbDir := filepath.Join(dir, "heartbeat")
	_ = os.MkdirAll(hbDir, 0755)
	inboxDir := filepath.Join(dir, "inboxes")
	_ = os.MkdirAll(inboxDir, 0755)
	panesFile := filepath.Join(dir, "panes.json")
	_ = os.WriteFile(panesFile, []byte(`{"shuri": "%5", "happy": "%8"}`), 0644)

	// Write a heartbeat.
	hb := `{"agent":"shuri","status":"working","current_task":"testing","since":"` +
		time.Now().Format(time.RFC3339) + `","updated":"` + time.Now().Format(time.RFC3339) +
		`","blocked_on":null,"next_task":"deploy"}`
	_ = os.WriteFile(filepath.Join(hbDir, "shuri.json"), []byte(hb), 0644)

	cfg := Config{
		Host:           "127.0.0.1",
		Port:           0,
		TeamName:       "test-team",
		ClawteamDir:    dir,
		HeartbeatDir:   hbDir,
		BacklogCLI:     "/nonexistent/backlog_cli.py",
		TmuxSession:    "test",
		PanesFile:      panesFile,
		PollIntervalMS: 60000, // Long poll to avoid interference.
		PaneBufferSize: 100,
	}

	panes, _ := LoadPanesMap(panesFile)
	hub := NewHub()
	poller := NewPanePoller(panes, cfg.PaneBufferSize, cfg.PollIntervalMS, cfg.TmuxSession)
	state := NewAgentStateAggregator(cfg, panes)
	inbox := NewInboxBridge(cfg.ClawteamDir, cfg.TeamName)
	tasks := NewTaskBridge(cfg.BacklogCLI)

	_ = state.Refresh()

	return &Server{
		cfg:       cfg,
		hub:       hub,
		poller:    poller,
		state:     state,
		inbox:     inbox,
		tasks:     tasks,
		startTime: time.Now(),
	}
}

func TestHandleHealth(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/team/health", nil)
	w := httptest.NewRecorder()

	srv.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var health HealthStatus
	if err := json.Unmarshal(w.Body.Bytes(), &health); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	if health.Agents != 2 {
		t.Errorf("expected 2 agents, got %d", health.Agents)
	}
	if health.Services["team-server"] != "ok" {
		t.Errorf("expected team-server ok")
	}
	// Verify resources are populated (hostname at minimum).
	if health.Resources.Hostname == "" {
		t.Error("expected non-empty hostname in resources")
	}
	// TokenUsage should be a non-nil slice (may be empty).
	if health.TokenUsage == nil {
		t.Error("expected non-nil token_usage slice")
	}
}

func TestHandleListAgents(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/team/agents", nil)
	w := httptest.NewRecorder()

	srv.handleListAgents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var agents []AgentState
	if err := json.Unmarshal(w.Body.Bytes(), &agents); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(agents))
	}

	// Find shuri.
	var found bool
	for _, a := range agents {
		if a.Name == "shuri" {
			found = true
			if a.Status != "working" {
				t.Errorf("expected shuri status working, got %s", a.Status)
			}
			if a.Machine != "titan-pc" {
				t.Errorf("expected shuri machine titan-pc, got %s", a.Machine)
			}
		}
	}
	if !found {
		t.Error("shuri not found in agents list")
	}
}

func TestHandleListAgents_ExcludesTeamLead(t *testing.T) {
	dir := t.TempDir()

	hbDir := filepath.Join(dir, "heartbeat")
	_ = os.MkdirAll(hbDir, 0755)
	inboxDir := filepath.Join(dir, "inboxes")
	_ = os.MkdirAll(inboxDir, 0755)
	// Include team-lead in panes to verify it gets filtered out.
	panesFile := filepath.Join(dir, "panes.json")
	_ = os.WriteFile(panesFile, []byte(`{"team-lead": "%0", "shuri": "%5"}`), 0644)

	cfg := Config{
		Host:           "127.0.0.1",
		Port:           0,
		TeamName:       "test-team",
		ClawteamDir:    dir,
		HeartbeatDir:   hbDir,
		BacklogCLI:     "/nonexistent/backlog_cli.py",
		TmuxSession:    "test",
		PanesFile:      panesFile,
		PollIntervalMS: 60000,
		PaneBufferSize: 100,
	}

	panes, _ := LoadPanesMap(panesFile)
	hub := NewHub()
	poller := NewPanePoller(panes, cfg.PaneBufferSize, cfg.PollIntervalMS, cfg.TmuxSession)
	state := NewAgentStateAggregator(cfg, panes)
	inbox := NewInboxBridge(cfg.ClawteamDir, cfg.TeamName)
	tasks := NewTaskBridge(cfg.BacklogCLI)
	_ = state.Refresh()

	srv := &Server{
		cfg:       cfg,
		hub:       hub,
		poller:    poller,
		state:     state,
		inbox:     inbox,
		tasks:     tasks,
		startTime: time.Now(),
	}

	req := httptest.NewRequest("GET", "/api/team/agents", nil)
	w := httptest.NewRecorder()
	srv.handleListAgents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var agents []AgentState
	if err := json.Unmarshal(w.Body.Bytes(), &agents); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	// Should only have shuri, not team-lead.
	if len(agents) != 1 {
		t.Errorf("expected 1 agent (team-lead filtered), got %d", len(agents))
	}
	for _, a := range agents {
		if a.Name == "team-lead" {
			t.Error("team-lead should be excluded from agents list")
		}
	}
}

func TestHandleGetAgent(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/team/agents/shuri", nil)
	req.SetPathValue("name", "shuri")
	w := httptest.NewRecorder()

	srv.handleGetAgent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var agent AgentState
	if err := json.Unmarshal(w.Body.Bytes(), &agent); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	if agent.Name != "shuri" {
		t.Errorf("expected name shuri, got %s", agent.Name)
	}
}

func TestHandleGetAgent_NotFound(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/team/agents/unknown", nil)
	req.SetPathValue("name", "unknown")
	w := httptest.NewRecorder()

	srv.handleGetAgent(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleGetPane_NotFound(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/team/agents/shuri/pane", nil)
	req.SetPathValue("name", "shuri")
	w := httptest.NewRecorder()

	srv.handleGetPane(w, req)

	// No pane data yet (poller hasn't run).
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 (no pane data), got %d", w.Code)
	}
}

func TestHandleSendChat(t *testing.T) {
	srv := testServer(t)

	body := `{"to": "shuri", "body": "Build the dashboard", "priority": "P1"}`
	req := httptest.NewRequest("POST", "/api/team/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handleSendChat(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var msg ChatMessage
	if err := json.Unmarshal(w.Body.Bytes(), &msg); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	if msg.To != "shuri" {
		t.Errorf("expected to=shuri, got %s", msg.To)
	}
	if msg.Priority != "P1" {
		t.Errorf("expected priority P1, got %s", msg.Priority)
	}
}

func TestHandleSendChat_InvalidBody(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("POST", "/api/team/chat", strings.NewReader("not json"))
	w := httptest.NewRecorder()

	srv.handleSendChat(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleSendChat_MissingRecipient(t *testing.T) {
	srv := testServer(t)

	body := `{"body": "Hello"}`
	req := httptest.NewRequest("POST", "/api/team/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handleSendChat(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 (validation error), got %d", w.Code)
	}
}

func TestHandleChatHistory(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/team/chat/history?agent=shuri&limit=10", nil)
	w := httptest.NewRecorder()

	srv.handleChatHistory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var messages []ChatMessage
	if err := json.Unmarshal(w.Body.Bytes(), &messages); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	// Empty is fine for fresh install.
}

func TestHandleRestartAgent_NoConfirm(t *testing.T) {
	srv := testServer(t)

	body := `{"confirm": false}`
	req := httptest.NewRequest("POST", "/api/team/agents/shuri/restart", strings.NewReader(body))
	req.SetPathValue("name", "shuri")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handleRestartAgent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 without confirm, got %d", w.Code)
	}
}

func TestHandleRestartAgent_WithConfirm(t *testing.T) {
	srv := testServer(t)

	body := `{"confirm": true}`
	req := httptest.NewRequest("POST", "/api/team/agents/shuri/restart", strings.NewReader(body))
	req.SetPathValue("name", "shuri")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handleRestartAgent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleRestartAgent_NotFound(t *testing.T) {
	srv := testServer(t)

	body := `{"confirm": true}`
	req := httptest.NewRequest("POST", "/api/team/agents/unknown/restart", strings.NewReader(body))
	req.SetPathValue("name", "unknown")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handleRestartAgent(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleCreateTask_InvalidBody(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("POST", "/api/team/tasks", strings.NewReader("not json"))
	w := httptest.NewRecorder()

	srv.handleCreateTask(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleUpdateTask_MissingID(t *testing.T) {
	srv := testServer(t)

	body := `{"status": "done"}`
	req := httptest.NewRequest("PATCH", "/api/team/tasks/", strings.NewReader(body))
	req.SetPathValue("id", "")
	w := httptest.NewRecorder()

	srv.handleUpdateTask(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestIsAllowedOrigin(t *testing.T) {
	tests := []struct {
		origin  string
		allowed bool
	}{
		{"http://localhost:3002", true},
		{"http://127.0.0.1:3002", true},
		{"http://192.168.0.128:3002", true},
		{"https://localhost:3002", true},
		{"http://evil.com", false},
		{"http://192.168.1.1:3002", false},
		{"http://titan.local:3002", true},
	}

	for _, tt := range tests {
		t.Run(tt.origin, func(t *testing.T) {
			if got := isAllowedOrigin(tt.origin); got != tt.allowed {
				t.Errorf("isAllowedOrigin(%q) = %v, want %v", tt.origin, got, tt.allowed)
			}
		})
	}
}

func TestCSPHeaders(t *testing.T) {
	srv := testServer(t)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/team/health", srv.handleHealth)
	handler := srv.withCSP(mux)

	req := httptest.NewRequest("GET", "/api/team/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("expected CSP header")
	}
	if !strings.Contains(csp, "default-src 'self'") {
		t.Error("CSP should contain default-src 'self'")
	}

	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("expected X-Content-Type-Options: nosniff")
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("expected X-Frame-Options: DENY")
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"hello": "world"})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected content-type application/json, got %s", ct)
	}

	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	if result["hello"] != "world" {
		t.Errorf("expected hello=world, got %s", result["hello"])
	}
}
