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

// handleSessionRead handles PATCH /api/sessions/{id}/read — marks a session as read (completed_unread → idle).
func (s *Server) handleSessionRead(w http.ResponseWriter, r *http.Request) {
	if s.planner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "planner not configured"})
		return
	}
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid session id"})
		return
	}
	if err := s.planner.MarkSessionRead(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// Broadcast the status change to all connected clients.
	s.broadcastSessionStatusChanged(id, "idle")
	writeJSON(w, http.StatusOK, map[string]string{"status": "idle"})
}


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
