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
	"strings"
	"time"

	"github.com/rishav1305/soul-v2/internal/scout/ai"
	"github.com/rishav1305/soul-v2/internal/scout/pipelines"
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
	startTime  time.Time
	store      *store.Store
	profileDB  *profiledb.Client
	aiService  *ai.Service
	scheduler  *sweep.Scheduler
	configPath string
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

// WithStore sets an existing store instance (skips opening in Start).
func WithStore(st *store.Store) Option { return func(s *Server) { s.store = st } }

// WithAIService sets the AI service for AI-powered tool handlers.
func WithAIService(svc *ai.Service) Option { return func(s *Server) { s.aiService = svc } }

// WithProfileDB sets the profiledb client (avoids server creating a duplicate).
func WithProfileDB(pdb *profiledb.Client) Option { return func(s *Server) { s.profileDB = pdb } }

// WithScheduler sets the sweep scheduler.
func WithScheduler(sch *sweep.Scheduler) Option { return func(s *Server) { s.scheduler = sch } }

// WithConfigPath sets the path to the sweep config file.
func WithConfigPath(p string) Option { return func(s *Server) { s.configPath = p } }

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
	// Open SQLite store if not already provided via WithStore.
	if s.store == nil {
		dbPath := filepath.Join(s.dataDir, "scout.db")
		st, err := store.Open(dbPath)
		if err != nil {
			return fmt.Errorf("scout server: open store: %w", err)
		}
		s.store = st
	}

	// Optional: connect to profile PostgreSQL (skip if already injected via main).
	if s.profileDB == nil && s.pgURL != "" {
		client, err := profiledb.New(s.pgURL)
		if err != nil {
			log.Printf("scout: profiledb unavailable: %v", err)
		} else {
			s.profileDB = client
			log.Printf("scout: profiledb connected")
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
// Stops scheduler, drains HTTP, then closes DB/profiledb.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.scheduler != nil {
		s.scheduler.Stop()
	}
	var err error
	if s.httpServer != nil {
		err = s.httpServer.Shutdown(ctx)
	}
	if s.profileDB != nil {
		s.profileDB.Close()
	}
	if s.store != nil {
		s.store.Close()
	}
	return err
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

	// Sweep config.
	s.mux.HandleFunc("GET /api/sweep/config", s.handleGetSweepConfig)
	s.mux.HandleFunc("PUT /api/sweep/config", s.handlePutSweepConfig)

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

	// AI tools.
	s.mux.HandleFunc("POST /api/ai/match", s.handleAIMatch)
	s.mux.HandleFunc("POST /api/ai/proposal", s.handleAIProposal)
	s.mux.HandleFunc("POST /api/ai/cover-letter", s.handleAICoverLetter)
	s.mux.HandleFunc("POST /api/ai/outreach", s.handleAIOutreach)
	s.mux.HandleFunc("POST /api/ai/salary", s.handleAISalary)
	s.mux.HandleFunc("POST /api/ai/referral", s.handleAIReferral)
	s.mux.HandleFunc("POST /api/ai/pitch", s.handleAIPitch)

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
		origin := r.Header.Get("Origin")
		allowed := map[string]bool{
			"http://127.0.0.1:3002": true,
			"http://localhost:3002": true,
			"http://127.0.0.1:5173": true, // vite dev
			"http://localhost:5173": true,
		}
		w.Header().Set("Vary", "Origin")
		if origin == "" || !allowed[origin] {
			// No CORS headers for disallowed/missing origins.
			// Non-browser clients (curl, server-to-server) are unaffected.
			// Browsers will block the response due to missing ACAO header.
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, OPTIONS")
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

// asyncErrorStatus maps async launch errors to appropriate HTTP status codes.
func asyncErrorStatus(err error) int {
	msg := err.Error()
	if strings.Contains(msg, "queue full") || strings.Contains(msg, "max 3 concurrent") {
		return http.StatusServiceUnavailable // 503 — retryable
	}
	if strings.Contains(msg, "not found") || strings.Contains(msg, "no rows") {
		return http.StatusNotFound // 404 — bad lead_id
	}
	return http.StatusInternalServerError
}

// emptyDigest returns the zero-value digest response shape.
func emptyDigest() map[string]interface{} {
	return map[string]interface{}{
		"last_run":           "",
		"next_run":           "",
		"new_leads":          0,
		"duplicates":         0,
		"high_matches":       0,
		"high_match_leads":   []interface{}{},
		"score_distribution": map[string]int{},
	}
}

// aiErrorStatus maps AI service errors to appropriate HTTP status codes.
func aiErrorStatus(err error) int {
	msg := err.Error()
	if strings.Contains(msg, "not found") || strings.Contains(msg, "no rows") {
		return http.StatusNotFound
	}
	if strings.Contains(msg, "invalid platform") || strings.Contains(msg, "profiledb not configured") {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

// parseID extracts an integer ID from tool input, handling both float64 (JSON number)
// and string representations. Returns 0 if key is missing or unparseable.
func parseID(input map[string]interface{}, key string) int64 {
	v, ok := input[key]
	if !ok {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return int64(val)
	case string:
		id, _ := strconv.ParseInt(val, 10, 64)
		return id
	case json.Number:
		id, _ := val.Int64()
		return id
	default:
		return 0
	}
}

// --- Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"service":   "soul-scout",
		"uptime":    time.Since(s.startTime).String(),
		"profiledb": s.profileDB != nil,
		"scheduler": s.scheduler != nil,
	})
}

// --- Leads ---

func (s *Server) handleAddLead(w http.ResponseWriter, r *http.Request) {
	var lead store.Lead
	if err := decodeBody(r, &lead); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if lead.JobTitle == "" && lead.Company == "" {
		writeError(w, http.StatusBadRequest, "job_title or company is required")
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
	pipelineFilter := r.URL.Query().Get("pipeline")
	activeOnly := r.URL.Query().Get("active_only") == "true"
	leads, err := s.store.ListLeads(pipelineFilter, activeOnly)
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
		if lead.Pipeline != "" {
			if err := pipelines.ValidateTransition(lead.Pipeline, lead.Stage, body.Action); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		if err := s.store.RecordStageHistory(id, lead.Stage, body.Action, body.Notes); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := s.store.UpdateLead(id, map[string]interface{}{"stage": body.Action}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	updated, err := s.store.GetLead(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// --- Analytics ---

func (s *Server) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	pipelineFilter := r.URL.Query().Get("pipeline")
	analytics, err := s.store.GetAnalytics(pipelineFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Transform to match frontend ScoutAnalytics shape.
	var totalLeads int
	for _, c := range analytics.Stats.ByType {
		totalLeads += c
	}

	// Flatten conversion funnels into [{stage, count, rate}].
	var conversion []map[string]interface{}
	for _, f := range analytics.Conversion.Funnels {
		for _, step := range f.Steps {
			rate := 0.0
			if totalLeads > 0 {
				rate = float64(step.Count) / float64(totalLeads) * 100
			}
			conversion = append(conversion, map[string]interface{}{
				"stage": step.Stage,
				"count": step.Count,
				"rate":  rate,
			})
		}
	}
	if conversion == nil {
		conversion = []map[string]interface{}{}
	}

	// Build weekly_trend from stage_history (last 8 weeks).
	weeklyTrend := s.buildWeeklyTrend()

	resp := map[string]interface{}{
		"by_type":      analytics.Stats.ByType,
		"by_source":    analytics.Stats.BySource,
		"by_stage":     analytics.Stats.ByStage,
		"total_leads":  totalLeads,
		"active_leads": analytics.Stats.Active,
		"conversion":   conversion,
		"weekly_trend": weeklyTrend,
	}
	writeJSON(w, http.StatusOK, resp)
}

// buildWeeklyTrend returns lead creation counts per week for the last 8 weeks.
func (s *Server) buildWeeklyTrend() []map[string]interface{} {
	var trend []map[string]interface{}
	now := time.Now().UTC()
	for i := 7; i >= 0; i-- {
		weekStart := now.AddDate(0, 0, -7*i)
		weekEnd := weekStart.AddDate(0, 0, 7)
		label := weekStart.Format("Jan 2")
		var count int
		_ = s.store.DB().QueryRow(
			"SELECT COUNT(*) FROM leads WHERE created_at >= ? AND created_at < ?",
			weekStart.Format(time.RFC3339), weekEnd.Format(time.RFC3339),
		).Scan(&count)
		trend = append(trend, map[string]interface{}{
			"week":  label,
			"count": count,
		})
	}
	return trend
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
	// Use scheduler for real TheirStack sweep if available
	if s.scheduler != nil {
		runID, started := s.scheduler.RunNow()
		if started {
			writeJSON(w, http.StatusAccepted, map[string]interface{}{"run_id": runID, "status": "started"})
		} else {
			writeJSON(w, http.StatusConflict, map[string]interface{}{"status": "already_running"})
		}
		return
	}
	writeError(w, http.StatusServiceUnavailable, "scheduler not configured — set SOUL_SCOUT_THEIRSTACK_KEY")
}

func (s *Server) handleSweepNow(w http.ResponseWriter, r *http.Request) {
	if s.scheduler != nil {
		runID, started := s.scheduler.RunNow()
		if started {
			writeJSON(w, http.StatusAccepted, map[string]interface{}{
				"run_id": runID,
				"status": "started",
			})
		} else {
			writeJSON(w, http.StatusConflict, map[string]interface{}{
				"status": "already_running",
			})
		}
		return
	}
	writeError(w, http.StatusServiceUnavailable, "scheduler not configured")
}

func (s *Server) handleSweepStatus(w http.ResponseWriter, r *http.Request) {
	if s.scheduler != nil {
		writeJSON(w, http.StatusOK, s.scheduler.Status())
		return
	}
	writeJSON(w, http.StatusOK, sweep.SweepStatus())
}

func (s *Server) handleSweepDigest(w http.ResponseWriter, r *http.Request) {
	val, err := s.store.GetSyncMeta("sweep_last_digest")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "read digest: "+err.Error())
		return
	}
	if val == "" {
		writeJSON(w, http.StatusOK, emptyDigest())
		return
	}
	var digest map[string]interface{}
	if err := json.Unmarshal([]byte(val), &digest); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse digest: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, digest)
}

// --- Sweep Config ---

func (s *Server) handleGetSweepConfig(w http.ResponseWriter, r *http.Request) {
	if s.configPath == "" {
		writeError(w, http.StatusServiceUnavailable, "sweep config path not set")
		return
	}
	cfg, err := sweep.LoadConfig(s.configPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handlePutSweepConfig(w http.ResponseWriter, r *http.Request) {
	if s.configPath == "" {
		writeError(w, http.StatusServiceUnavailable, "sweep config path not set")
		return
	}
	var cfg sweep.SweepConfig
	if err := decodeBody(r, &cfg); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Validate and backfill critical fields before saving
	if cfg.IntervalHours <= 0 {
		writeError(w, http.StatusBadRequest, "interval_hours must be > 0")
		return
	}
	if cfg.Limit <= 0 {
		writeError(w, http.StatusBadRequest, "limit must be > 0")
		return
	}
	if cfg.CreditBudget <= 0 {
		writeError(w, http.StatusBadRequest, "credit_budget must be > 0")
		return
	}
	// TheirStack requires at least one of: posted_at_max_age_days, posted_at_gte/lte,
	// company_domain_or, company_linkedin_url_or, company_name_or
	if cfg.PostedAtMaxAgeDays <= 0 {
		writeError(w, http.StatusBadRequest, "posted_at_max_age_days must be > 0 (TheirStack API requires a date filter)")
		return
	}
	if err := sweep.SaveConfig(s.configPath, &cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Update live scheduler config if running
	if s.scheduler != nil {
		s.scheduler.UpdateConfig(&cfg)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
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
	raw, err := s.profileDB.GetFullProfile()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Transform raw profiledb rows into frontend ScoutProfile shape.
	writeJSON(w, http.StatusOK, transformProfile(raw))
}

// transformProfile reshapes raw profiledb data into the frontend ScoutProfile type.
func transformProfile(raw map[string]interface{}) map[string]interface{} {
	profile := map[string]interface{}{
		"experience":     []interface{}{},
		"projects":       []interface{}{},
		"skills":         []string{},
		"education":      []interface{}{},
		"certifications": []interface{}{},
	}

	// Experience: role->title, company->company, period->duration.
	if rows, ok := raw["experience"].([]map[string]interface{}); ok {
		var out []map[string]string
		for _, r := range rows {
			out = append(out, map[string]string{
				"title":       strVal(r, "role"),
				"company":     strVal(r, "company"),
				"duration":    strVal(r, "period"),
				"description": strVal(r, "description"),
			})
		}
		profile["experience"] = out
	}

	// Projects: title->name, short_description->description, link->url.
	if rows, ok := raw["projects"].([]map[string]interface{}); ok {
		var out []map[string]string
		for _, r := range rows {
			out = append(out, map[string]string{
				"name":        strVal(r, "title"),
				"description": strVal(r, "short_description"),
				"url":         strVal(r, "link"),
			})
		}
		profile["projects"] = out
	}

	// Skills: flatten skill_categories into a string slice.
	// Each row has a "skills" field that pgx parses as []interface{} of maps with "name" and "level".
	if rows, ok := raw["skill_categories"].([]map[string]interface{}); ok {
		var skills []string
		for _, r := range rows {
			if arr, ok := r["skills"].([]interface{}); ok {
				for _, item := range arr {
					if m, ok := item.(map[string]interface{}); ok {
						if name := strVal(m, "name"); name != "" {
							skills = append(skills, name)
						}
					}
				}
			}
		}
		profile["skills"] = skills
	}

	// Education: degree, institution, period->year.
	if rows, ok := raw["education"].([]map[string]interface{}); ok {
		var out []map[string]string
		for _, r := range rows {
			out = append(out, map[string]string{
				"degree":      strVal(r, "degree"),
				"institution": strVal(r, "institution"),
				"year":        strVal(r, "period"),
			})
		}
		profile["education"] = out
	}

	// Certifications: skip if error object from missing table.
	if rows, ok := raw["certifications"].([]map[string]interface{}); ok {
		var out []map[string]string
		for _, r := range rows {
			out = append(out, map[string]string{
				"name":   strVal(r, "name"),
				"issuer": strVal(r, "issuer"),
				"year":   strVal(r, "year"),
			})
		}
		profile["certifications"] = out
	}

	return profile
}

// strVal safely extracts a string from a map value.
func strVal(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok && v != nil {
		return fmt.Sprintf("%v", v)
	}
	return ""
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
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Platform == "" {
		writeError(w, http.StatusBadRequest, "platform is required")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "optimizer launch deferred — use AI tools instead",
	})
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
		latest, _ := s.store.LatestAgentRun()
		if latest == nil {
			writeJSON(w, http.StatusOK, &store.AgentRun{Status: "no_runs"})
			return
		}
		writeJSON(w, http.StatusOK, latest)
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

// --- AI Tools ---

func (s *Server) handleAIMatch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LeadID int64 `json:"lead_id"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.LeadID <= 0 {
		writeError(w, http.StatusBadRequest, "lead_id is required")
		return
	}
	if s.aiService == nil {
		writeError(w, http.StatusServiceUnavailable, "AI service not configured")
		return
	}
	result, err := s.aiService.ResumeMatch(r.Context(), body.LeadID)
	if err != nil {
		writeError(w, aiErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAIProposal(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LeadID   int64  `json:"lead_id"`
		Platform string `json:"platform"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.LeadID <= 0 {
		writeError(w, http.StatusBadRequest, "lead_id is required")
		return
	}
	if s.aiService == nil {
		writeError(w, http.StatusServiceUnavailable, "AI service not configured")
		return
	}
	result, err := s.aiService.ProposalGen(r.Context(), body.LeadID, body.Platform)
	if err != nil {
		writeError(w, aiErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAICoverLetter(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LeadID int64 `json:"lead_id"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.LeadID <= 0 {
		writeError(w, http.StatusBadRequest, "lead_id is required")
		return
	}
	if s.aiService == nil {
		writeError(w, http.StatusServiceUnavailable, "AI service not configured")
		return
	}
	result, err := s.aiService.CoverLetter(r.Context(), body.LeadID)
	if err != nil {
		writeError(w, aiErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAIOutreach(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LeadID int64 `json:"lead_id"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.LeadID <= 0 {
		writeError(w, http.StatusBadRequest, "lead_id is required")
		return
	}
	if s.aiService == nil {
		writeError(w, http.StatusServiceUnavailable, "AI service not configured")
		return
	}
	result, err := s.aiService.ColdOutreach(r.Context(), body.LeadID)
	if err != nil {
		writeError(w, aiErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAISalary(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LeadID int64 `json:"lead_id"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.LeadID <= 0 {
		writeError(w, http.StatusBadRequest, "lead_id is required")
		return
	}
	if s.aiService == nil {
		writeError(w, http.StatusServiceUnavailable, "AI service not configured")
		return
	}
	result, err := s.aiService.SalaryLookup(r.Context(), body.LeadID)
	if err != nil {
		writeError(w, aiErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAIReferral(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LeadID int64 `json:"lead_id"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.LeadID <= 0 {
		writeError(w, http.StatusBadRequest, "lead_id is required")
		return
	}
	if s.aiService == nil {
		writeError(w, http.StatusServiceUnavailable, "AI service not configured")
		return
	}
	runID, err := s.aiService.ReferralFinder(r.Context(), body.LeadID)
	if err != nil {
		writeError(w, asyncErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"run_id": runID, "status": "running"})
}

func (s *Server) handleAIPitch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LeadID int64 `json:"lead_id"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.LeadID <= 0 {
		writeError(w, http.StatusBadRequest, "lead_id is required")
		return
	}
	if s.aiService == nil {
		writeError(w, http.StatusServiceUnavailable, "AI service not configured")
		return
	}
	runID, err := s.aiService.CompanyPitch(r.Context(), body.LeadID)
	if err != nil {
		writeError(w, asyncErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"run_id": runID, "status": "running"})
}

// --- Tool Dispatch ---

func (s *Server) handleToolExecute(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var input map[string]interface{}
	if r.Body != nil && r.ContentLength != 0 {
		if err := decodeBody(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
	}
	if input == nil {
		input = map[string]interface{}{}
	}

	switch name {
	case "lead_list":
		pipelineFilter, _ := input["pipeline"].(string)
		activeOnly, _ := input["active_only"].(bool)
		leads, err := s.store.ListLeads(pipelineFilter, activeOnly)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if leads == nil {
			leads = []store.Lead{}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"leads": leads})

	case "lead_get":
		leadID := parseID(input, "lead_id")
		if leadID == 0 {
			writeError(w, http.StatusBadRequest, "lead_id is required")
			return
		}
		lead, err := s.store.GetLead(leadID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, lead)

	case "lead_add":
		data, _ := json.Marshal(input)
		var lead store.Lead
		if err := json.Unmarshal(data, &lead); err != nil {
			writeError(w, http.StatusBadRequest, "invalid lead data: "+err.Error())
			return
		}
		id, err := s.store.AddLead(lead)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]interface{}{"id": id})

	case "lead_update":
		leadID := parseID(input, "lead_id")
		if leadID == 0 {
			writeError(w, http.StatusBadRequest, "lead_id is required")
			return
		}
		delete(input, "lead_id")
		if len(input) == 0 {
			writeError(w, http.StatusBadRequest, "no fields to update")
			return
		}
		if err := s.store.UpdateLead(leadID, input); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		updated, err := s.store.GetLead(leadID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, updated)

	case "lead_action":
		leadID := parseID(input, "lead_id")
		if leadID == 0 {
			writeError(w, http.StatusBadRequest, "lead_id is required")
			return
		}
		action, _ := input["action"].(string)
		notes, _ := input["notes"].(string)
		lead, err := s.store.GetLead(leadID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if action != "" && action != lead.Stage {
			if lead.Pipeline != "" {
				if err := pipelines.ValidateTransition(lead.Pipeline, lead.Stage, action); err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
			}
			if err := s.store.RecordStageHistory(leadID, lead.Stage, action, notes); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if err := s.store.UpdateLead(leadID, map[string]interface{}{"stage": action}); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		updated, err := s.store.GetLead(leadID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, updated)

	case "analytics":
		pipelineFilter, _ := input["pipeline"].(string)
		analytics, err := s.store.GetAnalytics(pipelineFilter)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, analytics)

	case "sync":
		platform, _ := input["platform"].(string)
		id, err := s.store.AddSyncResult(store.SyncResult{Platform: platform, Status: "checked"})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"id": id, "status": "checked"})

	case "sweep", "sweep_now":
		if s.scheduler != nil {
			runID, started := s.scheduler.RunNow()
			if started {
				writeJSON(w, http.StatusAccepted, map[string]interface{}{"run_id": runID, "status": "started"})
			} else {
				writeJSON(w, http.StatusConflict, map[string]interface{}{"status": "already_running"})
			}
		} else {
			writeError(w, http.StatusServiceUnavailable, "scheduler not configured — set SOUL_SCOUT_THEIRSTACK_KEY")
		}

	case "sweep_status":
		if s.scheduler != nil {
			writeJSON(w, http.StatusOK, s.scheduler.Status())
		} else {
			writeJSON(w, http.StatusOK, sweep.SweepStatus())
		}

	case "sweep_digest":
		digestJSON, err := s.store.GetSyncMeta("sweep_last_digest")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "read digest: "+err.Error())
			return
		}
		if digestJSON != "" {
			var digest map[string]interface{}
			if err := json.Unmarshal([]byte(digestJSON), &digest); err != nil {
				writeError(w, http.StatusInternalServerError, "parse digest: "+err.Error())
				return
			}
			writeJSON(w, http.StatusOK, digest)
		} else {
			writeJSON(w, http.StatusOK, emptyDigest())
		}

	case "profile":
		if s.profileDB == nil {
			writeError(w, http.StatusServiceUnavailable, "profiledb not configured")
			return
		}
		section, _ := input["section"].(string)
		if section != "" {
			data, err := s.profileDB.GetSection(section)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, data)
		} else {
			raw, err := s.profileDB.GetFullProfile()
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, transformProfile(raw))
		}

	case "profile_pull":
		if s.profileDB == nil {
			writeError(w, http.StatusServiceUnavailable, "profiledb not configured")
			return
		}
		profile, err := s.profileDB.GetFullProfile()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"status": "pulled", "profile": profile})

	case "profile_push":
		writeJSON(w, http.StatusOK, map[string]string{"status": "push deferred"})

	case "optimization_add":
		data, _ := json.Marshal(input)
		var opt store.Optimization
		if err := json.Unmarshal(data, &opt); err != nil {
			writeError(w, http.StatusBadRequest, "invalid optimization data: "+err.Error())
			return
		}
		id, err := s.store.AddOptimization(opt)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]interface{}{"id": id})

	case "optimization_list":
		opts, err := s.store.ListOptimizations()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if opts == nil {
			opts = []store.Optimization{}
		}
		writeJSON(w, http.StatusOK, opts)

	case "optimize_profile":
		writeJSON(w, http.StatusOK, map[string]string{"status": "optimizer launch deferred — use AI tools instead"})

	case "optimize_apply":
		writeJSON(w, http.StatusOK, map[string]string{"status": "apply deferred"})

	case "agent_status":
		runID := parseID(input, "run_id")
		if runID == 0 {
			latest, _ := s.store.LatestAgentRun()
			if latest == nil {
				writeJSON(w, http.StatusOK, &store.AgentRun{Status: "no_runs"})
				return
			}
			writeJSON(w, http.StatusOK, latest)
			return
		}
		run, err := s.store.GetAgentRun(runID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, run)

	case "agent_history":
		platform, _ := input["platform"].(string)
		runs, err := s.store.ListAgentRuns(platform)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if runs == nil {
			runs = []store.AgentRun{}
		}
		writeJSON(w, http.StatusOK, runs)

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
