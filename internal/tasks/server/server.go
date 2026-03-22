package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rishav1305/soul-v2/internal/chat/metrics"
	"github.com/rishav1305/soul-v2/internal/tasks/executor"
	"github.com/rishav1305/soul-v2/internal/tasks/store"
	"github.com/rishav1305/soul-v2/pkg/events"
)

// Server is the tasks HTTP server.
type Server struct {
	mux         *http.ServeMux
	httpServer  *http.Server
	store       *store.Store
	executor    *executor.Executor
	broadcaster *Broadcaster
	logger      events.Logger
	metrics     *metrics.EventLogger
	host        string
	port        int
	startTime   time.Time
	pruneStop   chan struct{}
	pruneOnce   sync.Once
}

// Option configures the Server.
type Option func(*Server)

func WithStore(s *store.Store) Option      { return func(srv *Server) { srv.store = s } }
func WithLogger(l events.Logger) Option    { return func(srv *Server) { srv.logger = l } }
func WithHost(h string) Option             { return func(srv *Server) { srv.host = h } }
func WithPort(p int) Option                { return func(srv *Server) { srv.port = p } }
func WithExecutor(e *executor.Executor) Option { return func(srv *Server) { srv.executor = e } }
func WithMetrics(l *metrics.EventLogger) Option { return func(srv *Server) { srv.metrics = l } }

// New creates a new tasks Server.
func New(opts ...Option) *Server {
	s := &Server{
		mux:         http.NewServeMux(),
		broadcaster: NewBroadcaster(),
		logger:      events.NopLogger{},
		host:        "127.0.0.1",
		port:        3004,
		startTime:   time.Now(),
		pruneStop:   make(chan struct{}),
	}
	for _, opt := range opts {
		opt(s)
	}

	// Wire OnChange → Broadcast.
	if s.store != nil {
		s.store.OnChange = func(event string, payload any) {
			data, _ := json.Marshal(payload)
			s.broadcaster.Broadcast(Event{Type: event, Data: string(data)})
		}
	}

	// Register routes.
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/tasks", s.handleListTasks)
	s.mux.HandleFunc("POST /api/tasks", s.handleCreateTask)
	s.mux.HandleFunc("GET /api/tasks/sync", s.handleSync)
	s.mux.HandleFunc("GET /api/tasks/{id}", s.handleGetTask)
	s.mux.HandleFunc("PATCH /api/tasks/{id}", s.handleUpdateTask)
	s.mux.HandleFunc("DELETE /api/tasks/{id}", s.handleDeleteTask)
	s.mux.HandleFunc("POST /api/tasks/{id}/start", s.handleStartTask)
	s.mux.HandleFunc("POST /api/tasks/{id}/stop", s.handleStopTask)
	s.mux.HandleFunc("GET /api/tasks/{id}/activity", s.handleTaskActivity)
	s.mux.HandleFunc("POST /api/tasks/{id}/comments", s.handleCreateComment)
	s.mux.HandleFunc("GET /api/tasks/{id}/comments", s.handleListComments)
	s.mux.HandleFunc("POST /api/tasks/{id}/dependencies", s.handleAddDependency)
	s.mux.HandleFunc("DELETE /api/tasks/{id}/dependencies/{depId}", s.handleRemoveDependency)
	s.mux.HandleFunc("GET /api/stream", s.handleStream)

	// Build middleware chain.
	handler := http.Handler(s.mux)
	if s.metrics != nil {
		handler = requestLoggerMiddleware(s.metrics)(handler)
	}
	handler = bodyLimitMiddleware(64 << 10)(handler)
	handler = cspMiddleware(handler)
	handler = requestIDMiddleware(handler)
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

// Start begins listening. It also starts the tombstone pruning goroutine.
func (s *Server) Start() error {
	if s.store != nil {
		s.store.PruneTombstones()
		go func() {
			ticker := time.NewTicker(1 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					s.store.PruneTombstones()
				case <-s.pruneStop:
					return
				}
			}
		}()
	}
	log.Printf("soul-tasks listening on http://%s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server and the pruning goroutine.
func (s *Server) Shutdown(ctx context.Context) error {
	s.pruneOnce.Do(func() { close(s.pruneStop) })
	return s.httpServer.Shutdown(ctx)
}

// --- Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	counts, _ := s.store.CountByStage()
	active := counts["active"]

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "ok",
		"uptime":       time.Since(s.startTime).Round(time.Second).String(),
		"active_tasks": active,
	})
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	stage := r.URL.Query().Get("stage")
	product := r.URL.Query().Get("product")

	tasks, err := s.store.List(stage, product)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if tasks == nil {
		tasks = []store.Task{}
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	cursorParam := r.URL.Query().Get("cursor")
	seq, ts, err := store.DecodeCursor(cursorParam)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cursor"})
		return
	}

	now := time.Now().Unix()

	// Full sync if no cursor or stale (>24h).
	if cursorParam == "" || (now-ts > 24*3600) {
		tasks, err := s.store.List("", "")
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if tasks == nil {
			tasks = []store.Task{}
		}
		maxSeq, err := s.store.MaxSeq()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read sequence"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"tasks":    tasks,
			"deleted":  []int64{},
			"cursor":   store.EncodeCursor(maxSeq, now),
			"fullSync": true,
		})
		return
	}

	currentSeq, err := s.store.MaxSeq()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read sequence"})
		return
	}

	tasks, err := s.store.ListModifiedSince(seq)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if tasks == nil {
		tasks = []store.Task{}
	}

	deleted, err := s.store.ListDeletedSince(seq)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if deleted == nil {
		deleted = []int64{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tasks":    tasks,
		"deleted":  deleted,
		"cursor":   store.EncodeCursor(currentSeq, now),
		"fullSync": false,
	})
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Product     string `json:"product"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if body.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}

	task, err := s.store.Create(body.Title, body.Description, body.Product)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Log activity.
	_, _ = s.store.AddActivity(task.ID, "task.created", map[string]interface{}{
		"title": task.Title,
	})

	_ = s.logger.Log("task.created", map[string]interface{}{
		"task_id": task.ID,
		"title":   task.Title,
	})

	writeJSON(w, http.StatusCreated, task)
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	task, err := s.store.Get(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var fields map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	task, err := s.store.Update(id, fields)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
			return
		}
		if strings.Contains(err.Error(), "invalid stage") {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, task)
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if err := s.store.Delete(id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStartTask(w http.ResponseWriter, r *http.Request) {
	if s.executor == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "executor not configured"})
		return
	}
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if err := s.executor.Start(r.Context(), id); err != nil {
		status := http.StatusInternalServerError
		errMsg := err.Error()
		if strings.Contains(errMsg, "already running") || strings.Contains(errMsg, "max parallel") {
			status = http.StatusConflict
		}
		if strings.Contains(errMsg, "not found") {
			status = http.StatusNotFound
		}
		if strings.Contains(errMsg, "must be backlog or blocked") {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]string{"error": errMsg})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (s *Server) handleStopTask(w http.ResponseWriter, r *http.Request) {
	if s.executor == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "executor not configured"})
		return
	}
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if err := s.executor.Stop(id); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not running") {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (s *Server) handleTaskActivity(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	afterStr := r.URL.Query().Get("after")
	if afterStr != "" {
		afterID, err := strconv.ParseInt(afterStr, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid after param"})
			return
		}
		activities, err := s.store.ActivityAfterID(id, afterID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if activities == nil {
			activities = []store.Activity{}
		}
		writeJSON(w, http.StatusOK, activities)
		return
	}

	activities, err := s.store.ListActivity(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if activities == nil {
		activities = []store.Activity{}
	}
	writeJSON(w, http.StatusOK, activities)
}

func (s *Server) handleAddDependency(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	var body struct {
		DependsOn int64 `json:"depends_on"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if body.DependsOn == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "depends_on is required"})
		return
	}
	if err := s.store.AddDependency(id, body.DependsOn); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (s *Server) handleRemoveDependency(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	depID, err := strconv.ParseInt(r.PathValue("depId"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid dependency id"})
		return
	}
	if err := s.store.RemoveDependency(id, depID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateComment(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	var body struct {
		Author string `json:"author"`
		Type   string `json:"type"`
		Body   string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if body.Author == "" || body.Body == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "author and body are required"})
		return
	}
	if body.Type == "" {
		body.Type = "feedback"
	}

	cmt, err := s.store.InsertComment(id, body.Author, body.Type, body.Body)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"id": cmt.ID})
}

func (s *Server) handleListComments(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	afterStr := r.URL.Query().Get("after")
	if afterStr != "" {
		afterID, err := strconv.ParseInt(afterStr, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid after param"})
			return
		}
		comments, err := s.store.AllCommentsAfterID(id, afterID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if comments == nil {
			comments = []store.Comment{}
		}
		writeJSON(w, http.StatusOK, comments)
		return
	}

	comments, err := s.store.GetComments(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if comments == nil {
		comments = []store.Comment{}
	}
	writeJSON(w, http.StatusOK, comments)
}

// handleStream handles the GET /api/stream SSE endpoint.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch, cancel := s.broadcaster.Subscribe()
	defer cancel()

	// Send initial connected event.
	fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
	flusher.Flush()

	for {
		select {
		case ev := <-ch:
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, ev.Data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
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
// Health-check and SSE stream requests are passed through without logging.
func requestLoggerMiddleware(logger *metrics.EventLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/health" || r.URL.Path == "/api/stream" {
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

// --- Helpers ---

func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
