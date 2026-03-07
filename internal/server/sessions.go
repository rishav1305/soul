package server

import (
	"fmt"
	"net/http"
	"strconv"
)

// handleSessionCreate handles POST /api/sessions — creates a new chat session.
func (s *Server) handleSessionCreate(w http.ResponseWriter, r *http.Request) {
	if s.planner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "planner not configured"})
		return
	}

	var body struct {
		Title string `json:"title"`
	}
	if err := readJSON(r, &body); err != nil {
		// Allow empty body — title defaults to "".
		body.Title = ""
	}

	session, err := s.planner.CreateSession(body.Title)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to create session: %v", err)})
		return
	}

	writeJSON(w, http.StatusCreated, session)
}

// handleSessionList handles GET /api/sessions — lists recent chat sessions.
func (s *Server) handleSessionList(w http.ResponseWriter, r *http.Request) {
	if s.planner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "planner not configured"})
		return
	}

	sessions, err := s.planner.ListSessions(30)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to list sessions: %v", err)})
		return
	}

	// Return empty array instead of null.
	if sessions == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

// handleSessionMessages handles GET /api/sessions/{id}/messages — returns messages for a session.
func (s *Server) handleSessionMessages(w http.ResponseWriter, r *http.Request) {
	if s.planner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "planner not configured"})
		return
	}

	idStr := r.PathValue("id")
	if idStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing session id"})
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid session id: %q", idStr)})
		return
	}

	msgs, err := s.planner.GetSessionMessages(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to get messages: %v", err)})
		return
	}

	// Return empty array instead of null.
	if msgs == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, msgs)
}
