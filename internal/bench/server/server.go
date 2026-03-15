package server

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/rishav1305/soul-v2/internal/bench/harness"
	"github.com/rishav1305/soul-v2/internal/bench/prompts"
	"github.com/rishav1305/soul-v2/internal/bench/results"
)

// Server is the bench HTTP server.
type Server struct {
	mux        *http.ServeMux
	httpServer *http.Server
	host       string
	port       int
	dataDir    string
	startTime  time.Time
}

// Option configures the Server.
type Option func(*Server)

// WithHost sets the bind address.
func WithHost(h string) Option { return func(s *Server) { s.host = h } }

// WithPort sets the listen port.
func WithPort(p int) Option { return func(s *Server) { s.port = p } }

// WithDataDir sets the data storage directory.
func WithDataDir(d string) Option { return func(s *Server) { s.dataDir = d } }

// New creates a new bench Server.
func New(opts ...Option) *Server {
	s := &Server{
		mux:       http.NewServeMux(),
		host:      "127.0.0.1",
		port:      3026,
		startTime: time.Now(),
	}
	for _, opt := range opts {
		opt(s)
	}

	// Register routes.
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/bench/prompts", s.handleListCategories)
	s.mux.HandleFunc("GET /api/bench/prompts/{category}", s.handleGetPrompts)
	s.mux.HandleFunc("POST /api/bench/run", s.handleRunBenchmark)
	s.mux.HandleFunc("POST /api/bench/smoke", s.handleSmoke)
	s.mux.HandleFunc("GET /api/bench/results", s.handleListResults)
	s.mux.HandleFunc("GET /api/bench/results/{id}", s.handleGetResult)
	s.mux.HandleFunc("GET /api/bench/compare", s.handleCompare)
	s.mux.HandleFunc("POST /api/tools/{name}/execute", s.handleToolExecute)

	// Build middleware chain.
	handler := http.Handler(s.mux)
	handler = corsMiddleware(handler)
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
	log.Printf("soul-bench listening on http://%s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// --- Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"uptime": time.Since(s.startTime).String(),
	})
}

func (s *Server) handleListCategories(w http.ResponseWriter, _ *http.Request) {
	cats := prompts.Categories()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"categories": cats,
		"count":      len(cats),
	})
}

func (s *Server) handleGetPrompts(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	loaded, err := prompts.LoadCategory(category)
	if err != nil {
		writeError(w, http.StatusNotFound, "category not found: "+category)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"category": category,
		"prompts":  loaded,
		"count":    len(loaded),
	})
}

type runRequest struct {
	ModelEndpoint string   `json:"model_endpoint"`
	Categories    []string `json:"categories"`
	MaxTokens     int      `json:"max_tokens"`
	GPU           bool     `json:"gpu"`
}

func (s *Server) handleRunBenchmark(w http.ResponseWriter, r *http.Request) {
	var req runRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ModelEndpoint == "" {
		writeError(w, http.StatusBadRequest, "model_endpoint is required")
		return
	}

	config := harness.BenchConfig{
		ModelEndpoint: req.ModelEndpoint,
		Categories:    req.Categories,
		MaxTokens:     req.MaxTokens,
		GPU:           req.GPU,
	}

	result, err := harness.RunBenchmark(config)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "benchmark failed: "+err.Error())
		return
	}

	// Save result.
	if s.dataDir != "" {
		if err := results.SaveResult(s.dataDir, result); err != nil {
			log.Printf("failed to save result: %v", err)
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleSmoke(w http.ResponseWriter, r *http.Request) {
	var req runRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ModelEndpoint == "" {
		writeError(w, http.StatusBadRequest, "model_endpoint is required")
		return
	}

	config := harness.BenchConfig{
		ModelEndpoint: req.ModelEndpoint,
		MaxTokens:     req.MaxTokens,
		GPU:           req.GPU,
	}

	result, err := harness.RunSmoke(config)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "smoke test failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleListResults(w http.ResponseWriter, _ *http.Request) {
	if s.dataDir == "" {
		writeError(w, http.StatusInternalServerError, "data directory not configured")
		return
	}
	list, err := results.ListResults(s.dataDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list results: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"results": list,
		"count":   len(list),
	})
}

func (s *Server) handleGetResult(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.dataDir == "" {
		writeError(w, http.StatusInternalServerError, "data directory not configured")
		return
	}
	result, err := results.GetResult(s.dataDir, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "result not found: "+id)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	id1 := r.URL.Query().Get("id1")
	id2 := r.URL.Query().Get("id2")
	if id1 == "" || id2 == "" {
		writeError(w, http.StatusBadRequest, "id1 and id2 query parameters required")
		return
	}
	if s.dataDir == "" {
		writeError(w, http.StatusInternalServerError, "data directory not configured")
		return
	}
	comp, err := results.CompareResults(s.dataDir, id1, id2)
	if err != nil {
		writeError(w, http.StatusNotFound, "compare failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, comp)
}

// toolRequest is the payload for tool dispatch.
type toolRequest struct {
	Input map[string]interface{} `json:"input"`
}

func (s *Server) handleToolExecute(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	var req toolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	switch name {
	case "bench_list_categories":
		cats := prompts.Categories()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"categories": cats,
		})
	case "bench_run_smoke":
		endpoint, _ := req.Input["model_endpoint"].(string)
		if endpoint == "" {
			writeError(w, http.StatusBadRequest, "model_endpoint required")
			return
		}
		result, err := harness.RunSmoke(harness.BenchConfig{ModelEndpoint: endpoint})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	case "bench_list_results":
		list, err := results.ListResults(s.dataDir)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"results": list})
	case "bench_compare":
		id1, _ := req.Input["id1"].(string)
		id2, _ := req.Input["id2"].(string)
		comp, err := results.CompareResults(s.dataDir, id1, id2)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, comp)
	default:
		writeError(w, http.StatusNotFound, "unknown tool: "+name)
	}
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
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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
