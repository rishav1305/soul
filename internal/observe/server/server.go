package server

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/rishav1305/soul-v2/internal/chat/metrics"
)

// Server is the observe HTTP server.
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

func WithHost(h string) Option    { return func(s *Server) { s.host = h } }
func WithPort(p int) Option      { return func(s *Server) { s.port = p } }
func WithDataDir(d string) Option { return func(s *Server) { s.dataDir = d } }

// New creates a new observe Server.
func New(opts ...Option) *Server {
	s := &Server{
		mux:       http.NewServeMux(),
		host:      "127.0.0.1",
		port:      3010,
		startTime: time.Now(),
	}
	for _, opt := range opts {
		opt(s)
	}

	// Register routes.
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/overview", s.handleOverview)
	s.mux.HandleFunc("GET /api/latency", s.handleLatency)
	s.mux.HandleFunc("GET /api/alerts", s.handleAlerts)
	s.mux.HandleFunc("GET /api/db", s.handleDB)
	s.mux.HandleFunc("GET /api/requests", s.handleRequests)
	s.mux.HandleFunc("GET /api/frontend", s.handleFrontend)
	s.mux.HandleFunc("GET /api/usage", s.handleUsage)
	s.mux.HandleFunc("GET /api/quality", s.handleQuality)
	s.mux.HandleFunc("GET /api/layers", s.handleLayers)
	s.mux.HandleFunc("GET /api/system", s.handleSystem)
	s.mux.HandleFunc("GET /api/tail", s.handleTail)
	s.mux.HandleFunc("GET /api/pillars", s.handlePillars)

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
	log.Printf("soul-observe listening on http://%s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// aggregator creates a product-filtered aggregator from query params.
func (s *Server) aggregator(r *http.Request) *metrics.Aggregator {
	product := r.URL.Query().Get("product")
	return metrics.NewAggregatorForProduct(s.dataDir, product)
}

// --- Middleware ---

// recoveryMiddleware catches panics and returns a 500 JSON error.
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

// corsMiddleware sets CORS headers for proxied access.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://127.0.0.1:3002")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
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
