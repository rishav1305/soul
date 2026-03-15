package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/rishav1305/soul-v2/internal/scout/agent"
	"github.com/rishav1305/soul-v2/internal/scout/profiledb"
	"github.com/rishav1305/soul-v2/internal/scout/store"
	"github.com/rishav1305/soul-v2/internal/scout/sweep"
)

// Server is the scout HTTP server.
type Server struct {
	mux        *http.ServeMux
	httpServer *http.Server
	host       string
	port       int
	dataDir    string
	pgURL      string
	cdpURL     string
	startTime  time.Time
	store      *store.Store
	profileDB  *profiledb.Client
	cdpClient  *sweep.CDPClient
}

// Option configures the Server.
type Option func(*Server)

// WithHost sets the bind address.
func WithHost(h string) Option { return func(s *Server) { s.host = h } }

// WithPort sets the listen port.
func WithPort(p int) Option { return func(s *Server) { s.port = p } }

// WithDataDir sets the data directory for SQLite databases.
func WithDataDir(d string) Option { return func(s *Server) { s.dataDir = d } }

// WithPgURL sets the PostgreSQL connection string for profiledb.
func WithPgURL(u string) Option { return func(s *Server) { s.pgURL = u } }

// WithCdpURL sets the Chrome DevTools Protocol endpoint.
func WithCdpURL(u string) Option { return func(s *Server) { s.cdpURL = u } }

// New creates a new scout Server.
func New(opts ...Option) *Server {
	s := &Server{
		mux:       http.NewServeMux(),
		host:      "127.0.0.1",
		port:      3020,
		startTime: time.Now(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Start initializes the store, optional services, and begins listening.
func (s *Server) Start() error {
	// Open SQLite store.
	dbPath := filepath.Join(s.dataDir, "scout.db")
	st, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("scout server: open store: %w", err)
	}
	s.store = st

	// Optional: connect to profile PostgreSQL.
	if s.pgURL != "" {
		client, err := profiledb.New(s.pgURL)
		if err != nil {
			log.Printf("scout: profiledb unavailable: %v", err)
		} else {
			s.profileDB = client
			log.Printf("scout: profiledb connected")
		}
	}

	// Optional: set up CDP client.
	if s.cdpURL != "" {
		s.cdpClient = sweep.NewCDPClient(s.cdpURL)
		if s.cdpClient.Available() {
			log.Printf("scout: CDP available at %s", s.cdpURL)
		} else {
			log.Printf("scout: CDP not reachable at %s", s.cdpURL)
		}
	}

	// Register routes.
	s.registerRoutes()

	// Build middleware chain.
	handler := http.Handler(s.mux)
	handler = corsMiddleware(handler)
	handler = recoveryMiddleware(handler)

	s.httpServer = &http.Server{
		Addr:              net.JoinHostPort(s.host, strconv.Itoa(s.port)),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("soul-scout listening on http://%s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server and closes resources.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.profileDB != nil {
		s.profileDB.Close()
	}
	if s.store != nil {
		s.store.Close()
	}
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// ServeHTTP implements http.Handler for testing.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) registerRoutes() {
	// Health.
	s.mux.HandleFunc("GET /api/health", s.handleHealth)

	// Leads.
	s.mux.HandleFunc("POST /api/leads", s.handleAddLead)
	s.mux.HandleFunc("GET /api/leads", s.handleListLeads)
	s.mux.HandleFunc("GET /api/leads/scored", s.handleScoredLeads)
	s.mux.HandleFunc("GET /api/leads/{id}", s.handleGetLead)
	s.mux.HandleFunc("PATCH /api/leads/{id}", s.handleUpdateLead)
	s.mux.HandleFunc("POST /api/leads/{id}/action", s.handleRecordAction)

	// Analytics.
	s.mux.HandleFunc("GET /api/analytics", s.handleAnalytics)

	// Sync.
	s.mux.HandleFunc("POST /api/sync", s.handleSync)

	// Sweep.
	s.mux.HandleFunc("POST /api/sweep", s.handleSweep)
	s.mux.HandleFunc("POST /api/sweep/now", s.handleSweepNow)
	s.mux.HandleFunc("GET /api/sweep/status", s.handleSweepStatus)
	s.mux.HandleFunc("GET /api/sweep/digest", s.handleSweepDigest)

	// Profile.
	s.mux.HandleFunc("GET /api/profile", s.handleGetProfile)
	s.mux.HandleFunc("POST /api/profile/pull", s.handleProfilePull)
	s.mux.HandleFunc("POST /api/profile/push", s.handleProfilePush)

	// Optimizations.
	s.mux.HandleFunc("POST /api/optimizations", s.handleAddOptimization)
	s.mux.HandleFunc("GET /api/optimizations", s.handleListOptimizations)
	s.mux.HandleFunc("POST /api/optimize", s.handleLaunchOptimizer)
	s.mux.HandleFunc("POST /api/optimize/apply", s.handleApplyOptimization)

	// Agent.
	s.mux.HandleFunc("GET /api/agent/status", s.handleAgentStatus)
	s.mux.HandleFunc("GET /api/agent/history", s.handleAgentHistory)

	// Tool dispatch.
	s.mux.HandleFunc("POST /api/tools/{name}/execute", s.handleToolExecute)
}

// --- Middleware ---

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic recovered: %v", err)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://127.0.0.1:3002")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeBody(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// --- Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"service":   "soul-scout",
		"uptime":    time.Since(s.startTime).String(),
		"profiledb": s.profileDB != nil,
		"cdp":       s.cdpClient != nil && s.cdpClient.Available(),
	})
}

// --- Leads ---

func (s *Server) handleAddLead(w http.ResponseWriter, r *http.Request) {
	var lead store.Lead
	if err := decodeBody(r, &lead); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if lead.Title == "" || lead.Type == "" {
		writeError(w, http.StatusBadRequest, "title and type are required")
		return
	}
	id, err := s.store.AddLead(lead)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	saved, err := s.store.GetLead(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, saved)
}

func (s *Server) handleListLeads(w http.ResponseWriter, r *http.Request) {
	typeFilter := r.URL.Query().Get("type")
	activeOnly := r.URL.Query().Get("active_only") == "true"
	leads, err := s.store.ListLeads(typeFilter, activeOnly)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if leads == nil {
		leads = []store.Lead{}
	}
	writeJSON(w, http.StatusOK, leads)
}

func (s *Server) handleGetLead(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid lead ID")
		return
	}
	lead, err := s.store.GetLead(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, lead)
}

func (s *Server) handleUpdateLead(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid lead ID")
		return
	}
	var fields map[string]interface{}
	if err := decodeBody(r, &fields); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := s.store.UpdateLead(id, fields); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	lead, err := s.store.GetLead(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, lead)
}

func (s *Server) handleScoredLeads(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	leads, err := s.store.ScoredLeads(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if leads == nil {
		leads = []store.Lead{}
	}
	writeJSON(w, http.StatusOK, leads)
}

func (s *Server) handleRecordAction(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid lead ID")
		return
	}
	var body struct {
		Action string `json:"action"`
		Notes  string `json:"notes"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// Get current lead to record stage transition if action is a stage change.
	lead, err := s.store.GetLead(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if body.Action != "" && body.Action != lead.Stage {
		if err := s.store.RecordStageHistory(id, lead.Stage, body.Action, body.Notes); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := s.store.UpdateLead(id, map[string]interface{}{"stage": body.Action}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	updated, _ := s.store.GetLead(id)
	writeJSON(w, http.StatusOK, updated)
}

// --- Analytics ---

func (s *Server) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	typeFilter := r.URL.Query().Get("type")
	analytics, err := s.store.GetAnalytics(typeFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, analytics)
}

// --- Sync ---

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Platform string `json:"platform"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	// Stub: record a sync check.
	id, err := s.store.AddSyncResult(store.SyncResult{
		Platform: body.Platform,
		Status:   "checked",
		Details:  "sync check recorded — actual platform verification deferred",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":       id,
		"platform": body.Platform,
		"status":   "checked",
	})
}

// --- Sweep ---

func (s *Server) handleSweep(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Platforms []string `json:"platforms"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if len(body.Platforms) == 0 {
		body.Platforms = []string{"linkedin", "indeed", "upwork", "toptal", "wellfound"}
	}
	results, err := sweep.Sweep(body.Platforms, s.store)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleSweepNow(w http.ResponseWriter, r *http.Request) {
	platforms := []string{"linkedin", "indeed", "upwork", "toptal", "wellfound"}
	results, err := sweep.Sweep(platforms, s.store)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleSweepStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, sweep.SweepStatus())
}

func (s *Server) handleSweepDigest(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"period":    "weekly",
		"newLeads":  0,
		"applied":   0,
		"responses": 0,
		"note":      "digest generation deferred — requires sweep history",
	})
}

// --- Profile ---

func (s *Server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	if s.profileDB == nil {
		writeError(w, http.StatusServiceUnavailable, "profiledb not configured — set SOUL_SCOUT_PG_URL")
		return
	}
	section := r.URL.Query().Get("section")
	if section != "" {
		data, err := s.profileDB.GetSection(section)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, data)
		return
	}
	profile, err := s.profileDB.GetFullProfile()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) handleProfilePull(w http.ResponseWriter, r *http.Request) {
	if s.profileDB == nil {
		writeError(w, http.StatusServiceUnavailable, "profiledb not configured — set SOUL_SCOUT_PG_URL")
		return
	}
	profile, err := s.profileDB.GetFullProfile()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "pulled",
		"profile": profile,
	})
}

func (s *Server) handleProfilePush(w http.ResponseWriter, r *http.Request) {
	if s.profileDB == nil {
		writeError(w, http.StatusServiceUnavailable, "profiledb not configured — set SOUL_SCOUT_PG_URL")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "push deferred — write operations require explicit field mapping",
	})
}

// --- Optimizations ---

func (s *Server) handleAddOptimization(w http.ResponseWriter, r *http.Request) {
	var opt store.Optimization
	if err := decodeBody(r, &opt); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	id, err := s.store.AddOptimization(opt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"id": id, "status": "created"})
}

func (s *Server) handleListOptimizations(w http.ResponseWriter, r *http.Request) {
	opts, err := s.store.ListOptimizations()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if opts == nil {
		opts = []store.Optimization{}
	}
	writeJSON(w, http.StatusOK, opts)
}

func (s *Server) handleLaunchOptimizer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Platform string `json:"platform"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if body.Platform == "" {
		writeError(w, http.StatusBadRequest, "platform is required")
		return
	}
	run, err := agent.LaunchOptimizer(body.Platform, s.store)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, run)
}

func (s *Server) handleApplyOptimization(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "apply deferred — requires CDP and agent approval workflow",
	})
}

// --- Agent ---

func (s *Server) handleAgentStatus(w http.ResponseWriter, r *http.Request) {
	runIDStr := r.URL.Query().Get("run_id")
	if runIDStr == "" {
		writeError(w, http.StatusBadRequest, "run_id query parameter is required")
		return
	}
	runID, err := strconv.ParseInt(runIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid run_id")
		return
	}
	run, err := s.store.GetAgentRun(runID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) handleAgentHistory(w http.ResponseWriter, r *http.Request) {
	platform := r.URL.Query().Get("platform")
	runs, err := s.store.ListAgentRuns(platform)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if runs == nil {
		runs = []store.AgentRun{}
	}
	writeJSON(w, http.StatusOK, runs)
}

// --- Tool Dispatch ---

func (s *Server) handleToolExecute(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var input map[string]interface{}
	if err := decodeBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	switch name {
	case "list_leads":
		typeFilter, _ := input["type"].(string)
		activeOnly, _ := input["active_only"].(bool)
		leads, err := s.store.ListLeads(typeFilter, activeOnly)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if leads == nil {
			leads = []store.Lead{}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"leads": leads})

	case "get_lead":
		idFloat, _ := input["id"].(float64)
		lead, err := s.store.GetLead(int64(idFloat))
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, lead)

	case "add_lead":
		data, _ := json.Marshal(input)
		var lead store.Lead
		json.Unmarshal(data, &lead)
		id, err := s.store.AddLead(lead)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]interface{}{"id": id})

	case "analytics":
		typeFilter, _ := input["type"].(string)
		analytics, err := s.store.GetAnalytics(typeFilter)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, analytics)

	case "sweep_status":
		writeJSON(w, http.StatusOK, sweep.SweepStatus())

	case "scored_leads":
		limit := 20
		if l, ok := input["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}
		leads, err := s.store.ScoredLeads(limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if leads == nil {
			leads = []store.Lead{}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"leads": leads})

	default:
		writeError(w, http.StatusNotFound, fmt.Sprintf("unknown tool: %s", name))
	}
}
