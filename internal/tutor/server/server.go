package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rishav1305/soul-v2/internal/chat/metrics"
	"github.com/rishav1305/soul-v2/internal/tutor/eval"
	"github.com/rishav1305/soul-v2/internal/tutor/modules"
	"github.com/rishav1305/soul-v2/internal/tutor/store"
)

// Server is the tutor HTTP server.
type Server struct {
	store      *store.Store
	modules    *modules.Registry
	metrics    *metrics.EventLogger
	evaluator  *eval.Evaluator
	mux        *http.ServeMux
	httpServer *http.Server
	host       string
	port       int
	startTime  time.Time
	contentDir string
}

// Option configures the Server.
type Option func(*Server)

func WithStore(s *store.Store) Option           { return func(srv *Server) { srv.store = s } }
func WithHost(h string) Option                  { return func(srv *Server) { srv.host = h } }
func WithPort(p int) Option                     { return func(srv *Server) { srv.port = p } }
func WithContentDir(d string) Option            { return func(srv *Server) { srv.contentDir = d } }
func WithMetrics(l *metrics.EventLogger) Option { return func(srv *Server) { srv.metrics = l } }
func WithEvaluator(e *eval.Evaluator) Option    { return func(srv *Server) { srv.evaluator = e } }

// New creates a new tutor Server.
func New(opts ...Option) *Server {
	s := &Server{
		mux:       http.NewServeMux(),
		host:      "127.0.0.1",
		port:      3006,
		startTime: time.Now(),
	}
	for _, opt := range opts {
		opt(s)
	}

	// Create module registry.
	s.modules = modules.NewRegistry(s.store, s.contentDir, s.evaluator)

	// Register routes.
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/tutor/dashboard", s.handleDashboard)
	s.mux.HandleFunc("GET /api/tutor/analytics", s.handleAnalytics)
	s.mux.HandleFunc("GET /api/tutor/topics", s.handleListTopics)
	s.mux.HandleFunc("GET /api/tutor/topics/{id}", s.handleGetTopic)
	s.mux.HandleFunc("POST /api/tutor/drill/start", s.handleDrillStart)
	s.mux.HandleFunc("POST /api/tutor/drill/answer", s.handleDrillAnswer)
	s.mux.HandleFunc("GET /api/tutor/drill/due", s.handleDrillDue)
	s.mux.HandleFunc("GET /api/tutor/mocks", s.handleListMocks)
	s.mux.HandleFunc("POST /api/tutor/mocks", s.handleCreateMock)
	s.mux.HandleFunc("GET /api/tutor/mocks/{id}", s.handleGetMock)
	s.mux.HandleFunc("POST /api/tutor/mocks/{id}/answer", s.handleMockAnswer)
	s.mux.HandleFunc("GET /api/tutor/plan", s.handleGetPlan)
	s.mux.HandleFunc("POST /api/tutor/plan", s.handleCreatePlan)
	s.mux.HandleFunc("PATCH /api/tutor/plan", s.handleUpdatePlan)
	s.mux.HandleFunc("POST /api/tutor/import", s.handleImport)
	s.mux.HandleFunc("POST /api/tools/{name}/execute", s.handleToolExecute)

	// Build middleware chain.
	handler := http.Handler(s.mux)
	if s.metrics != nil {
		handler = requestLoggerMiddleware(s.metrics)(handler)
	}
	handler = bodyLimitMiddleware(64 << 10)(handler)
	handler = cspMiddleware(handler)
	handler = recoveryMiddleware(handler)

	s.httpServer = &http.Server{
		Addr:              net.JoinHostPort(s.host, strconv.Itoa(s.port)),
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
	log.Printf("soul-tutor listening on http://%s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// --- Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	topics, _ := s.store.ListTopics("", "")
	topicCount := len(topics)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "ok",
		"uptime":      time.Since(s.startTime).Round(time.Second).String(),
		"topic_count": topicCount,
	})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	result, err := s.modules.Progress.Progress(map[string]interface{}{"view": "dashboard"})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result.Data)
}

func (s *Server) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	result, err := s.modules.Progress.Progress(map[string]interface{}{"view": "analytics"})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result.Data)
}

func (s *Server) handleListTopics(w http.ResponseWriter, r *http.Request) {
	module := r.URL.Query().Get("module")
	result, err := s.modules.Progress.Progress(map[string]interface{}{
		"view":   "topics",
		"module": module,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result.Data)
}

func (s *Server) handleGetTopic(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid topic id"})
		return
	}
	topic, err := s.store.GetTopic(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "topic not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, topic)
}

func (s *Server) handleDrillStart(w http.ResponseWriter, r *http.Request) {
	var input map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	result, err := s.modules.DSA.Drill(input)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result.Data)
}

func (s *Server) handleDrillAnswer(w http.ResponseWriter, r *http.Request) {
	var input map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	// Must have question_id and answer.
	if _, ok := input["question_id"]; !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "question_id is required"})
		return
	}
	if _, ok := input["answer"]; !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "answer is required"})
		return
	}
	result, err := s.modules.DSA.Drill(input)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result.Data)
}

func (s *Server) handleDrillDue(w http.ResponseWriter, r *http.Request) {
	dueReviews, err := s.store.GetDueReviews(time.Now())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if dueReviews == nil {
		dueReviews = []store.SpacedRepetition{}
	}

	// Enrich with topic info.
	type dueItem struct {
		TopicID    int64  `json:"topicId"`
		TopicName  string `json:"topicName"`
		Module     string `json:"module"`
		NextReview string `json:"nextReview"`
		Interval   int    `json:"intervalDays"`
	}
	var items []dueItem
	for _, sr := range dueReviews {
		topic, err := s.store.GetTopic(sr.TopicID)
		name := fmt.Sprintf("topic-%d", sr.TopicID)
		mod := ""
		if err == nil && topic != nil {
			name = topic.Name
			mod = topic.Module
		}
		items = append(items, dueItem{
			TopicID:    sr.TopicID,
			TopicName:  name,
			Module:     mod,
			NextReview: sr.NextReview.Format("2006-01-02"),
			Interval:   sr.IntervalDays,
		})
	}
	if items == nil {
		items = []dueItem{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"due":   items,
		"count": len(items),
	})
}

func (s *Server) handleListMocks(w http.ResponseWriter, r *http.Request) {
	result, err := s.modules.Progress.Progress(map[string]interface{}{"view": "mocks"})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result.Data)
}

func (s *Server) handleCreateMock(w http.ResponseWriter, r *http.Request) {
	var input map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	result, err := s.modules.Mock.MockInterview(input)
	if err != nil {
		if strings.Contains(err.Error(), "invalid type") {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, result.Data)
}

func (s *Server) handleGetMock(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid mock session id"})
		return
	}
	session, err := s.store.GetMockSession(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "mock session not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Include dimension scores.
	scores, _ := s.store.GetMockScores(id)
	if scores == nil {
		scores = []store.MockSessionScore{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"session": session,
		"scores":  scores,
	})
}

func (s *Server) handleMockAnswer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid mock session id"})
		return
	}

	var input struct {
		OverallScore float64            `json:"overall_score"`
		FeedbackJSON string             `json:"feedback_json"`
		Scores       []dimensionScoreIn `json:"scores"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if err := s.store.CompleteMockSession(id, input.OverallScore, input.FeedbackJSON); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "mock session not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Add dimension scores.
	for _, ds := range input.Scores {
		s.store.AddMockScore(id, ds.Dimension, ds.Score)
	}

	session, _ := s.store.GetMockSession(id)
	writeJSON(w, http.StatusOK, session)
}

type dimensionScoreIn struct {
	Dimension string  `json:"dimension"`
	Score     float64 `json:"score"`
}

func (s *Server) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	result, err := s.modules.Planner.Plan(map[string]interface{}{"action": "get"})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result.Data)
}

func (s *Server) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	var input map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	input["action"] = "create"
	result, err := s.modules.Planner.Plan(input)
	if err != nil {
		if strings.Contains(err.Error(), "requires") || strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "past") {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, result.Data)
}

func (s *Server) handleUpdatePlan(w http.ResponseWriter, r *http.Request) {
	result, err := s.modules.Planner.Plan(map[string]interface{}{"action": "update"})
	if err != nil {
		if strings.Contains(err.Error(), "no active plan") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result.Data)
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	importer := modules.NewImporter(s.store, s.contentDir)
	stats, err := importer.ImportStdlib()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleToolExecute(w http.ResponseWriter, r *http.Request) {
	toolName := r.PathValue("name")
	if toolName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tool name is required"})
		return
	}

	var input map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	var result *modules.ToolResult
	var err error

	switch toolName {
	// DSA tools
	case "dsa_learn":
		result, err = s.modules.DSA.Learn(input)
	case "dsa_build":
		result, err = s.modules.DSA.Build(input)
	case "dsa_drill":
		result, err = s.modules.DSA.Drill(input)
	case "dsa_solve":
		result, err = s.modules.DSA.Solve(input)
	case "dsa_generate":
		result, err = s.modules.DSA.GenerateContent(input)
	// AI tools
	case "ai_learn":
		result, err = s.modules.AI.LearnTheory(input)
	case "ai_drill":
		result, err = s.modules.AI.DrillTheory(input)
	case "ai_generate":
		result, err = s.modules.AI.GenerateAIContent(input)
	// Behavioral tools
	case "behavioral_narrative":
		result, err = s.modules.Behavioral.BuildNarrative(input)
	case "behavioral_star":
		result, err = s.modules.Behavioral.BuildStar(input)
	case "behavioral_hr":
		result, err = s.modules.Behavioral.DrillHR(input)
	// Mock tools
	case "mock_interview":
		result, err = s.modules.Mock.MockInterview(input)
	case "mock_analyze_jd":
		result, err = s.modules.Mock.AnalyzeJD(input)
	// Planner tools
	case "planner":
		result, err = s.modules.Planner.Plan(input)
	// Progress tools
	case "progress":
		result, err = s.modules.Progress.Progress(input)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("unknown tool: %s", toolName)})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tool":    toolName,
		"summary": result.Summary,
		"data":    result.Data,
	})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// --- Request metrics ---

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
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
			sr := &statusRecorder{ResponseWriter: w, status: 200}
			next.ServeHTTP(sr, r)
			duration := time.Since(start).Milliseconds()

			data := map[string]interface{}{
				"method":      r.Method,
				"path":        r.URL.Path,
				"status":      sr.status,
				"duration_ms": duration,
			}
			_ = logger.Log(metrics.EventAPIRequest, data)

			if duration > 500 {
				_ = logger.Log(metrics.EventAPISlow, data)
			}
		})
	}
}

// --- Middleware ---

var requestCounter atomic.Uint64

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
