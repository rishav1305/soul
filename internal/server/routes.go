package server

import (
	"encoding/json"
	"log"
	"net/http"
)

// registerRoutes sets up all HTTP routes on the server mux.
func (s *Server) registerRoutes() {
	// Health check endpoint.
	s.mux.HandleFunc("GET /api/health", handleHealth)

	// Catch-all for unknown API routes — returns 404 JSON.
	s.mux.HandleFunc("/api/", handleAPINotFound)

	// WebSocket endpoint for chat streaming.
	s.mux.HandleFunc("GET /ws", s.handleWebSocket)

	// SPA catch-all — serves embedded files, falls back to index.html.
	s.mux.Handle("/", spaHandler())
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: %v", err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func handleAPINotFound(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}
