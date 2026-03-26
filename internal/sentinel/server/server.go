package server

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/rishav1305/soul/internal/sentinel/engine"
	"github.com/rishav1305/soul/internal/sentinel/store"
)

// Server is the sentinel HTTP server.
type Server struct {
	mux        *http.ServeMux
	httpServer *http.Server
	host       string
	port       int
	engine     *engine.Engine
	store      *store.Store
	startTime  time.Time
}

// Option configures the Server.
type Option func(*Server)

// WithHost sets the bind address.
func WithHost(h string) Option { return func(s *Server) { s.host = h } }

// WithPort sets the listen port.
func WithPort(p int) Option { return func(s *Server) { s.port = p } }

// WithEngine sets the CTF engine.
func WithEngine(e *engine.Engine) Option { return func(s *Server) { s.engine = e } }

// WithStore sets the sentinel store.
func WithStore(st *store.Store) Option { return func(s *Server) { s.store = st } }

// New creates a new sentinel Server.
func New(opts ...Option) *Server {
	s := &Server{
		mux:       http.NewServeMux(),
		host:      "127.0.0.1",
		port:      3022,
		startTime: time.Now(),
	}
	for _, opt := range opts {
		opt(s)
	}

	// Register routes.
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/challenges", s.handleListChallenges)
	s.mux.HandleFunc("POST /api/challenges/start", s.handleStartChallenge)
	s.mux.HandleFunc("POST /api/challenges/submit", s.handleSubmitFlag)
	s.mux.HandleFunc("POST /api/attack", s.handleAttack)
	s.mux.HandleFunc("POST /api/sandbox/config", s.handleSaveSandboxConfig)
	s.mux.HandleFunc("POST /api/defend", s.handleSaveGuardrail)
	s.mux.HandleFunc("POST /api/scan", s.handleScan)
	s.mux.HandleFunc("GET /api/progress", s.handleProgress)
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
	log.Printf("soul-sentinel listening on http://%s", s.httpServer.Addr)
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

func (s *Server) handleListChallenges(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	difficulty := r.URL.Query().Get("difficulty")

	challenges, err := s.store.ListChallenges(category, difficulty, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Strip flags and system prompts from the response.
	type safeChallenge struct {
		ID          string   `json:"id"`
		Category    string   `json:"category"`
		Difficulty  string   `json:"difficulty"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Objective   string   `json:"objective"`
		Points      int      `json:"points"`
		MaxTurns    int      `json:"max_turns"`
		Phase       int      `json:"phase"`
		HintCount   int      `json:"hint_count"`
		LearnMore   string   `json:"learn_more"`
		Tools       []string `json:"tools"`
	}

	safe := make([]safeChallenge, len(challenges))
	for i, c := range challenges {
		safe[i] = safeChallenge{
			ID:          c.ID,
			Category:    c.Category,
			Difficulty:  c.Difficulty,
			Title:       c.Title,
			Description: c.Description,
			Objective:   c.Objective,
			Points:      c.Points,
			MaxTurns:    c.MaxTurns,
			Phase:       c.Phase,
			HintCount:   len(c.Hints),
			LearnMore:   c.LearnMore,
			Tools:       c.Tools,
		}
	}

	writeJSON(w, http.StatusOK, safe)
}

func (s *Server) handleStartChallenge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChallengeID string `json:"challengeId"`
		Reset       bool   `json:"reset"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ChallengeID == "" {
		writeError(w, http.StatusBadRequest, "challengeId is required")
		return
	}

	challenge, err := s.engine.StartSession(req.ChallengeID, req.Reset)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"challengeId": challenge.ID,
		"title":       challenge.Title,
		"description": challenge.Description,
		"objective":   challenge.Objective,
		"maxTurns":    challenge.MaxTurns,
		"points":      challenge.Points,
	})
}

func (s *Server) handleSubmitFlag(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChallengeID string `json:"challengeId"`
		Flag        string `json:"flag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ChallengeID == "" || req.Flag == "" {
		writeError(w, http.StatusBadRequest, "challengeId and flag are required")
		return
	}

	points, correct, err := s.engine.SubmitFlag(req.ChallengeID, req.Flag)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"correct": correct,
		"points":  points,
	})
}

func (s *Server) handleAttack(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChallengeID string `json:"challengeId"`
		Payload     string `json:"payload"`
		Sandbox     bool   `json:"sandbox"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Payload == "" {
		writeError(w, http.StatusBadRequest, "payload is required")
		return
	}

	if req.Sandbox {
		resp, err := s.engine.AttackSandbox(req.Payload)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"response": resp,
			"sandbox":  true,
		})
		return
	}

	if req.ChallengeID == "" {
		writeError(w, http.StatusBadRequest, "challengeId is required for non-sandbox attacks")
		return
	}

	resp, turns, err := s.engine.AttackChallenge(req.ChallengeID, req.Payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"response": resp,
		"turns":    turns,
	})
}

func (s *Server) handleSaveSandboxConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name          string   `json:"name"`
		SystemPrompt  string   `json:"systemPrompt"`
		Guardrails    []string `json:"guardrails"`
		WeaknessLevel string   `json:"weaknessLevel"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	guardrailsJSON, _ := json.Marshal(req.Guardrails)
	if req.WeaknessLevel == "" {
		req.WeaknessLevel = "none"
	}

	id, err := s.store.SaveSandboxConfig(req.Name, req.SystemPrompt, string(guardrailsJSON), req.WeaknessLevel)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":   id,
		"name": req.Name,
	})
}

func (s *Server) handleSaveGuardrail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string          `json:"name"`
		Rule json.RawMessage `json:"rule"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	id, err := s.store.SaveGuardrail(req.Name, string(req.Rule))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":   id,
		"name": req.Name,
	})
}

func (s *Server) handleScan(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "not_implemented",
		"message": "Security scanning is not yet implemented.",
	})
}

func (s *Server) handleProgress(w http.ResponseWriter, _ *http.Request) {
	progress, err := s.store.GetProgress()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	totalPoints := 0
	for _, p := range progress {
		totalPoints += p
	}

	// Build per-category breakdown.
	challenges, _ := s.store.ListChallenges("", "", 0)
	categories := make(map[string]struct{ total, completed int })
	for _, c := range challenges {
		cat := categories[c.Category]
		cat.total++
		if _, ok := progress[c.ID]; ok {
			cat.completed++
		}
		categories[c.Category] = cat
	}

	type categoryProgress struct {
		Total     int `json:"total"`
		Completed int `json:"completed"`
	}
	catMap := make(map[string]categoryProgress)
	for name, cat := range categories {
		catMap[name] = categoryProgress{Total: cat.total, Completed: cat.completed}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"totalPoints":         totalPoints,
		"completedChallenges": len(progress),
		"totalChallenges":     len(challenges),
		"categories":          catMap,
	})
}

func (s *Server) handleToolExecute(w http.ResponseWriter, r *http.Request) {
	toolName := r.PathValue("name")
	if toolName == "" {
		writeError(w, http.StatusBadRequest, "tool name is required")
		return
	}

	var input json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Dispatch based on tool name.
	switch toolName {
	case "list_challenges":
		s.handleListChallenges(w, r)
	case "start_challenge":
		s.toolStartChallenge(w, input)
	case "attack":
		s.toolAttack(w, input)
	case "submit_flag":
		s.toolSubmitFlag(w, input)
	default:
		writeError(w, http.StatusNotFound, "unknown tool: "+toolName)
	}
}

func (s *Server) toolStartChallenge(w http.ResponseWriter, input json.RawMessage) {
	var req struct {
		ChallengeID string `json:"challenge_id"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid input")
		return
	}
	challenge, err := s.engine.StartSession(req.ChallengeID, false)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"challengeId": challenge.ID,
		"title":       challenge.Title,
		"objective":   challenge.Objective,
		"maxTurns":    challenge.MaxTurns,
	})
}

func (s *Server) toolAttack(w http.ResponseWriter, input json.RawMessage) {
	var req struct {
		ChallengeID string `json:"challenge_id"`
		Payload     string `json:"payload"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid input")
		return
	}
	resp, turns, err := s.engine.AttackChallenge(req.ChallengeID, req.Payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"response": resp,
		"turns":    turns,
	})
}

func (s *Server) toolSubmitFlag(w http.ResponseWriter, input json.RawMessage) {
	var req struct {
		ChallengeID string `json:"challenge_id"`
		Flag        string `json:"flag"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid input")
		return
	}
	points, correct, err := s.engine.SubmitFlag(req.ChallengeID, req.Flag)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"correct": correct,
		"points":  points,
	})
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
