package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/rishav1305/soul-v2/internal/chat/metrics"
	"github.com/rishav1305/soul-v2/internal/projects/content"
	"github.com/rishav1305/soul-v2/internal/projects/store"
)

// Server is the projects HTTP server.
type Server struct {
	store      *store.Store
	metrics    *metrics.EventLogger
	mux        *http.ServeMux
	httpServer *http.Server
	host       string
	port       string
	contentDir string
	startTime  time.Time
}

// Option configures the Server.
type Option func(*Server)

func WithStore(s *store.Store) Option    { return func(srv *Server) { srv.store = s } }
func WithHost(h string) Option           { return func(srv *Server) { srv.host = h } }
func WithPort(p string) Option           { return func(srv *Server) { srv.port = p } }
func WithContentDir(d string) Option     { return func(srv *Server) { srv.contentDir = d } }
func WithMetrics(l *metrics.EventLogger) Option { return func(srv *Server) { srv.metrics = l } }

// New creates a new projects Server.
func New(opts ...Option) *Server {
	s := &Server{
		mux:       http.NewServeMux(),
		host:      "127.0.0.1",
		port:      "3008",
		startTime: time.Now(),
	}
	for _, opt := range opts {
		opt(s)
	}

	// Register routes.
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/projects/dashboard", s.handleDashboard)
	s.mux.HandleFunc("GET /api/projects/keywords", s.handleKeywords)
	s.mux.HandleFunc("GET /api/projects/{id}", s.handleGetProject)
	s.mux.HandleFunc("PATCH /api/projects/{id}", s.handleUpdateProject)
	s.mux.HandleFunc("PATCH /api/projects/{id}/milestones/{mid}", s.handleUpdateMilestone)
	s.mux.HandleFunc("POST /api/projects/{id}/metrics", s.handleRecordMetric)
	s.mux.HandleFunc("POST /api/projects/{id}/syncs", s.handleSyncPlatform)
	s.mux.HandleFunc("POST /api/projects/{id}/readiness", s.handleRecordReadiness)
	s.mux.HandleFunc("GET /api/projects/{id}/guide", s.handleGetGuide)
	s.mux.HandleFunc("POST /api/tools/{name}/execute", s.handleToolExecute)

	// Build middleware chain.
	handler := http.Handler(s.mux)
	if s.metrics != nil {
		handler = requestLoggerMiddleware(s.metrics)(handler)
	} else {
		handler = requestLogMiddleware(handler)
	}
	handler = bodyLimitMiddleware(64 << 10)(handler)
	handler = cspMiddleware(handler)
	handler = recoveryMiddleware(handler)

	s.httpServer = &http.Server{
		Addr:              net.JoinHostPort(s.host, s.port),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return s
}

// ServeHTTP implements http.Handler for testing.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Start begins listening.
func (s *Server) Start() error {
	log.Printf("soul-projects listening on http://%s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// --- Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	count, _ := s.store.ProjectCount()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":        "ok",
		"project_count": count,
		"uptime":        time.Since(s.startTime).Round(time.Second).String(),
	})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	dashboard, err := s.store.GetDashboard()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if dashboard.Projects == nil {
		dashboard.Projects = []store.ProjectSummary{}
	}
	writeJSON(w, http.StatusOK, dashboard)
}

func (s *Server) handleKeywords(w http.ResponseWriter, r *http.Request) {
	keywords, err := s.store.ListKeywords()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if keywords == nil {
		keywords = []store.Keyword{}
	}
	writeJSON(w, http.StatusOK, keywords)
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	project, err := s.store.GetProject(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	milestones, err := s.store.ListMilestones(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if milestones == nil {
		milestones = []store.Milestone{}
	}

	metrics, err := s.store.ListMetrics(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if metrics == nil {
		metrics = []store.Metric{}
	}

	keywords, err := s.store.ListProjectKeywords(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if keywords == nil {
		keywords = []store.Keyword{}
	}

	syncs, err := s.store.ListProfileSyncs(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if syncs == nil {
		syncs = []store.ProfileSync{}
	}

	readiness, err := s.store.GetReadiness(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	detail := store.ProjectDetail{
		Project:    project,
		Milestones: milestones,
		Metrics:    metrics,
		Keywords:   keywords,
		Syncs:      syncs,
		Readiness:  readiness,
	}

	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	var update store.ProjectUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if err := s.store.UpdateProject(id, update); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	project, _ := s.store.GetProject(id)
	writeJSON(w, http.StatusOK, project)
}

func (s *Server) handleUpdateMilestone(w http.ResponseWriter, r *http.Request) {
	mid, err := parseID(r, "mid")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid milestone id")
		return
	}

	var input struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if input.Status == "" {
		writeError(w, http.StatusBadRequest, "status is required")
		return
	}

	if err := s.store.UpdateMilestoneStatus(mid, input.Status); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "milestone not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleRecordMetric(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	var input struct {
		Name  string `json:"name"`
		Value string `json:"value"`
		Unit  string `json:"unit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if input.Name == "" || input.Value == "" {
		writeError(w, http.StatusBadRequest, "name and value are required")
		return
	}

	metricID, err := s.store.RecordMetric(id, input.Name, input.Value, input.Unit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":   metricID,
		"name": input.Name,
	})
}

func (s *Server) handleSyncPlatform(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	var input struct {
		Platform string `json:"platform"`
		Notes    string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if input.Platform == "" {
		writeError(w, http.StatusBadRequest, "platform is required")
		return
	}

	if err := s.store.UpdateProfileSync(id, input.Platform, input.Notes); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "profile sync not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "synced"})
}

func (s *Server) handleRecordReadiness(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	var input struct {
		CanExplain   bool `json:"can_explain"`
		CanDemo      bool `json:"can_demo"`
		CanTradeoffs bool `json:"can_tradeoffs"`
		SelfScore    int  `json:"self_score"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	readinessID, err := s.store.RecordReadiness(id, input.CanExplain, input.CanDemo, input.CanTradeoffs, input.SelfScore)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":         readinessID,
		"self_score": input.SelfScore,
	})
}

func (s *Server) handleGetGuide(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	project, err := s.store.GetProject(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	guidePath := filepath.Join(project.Name, "guide.md")

	// Try reading from disk first.
	if s.contentDir != "" {
		diskPath := filepath.Join(s.contentDir, guidePath)
		f, err := os.Open(diskPath)
		if err == nil {
			defer f.Close()
			data, err := io.ReadAll(io.LimitReader(f, 1<<20)) // 1MB limit
			if err == nil {
				writeJSON(w, http.StatusOK, map[string]string{"content": string(data)})
				return
			}
		}
	}

	// Fall back to embedded content.
	data, err := content.Guides.ReadFile(guidePath)
	if err != nil {
		writeError(w, http.StatusNotFound, "guide not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"content": string(data)})
}

// ToolResponse is the standard response format for tool execution.
type ToolResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func (s *Server) handleToolExecute(w http.ResponseWriter, r *http.Request) {
	toolName := r.PathValue("name")
	if toolName == "" {
		writeError(w, http.StatusBadRequest, "tool name is required")
		return
	}

	var body struct {
		Input map[string]interface{} `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	input := body.Input
	if input == nil {
		input = map[string]interface{}{}
	}

	var result interface{}
	var toolErr error

	switch toolName {
	case "dashboard":
		result, toolErr = s.toolDashboard(input)
	case "project_detail":
		result, toolErr = s.toolProjectDetail(input)
	case "update_progress":
		result, toolErr = s.toolUpdateProgress(input)
	case "record_metric":
		result, toolErr = s.toolRecordMetric(input)
	case "sync_profile":
		result, toolErr = s.toolSyncProfile(input)
	default:
		writeJSON(w, http.StatusBadRequest, ToolResponse{
			Success: false,
			Error:   fmt.Sprintf("unknown tool: %s", toolName),
		})
		return
	}

	if toolErr != nil {
		writeJSON(w, http.StatusOK, ToolResponse{
			Success: false,
			Error:   toolErr.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, ToolResponse{
		Success: true,
		Data:    result,
	})
}

// --- Tool implementations ---

func (s *Server) toolDashboard(input map[string]interface{}) (interface{}, error) {
	view, _ := input["view"].(string)
	if view == "keywords" {
		keywords, err := s.store.ListKeywords()
		if err != nil {
			return nil, err
		}
		if keywords == nil {
			keywords = []store.Keyword{}
		}
		return keywords, nil
	}
	dashboard, err := s.store.GetDashboard()
	if err != nil {
		return nil, err
	}
	if dashboard.Projects == nil {
		dashboard.Projects = []store.ProjectSummary{}
	}
	return dashboard, nil
}

func (s *Server) toolProjectDetail(input map[string]interface{}) (interface{}, error) {
	var project store.Project
	var err error

	if idVal, ok := input["project_id"]; ok {
		id, parseErr := toInt(idVal)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid project_id: %v", parseErr)
		}
		project, err = s.store.GetProject(id)
	} else if nameVal, ok := input["project_name"]; ok {
		name, _ := nameVal.(string)
		if name == "" {
			return nil, fmt.Errorf("project_name must be a non-empty string")
		}
		project, err = s.store.GetProjectByName(name)
	} else {
		return nil, fmt.Errorf("project_id or project_name is required")
	}
	if err != nil {
		return nil, err
	}

	milestones, _ := s.store.ListMilestones(project.ID)
	if milestones == nil {
		milestones = []store.Milestone{}
	}
	metrics, _ := s.store.ListMetrics(project.ID)
	if metrics == nil {
		metrics = []store.Metric{}
	}
	keywords, _ := s.store.ListProjectKeywords(project.ID)
	if keywords == nil {
		keywords = []store.Keyword{}
	}
	syncs, _ := s.store.ListProfileSyncs(project.ID)
	if syncs == nil {
		syncs = []store.ProfileSync{}
	}
	readiness, _ := s.store.GetReadiness(project.ID)

	return store.ProjectDetail{
		Project:    project,
		Milestones: milestones,
		Metrics:    metrics,
		Keywords:   keywords,
		Syncs:      syncs,
		Readiness:  readiness,
	}, nil
}

func (s *Server) toolUpdateProgress(input map[string]interface{}) (interface{}, error) {
	idVal, ok := input["project_id"]
	if !ok {
		return nil, fmt.Errorf("project_id is required")
	}
	id, err := toInt(idVal)
	if err != nil {
		return nil, fmt.Errorf("invalid project_id: %v", err)
	}

	// Update project fields if provided.
	var update store.ProjectUpdate
	if status, ok := input["status"].(string); ok && status != "" {
		update.Status = &status
	}
	if hours, ok := input["hours_actual"]; ok {
		if h, err := toFloat(hours); err == nil {
			update.HoursActual = &h
		}
	}
	if update.Status != nil || update.HoursActual != nil {
		if err := s.store.UpdateProject(id, update); err != nil {
			return nil, fmt.Errorf("update project: %w", err)
		}
	}

	// Update milestone if provided.
	if midVal, ok := input["milestone_id"]; ok {
		mid, err := toInt(midVal)
		if err != nil {
			return nil, fmt.Errorf("invalid milestone_id: %v", err)
		}
		milestoneStatus, _ := input["milestone_status"].(string)
		if milestoneStatus == "" {
			milestoneStatus = "done"
		}
		if err := s.store.UpdateMilestoneStatus(mid, milestoneStatus); err != nil {
			return nil, fmt.Errorf("update milestone: %w", err)
		}
	}

	// Record readiness if provided.
	if _, ok := input["self_score"]; ok {
		selfScore, err := toInt(input["self_score"])
		if err != nil {
			return nil, fmt.Errorf("invalid self_score: %v", err)
		}
		canExplain, _ := input["can_explain"].(bool)
		canDemo, _ := input["can_demo"].(bool)
		canTradeoffs, _ := input["can_tradeoffs"].(bool)
		if _, err := s.store.RecordReadiness(id, canExplain, canDemo, canTradeoffs, selfScore); err != nil {
			return nil, fmt.Errorf("record readiness: %w", err)
		}
	}

	// Return updated project detail.
	return s.toolProjectDetail(map[string]interface{}{"project_id": id})
}

func (s *Server) toolRecordMetric(input map[string]interface{}) (interface{}, error) {
	idVal, ok := input["project_id"]
	if !ok {
		return nil, fmt.Errorf("project_id is required")
	}
	id, err := toInt(idVal)
	if err != nil {
		return nil, fmt.Errorf("invalid project_id: %v", err)
	}

	name, _ := input["name"].(string)
	value, _ := input["value"].(string)
	unit, _ := input["unit"].(string)
	if name == "" || value == "" {
		return nil, fmt.Errorf("name and value are required")
	}

	metricID, err := s.store.RecordMetric(id, name, value, unit)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"id":   metricID,
		"name": name,
	}, nil
}

func (s *Server) toolSyncProfile(input map[string]interface{}) (interface{}, error) {
	idVal, ok := input["project_id"]
	if !ok {
		return nil, fmt.Errorf("project_id is required")
	}
	id, err := toInt(idVal)
	if err != nil {
		return nil, fmt.Errorf("invalid project_id: %v", err)
	}

	platform, _ := input["platform"].(string)
	notes, _ := input["notes"].(string)
	if platform == "" {
		return nil, fmt.Errorf("platform is required")
	}

	if err := s.store.UpdateProfileSync(id, platform, notes); err != nil {
		return nil, err
	}

	return map[string]string{
		"status":   "synced",
		"platform": platform,
	}, nil
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

func parseID(r *http.Request, param string) (int, error) {
	return strconv.Atoi(r.PathValue(param))
}

func toInt(v interface{}) (int, error) {
	switch val := v.(type) {
	case float64:
		return int(val), nil
	case int:
		return val, nil
	case string:
		return strconv.Atoi(val)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", v)
	}
}

func toFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

// --- Middleware ---

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[panic] %v\n%s", err, debug.Stack())
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func bodyLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil && (r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH") {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func cspMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

func requestLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Microsecond))
	})
}

// requestLoggerMiddleware times every HTTP request and logs api.request events.
// Requests exceeding 500ms also produce an api.slow event.
// Health-check requests are passed through without logging.
func requestLoggerMiddleware(logger *metrics.EventLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/health" {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			duration := time.Since(start).Milliseconds()

			data := map[string]interface{}{
				"method":      r.Method,
				"path":        r.URL.Path,
				"status":      sw.status,
				"duration_ms": duration,
			}
			_ = logger.Log(metrics.EventAPIRequest, data)

			if duration > 500 {
				_ = logger.Log(metrics.EventAPISlow, data)
			}
		})
	}
}
