package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

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

var currentGenPrefixes = []string{"claude-opus-4", "claude-sonnet-4", "claude-haiku-4"}

var defaultThinkingTypes = []string{"disabled", "adaptive", "enabled"}

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
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unable to fetch models"})
		return
	}

	s.modelCache.mu.Lock()
	s.modelCache.models = models
	s.modelCache.fetchedAt = time.Now()
	s.modelCache.mu.Unlock()

	writeJSON(w, http.StatusOK, modelsResponse{Models: models, ThinkingTypes: defaultThinkingTypes})
}
