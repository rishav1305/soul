package server

import (
	"encoding/json"
	"net/http"
)

// registerRoutes sets up all HTTP routes on the given mux.
func registerRoutes(mux *http.ServeMux) {
	// Health check endpoint.
	mux.HandleFunc("GET /api/health", handleHealth)

	// Catch-all for unknown API routes — returns 404 JSON.
	mux.HandleFunc("/api/", handleAPINotFound)

	// SPA catch-all — serves embedded files, falls back to index.html.
	mux.Handle("/", spaHandler())
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleAPINotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
}
