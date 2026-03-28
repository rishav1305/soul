package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rishav1305/soul/pkg/auth"
)

// modelsBetaHeader includes all beta features needed to probe current-gen models.
// Opus 4.6 and Sonnet 4.6 require interleaved-thinking; all OAuth calls need the OAuth beta.
const modelsBetaHeader = "prompt-caching-2024-07-31,interleaved-thinking-2025-05-14," + auth.OAuthBetaHeader

type ModelInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	MaxTokens int    `json:"max_tokens"`
}

type modelsResponse struct {
	Models        []ModelInfo `json:"models"`
	ThinkingTypes []string    `json:"thinking_types"`
}

type modelCache struct {
	mu        sync.RWMutex
	models    []ModelInfo
	fetchedAt time.Time
	ttl       time.Duration
}

var knownMaxTokens = map[string]int{
	"claude-opus-4":   64000,
	"claude-sonnet-4": 64000,
	"claude-haiku-4":  64000,
}

// Launch config: only Haiku 4.5 is confirmed working with OAuth tokens.
// Opus/Sonnet removed — CEO confirmed broken (Mar 28). Re-enable when fixed.
var currentGenPrefixes = []string{"claude-haiku-4"}

var defaultThinkingTypes = []string{"disabled", "adaptive", "enabled"}

// fallbackModels is returned when the Claude API is unreachable.
// Haiku 4.5 only — the sole model reliably accessible via Claude Code OAuth tokens.
var fallbackModels = []ModelInfo{
	{ID: "claude-haiku-4-5-20251001", Name: "Claude Haiku 4.5", MaxTokens: 64000},
}

func maxTokensForModel(modelID string) int {
	for prefix, tokens := range knownMaxTokens {
		if strings.HasPrefix(modelID, prefix) {
			return tokens
		}
	}
	return 16384
}

func isCurrentGen(modelID string) bool {
	for _, prefix := range currentGenPrefixes {
		if strings.HasPrefix(modelID, prefix) {
			return true
		}
	}
	return false
}

type claudeModelsResponse struct {
	Data []struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
		CreatedAt   string `json:"created_at"`
		Type        string `json:"type"`
	} `json:"data"`
	HasMore bool   `json:"has_more"`
	LastID  string `json:"last_id"`
}

func (s *Server) fetchModels() ([]ModelInfo, error) {
	if s.auth == nil {
		return nil, fmt.Errorf("authentication not configured")
	}
	token, err := s.auth.Token()
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	req, err := http.NewRequest("GET", "https://api.anthropic.com/v1/models?limit=100", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", modelsBetaHeader)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("models API returned %d: %s", resp.StatusCode, body)
	}

	var apiResp claudeModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode models: %w", err)
	}

	var models []ModelInfo
	for _, m := range apiResp.Data {
		if !isCurrentGen(m.ID) {
			continue
		}
		models = append(models, ModelInfo{
			ID:        m.ID,
			Name:      m.DisplayName,
			CreatedAt: m.CreatedAt,
			MaxTokens: maxTokensForModel(m.ID),
		})
	}
	return models, nil
}

// probeModel tests whether inference works for a given model ID by sending a
// minimal 1-token request. Returns true if the model responds with HTTP 200.
// This is used to filter out models the OAuth token can't access.
func (s *Server) probeModel(modelID string) bool {
	if s.auth == nil {
		return false
	}
	token, err := s.auth.Token()
	if err != nil {
		return false
	}

	probeBody := map[string]interface{}{
		"model":      modelID,
		"max_tokens": 1,
		"messages": []map[string]interface{}{
			{"role": "user", "content": []map[string]interface{}{
				{"type": "text", "text": "hi"},
			}},
		},
	}
	bodyBytes, err := json.Marshal(probeBody)
	if err != nil {
		return false
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", modelsBetaHeader)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	ok := resp.StatusCode == http.StatusOK
	if !ok {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("models: probe rejected model %q (status=%d, body=%s)", modelID, resp.StatusCode, truncateBody(body, 200))
	}
	return ok
}

// probeModels filters the model list to only those that can successfully
// perform inference with the current OAuth token. Results are NOT cached —
// this should only be called when refreshing the model cache.
func (s *Server) probeModels(models []ModelInfo) []ModelInfo {
	var working []ModelInfo
	for _, m := range models {
		if s.probeModel(m.ID) {
			working = append(working, m)
			log.Printf("models: probe OK for %q", m.ID)
		}
	}
	return working
}

// truncateBody returns up to maxLen bytes of body as a string.
func truncateBody(b []byte, maxLen int) string {
	if len(b) <= maxLen {
		return string(b)
	}
	return string(b[:maxLen]) + "...(truncated)"
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	s.modelCache.mu.RLock()
	cached := s.modelCache.models
	valid := time.Since(s.modelCache.fetchedAt) < s.modelCache.ttl
	s.modelCache.mu.RUnlock()

	if valid && cached != nil {
		writeJSON(w, http.StatusOK, modelsResponse{Models: cached, ThinkingTypes: defaultThinkingTypes})
		return
	}

	models, err := s.fetchModels()
	if err != nil {
		if cached != nil {
			writeJSON(w, http.StatusOK, modelsResponse{Models: cached, ThinkingTypes: defaultThinkingTypes})
			return
		}
		// API unreachable — use fallback (haiku-first order).
		writeJSON(w, http.StatusOK, modelsResponse{Models: fallbackModels, ThinkingTypes: defaultThinkingTypes})
		return
	}

	// Trust the /v1/models listing — it returns only models the account has access to.
	// Probing (sending a 1-token inference request per model) was removed because
	// Claude Code OAuth tokens can list and use all subscription models via the
	// streaming API, but probing with non-streaming requests fails for some models
	// due to OAuth scope restrictions (user:sessions:claude_code).
	working := models
	if len(working) == 0 {
		log.Printf("models: API returned no current-gen models — using fallback list")
		working = fallbackModels
	}

	s.modelCache.mu.Lock()
	s.modelCache.models = working
	s.modelCache.fetchedAt = time.Now()
	s.modelCache.mu.Unlock()

	writeJSON(w, http.StatusOK, modelsResponse{Models: working, ThinkingTypes: defaultThinkingTypes})
}
