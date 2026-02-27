package server

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	soulv1 "github.com/rishav1305/soul/proto/soul/v1"
)

// registerRoutes sets up all HTTP routes on the server mux.
func (s *Server) registerRoutes() {
	// Health check endpoint.
	s.mux.HandleFunc("GET /api/health", handleHealth)

	// Tools list endpoint — returns all registered tools.
	s.mux.HandleFunc("GET /api/tools", s.handleToolsList)

	// Direct tool execution endpoint (bypasses AI).
	s.mux.HandleFunc("POST /api/tools/{tool}/execute", s.handleToolExecute)

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

// toolInfo is the JSON-serializable tool metadata returned by /api/tools.
type toolInfo struct {
	Name             string `json:"name"`
	QualifiedName    string `json:"qualified_name"`
	Product          string `json:"product"`
	Description      string `json:"description"`
	RequiresApproval bool   `json:"requires_approval"`
	Tier             string `json:"tier,omitempty"`
}

// handleToolsList returns all registered tools as JSON.
func (s *Server) handleToolsList(w http.ResponseWriter, r *http.Request) {
	if s.products == nil {
		writeJSON(w, http.StatusOK, []toolInfo{})
		return
	}

	entries := s.products.Registry().AllTools()
	tools := make([]toolInfo, len(entries))
	for i, entry := range entries {
		tools[i] = toolInfo{
			Name:             entry.Tool.GetName(),
			QualifiedName:    entry.ProductName + "__" + entry.Tool.GetName(),
			Product:          entry.ProductName,
			Description:      entry.Tool.GetDescription(),
			RequiresApproval: entry.Tool.GetRequiresApproval(),
			Tier:             entry.Tool.GetTier(),
		}
	}

	writeJSON(w, http.StatusOK, tools)
}

// handleToolExecute directly executes a tool, bypassing the AI agent.
// This is used for UI actions like "Fix All" buttons.
func (s *Server) handleToolExecute(w http.ResponseWriter, r *http.Request) {
	qualifiedName := r.PathValue("tool")
	if qualifiedName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tool name required"})
		return
	}

	if s.products == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no products configured"})
		return
	}

	// Parse request body.
	var reqBody struct {
		Input json.RawMessage `json:"input"`
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &reqBody); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
	}

	// Look up the tool.
	registry := s.products.Registry()
	entry, found := registry.FindTool(qualifiedName)
	if !found {
		// Also try with the tool name as-is (without product prefix).
		// Search through all tools to find a match.
		found = false
		for _, e := range registry.AllTools() {
			if e.Tool.GetName() == qualifiedName || (e.ProductName+"__"+e.Tool.GetName()) == qualifiedName {
				entry = e
				found = true
				break
			}
		}
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "tool not found: " + qualifiedName})
			return
		}
	}

	// Get the gRPC client.
	client, ok := s.products.GetClient(entry.ProductName)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "product not available: " + entry.ProductName})
		return
	}

	// Build the input JSON string.
	inputJSON := "{}"
	if len(reqBody.Input) > 0 {
		inputJSON = string(reqBody.Input)
	}

	// Execute the tool via unary gRPC call.
	toolReq := &soulv1.ToolRequest{
		Tool:      entry.Tool.GetName(),
		InputJson: inputJSON,
	}

	resp, err := client.ExecuteTool(r.Context(), toolReq)
	if err != nil {
		// Check if it's a gRPC error with a message.
		errMsg := err.Error()
		if strings.Contains(errMsg, "desc = ") {
			parts := strings.SplitN(errMsg, "desc = ", 2)
			if len(parts) == 2 {
				errMsg = parts[1]
			}
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": errMsg})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":         resp.GetSuccess(),
		"output":          resp.GetOutput(),
		"structured_json": resp.GetStructuredJson(),
	})
}
