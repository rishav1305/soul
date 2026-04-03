package team

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"nhooyr.io/websocket"
)

// Server is the Soul Control HTTP/WebSocket server.
type Server struct {
	cfg       Config
	hub       *Hub
	poller    *PanePoller
	state     *AgentStateAggregator
	inbox     *InboxBridge
	tasks     *TaskBridge
	httpSrv   *http.Server
	startTime time.Time
}

// NewServer creates a fully wired Soul Control server.
func NewServer(cfg Config) (*Server, error) {
	panes, err := LoadPanesMap(cfg.PanesFile)
	if err != nil {
		// Non-fatal: operate without pane streaming.
		log.Printf("team: could not load panes.json: %v (pane streaming disabled)", err)
		panes = make(map[string]string)
	}

	hub := NewHub()
	poller := NewPanePoller(panes, cfg.PaneBufferSize, cfg.PollIntervalMS, cfg.TmuxSession)
	state := NewAgentStateAggregator(cfg, panes)
	inbox := NewInboxBridge(cfg.ClawteamDir, cfg.TeamName)
	tasks := NewTaskBridge(cfg.BacklogCLI)

	// Wire pane deltas to WebSocket hub.
	poller.OnDelta(func(d PaneDelta) {
		hub.Broadcast(WSMessage{
			Type:    "pane." + d.Agent,
			Payload: d,
		})
	})

	return &Server{
		cfg:       cfg,
		hub:       hub,
		poller:    poller,
		state:     state,
		inbox:     inbox,
		tasks:     tasks,
		startTime: time.Now(),
	}, nil
}

// ListenAndServe starts the HTTP server and background pollers.
func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()

	// Middleware: reject empty agent name segments before ServeMux path-cleaning
	// converts double-slash to redirect. This catches /api/team/agents//pane etc.
	guardEmpty := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.RawPath, "//") || strings.Contains(r.RequestURI, "//") {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "empty path segment"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	// Health
	mux.HandleFunc("GET /api/team/health", s.handleHealth)

	// Agent state
	mux.HandleFunc("GET /api/team/agents", s.handleListAgents)
	mux.HandleFunc("GET /api/team/agents/{name}", s.handleGetAgent)
	mux.HandleFunc("GET /api/team/agents/{name}/pane", s.handleGetPane)
	mux.HandleFunc("POST /api/team/agents/{name}/restart", s.handleRestartAgent)

	// Tasks
	mux.HandleFunc("GET /api/team/tasks", s.handleListTasks)
	mux.HandleFunc("POST /api/team/tasks", s.handleCreateTask)
	mux.HandleFunc("PATCH /api/team/tasks/{id}", s.handleUpdateTask)

	// Chat
	mux.HandleFunc("POST /api/team/chat", s.handleSendChat)
	mux.HandleFunc("GET /api/team/chat/history", s.handleChatHistory)

	// WebSocket
	mux.HandleFunc("/ws/team", s.handleWebSocket)

	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	s.httpSrv = &http.Server{
		Addr:         addr,
		Handler:      guardEmpty(s.withCSP(mux)),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start background pollers.
	ctx := context.Background()
	go s.poller.Start(ctx)
	go s.stateRefreshLoop(ctx)

	log.Printf("soul-team server listening on %s", addr)
	return s.httpSrv.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.poller.Stop()
	return s.httpSrv.Shutdown(ctx)
}

// CSP middleware — adds security headers.
func (s *Server) withCSP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self' ws://localhost:* ws://192.168.0.*:* wss://localhost:* wss://192.168.0.*:* wss://*.titan.local:*; script-src 'self'; style-src 'self' 'unsafe-inline'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) stateRefreshLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Initial refresh.
	if err := s.state.Refresh(); err != nil {
		log.Printf("team: initial state refresh: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.state.Refresh(); err != nil {
				log.Printf("team: state refresh: %v", err)
				continue
			}
			// Broadcast state update.
			s.hub.BroadcastAll(WSMessage{
				Type:    "state.update",
				Payload: s.state.GetAll(),
			})
		}
	}
}

// --- Health ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	agents := s.state.GetAll()
	agentsUp := 0
	for _, a := range agents {
		if a.Status != "offline" {
			agentsUp++
		}
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	status := HealthStatus{
		Status:    "ok",
		Agents:    len(agents),
		AgentsUp:  agentsUp,
		Uptime:    time.Since(s.startTime).Truncate(time.Second).String(),
		StartedAt: s.startTime,
		Services:  map[string]string{"team-server": "ok", "pane-poller": "ok", "ws-hub": "ok"},
		MemoryMB:  int(memStats.Alloc / 1024 / 1024),
	}

	if agentsUp == 0 {
		status.Status = "degraded"
	}

	writeJSON(w, http.StatusOK, status)
}

// --- Agents ---

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	agents := s.state.GetAll()
	if agents == nil {
		agents = []AgentState{}
	}
	writeJSON(w, http.StatusOK, agents)
}

func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "agent name required"})
		return
	}

	agent, ok := s.state.Get(name)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("agent %q not found", name)})
		return
	}

	writeJSON(w, http.StatusOK, agent)
}

func (s *Server) handleGetPane(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "agent name required"})
		return
	}

	lines, ok := s.poller.GetBuffer(name)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("no pane data for agent %q", name)})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"agent": name,
		"lines": lines,
		"total": len(lines),
	})
}

func (s *Server) handleRestartAgent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "agent name required"})
		return
	}

	// Validate agent exists.
	if _, ok := s.state.Get(name); !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("agent %q not found", name)})
		return
	}

	// Parse confirmation from body.
	var body struct {
		Confirm bool `json:"confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !body.Confirm {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "confirmation required",
			"message": "Send {\"confirm\": true} to restart agent",
		})
		return
	}

	// Send restart command via the team inbox system (not direct kill).
	_, err := s.inbox.SendMessage(SendMessageRequest{
		To:       name,
		From:     "team-lead",
		Body:     fmt.Sprintf("SYSTEM: restart requested for agent %s via Soul Control", name),
		Priority: "P1",
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("send restart command: %v", err)})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "restart_requested",
		"agent":   name,
		"message": "Restart command sent to agent inbox. Guardian will handle the restart.",
	})
}

// --- Tasks ---

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	status := r.URL.Query().Get("status")

	if project == "" && status == "" {
		// Return full dashboard.
		dashboard, err := s.tasks.Dashboard(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, dashboard)
		return
	}

	tasks, err := s.tasks.ListTasks(r.Context(), project, status)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if tasks == nil {
		tasks = []TaskItem{}
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid body: %v", err)})
		return
	}

	if err := s.tasks.CreateTask(r.Context(), req); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Broadcast task update.
	s.hub.BroadcastAll(WSMessage{
		Type:    "task.update",
		Payload: map[string]string{"action": "created", "title": req.Title, "project": req.Project},
	})

	writeJSON(w, http.StatusCreated, map[string]string{"status": "created", "title": req.Title})
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "task id required"})
		return
	}

	var req UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid body: %v", err)})
		return
	}

	if err := s.tasks.UpdateTask(r.Context(), id, req); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Broadcast task update.
	s.hub.BroadcastAll(WSMessage{
		Type:    "task.update",
		Payload: map[string]string{"action": "updated", "id": id},
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated", "id": id})
}

// --- Chat ---

func (s *Server) handleSendChat(w http.ResponseWriter, r *http.Request) {
	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid body: %v", err)})
		return
	}

	msg, err := s.inbox.SendMessage(req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Broadcast chat message.
	s.hub.BroadcastAll(WSMessage{
		Type:    "chat.message",
		Payload: msg,
	})

	writeJSON(w, http.StatusCreated, msg)
}

func (s *Server) handleChatHistory(w http.ResponseWriter, r *http.Request) {
	agent := r.URL.Query().Get("agent")
	if agent == "" {
		agent = "team-lead"
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}

	messages, err := s.inbox.GetHistory(agent, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if messages == nil {
		messages = []ChatMessage{}
	}

	writeJSON(w, http.StatusOK, messages)
}

// --- WebSocket ---

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Origin validation: allow LAN origins and localhost.
	origin := r.Header.Get("Origin")
	if origin != "" && !isAllowedOrigin(origin) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{
			"localhost:*",
			"127.0.0.1:*",
			"192.168.0.*:*",
			"*.titan.local:*",
		},
	})
	if err != nil {
		log.Printf("team ws: accept error: %v", err)
		return
	}

	client := NewClient(conn, r.Context())

	// Default: subscribe to all.
	client.SubscribeAll()

	s.hub.Register(client)

	// Send initial state snapshot.
	initialState := s.state.GetAll()
	if data, err := json.Marshal(WSMessage{Type: "state.snapshot", Payload: initialState}); err == nil {
		select {
		case client.send <- data:
		default:
		}
	}

	go client.WritePump()
	client.ReadPump(s.hub) // Blocks until disconnect.
}

func isAllowedOrigin(origin string) bool {
	lower := strings.ToLower(origin)
	allowed := []string{
		"http://localhost",
		"http://127.0.0.1",
		"http://192.168.0.",
		"https://localhost",
		"https://127.0.0.1",
		"https://192.168.0.",
	}
	for _, prefix := range allowed {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	if strings.Contains(lower, "titan.local") {
		return true
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("team: write json: %v", err)
	}
}

// Getenv reads an env var with fallback. Exported for use in cmd/team/main.go.
func Getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
