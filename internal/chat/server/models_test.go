package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMaxTokensForModel_KnownModels(t *testing.T) {
	tests := []struct {
		modelID  string
		expected int
	}{
		{"claude-opus-4-20250514", 64000},
		{"claude-sonnet-4-20250514", 64000},
		{"claude-haiku-4-5-20251001", 64000},
	}
	for _, tt := range tests {
		got := maxTokensForModel(tt.modelID)
		if got != tt.expected {
			t.Errorf("maxTokensForModel(%q) = %d, want %d", tt.modelID, got, tt.expected)
		}
	}
}

func TestMaxTokensForModel_UnknownModel(t *testing.T) {
	got := maxTokensForModel("gpt-4")
	if got != 16384 {
		t.Errorf("unknown model should return 16384, got %d", got)
	}
}

func TestIsCurrentGen(t *testing.T) {
	tests := []struct {
		modelID  string
		expected bool
	}{
		{"claude-haiku-4-5-20251001", true},
		{"claude-haiku-4-20240307", true},
		{"claude-opus-4-20250514", false}, // Not in currentGenPrefixes (removed per CEO)
		{"claude-sonnet-4-20250514", false},
		{"gpt-4", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isCurrentGen(tt.modelID)
		if got != tt.expected {
			t.Errorf("isCurrentGen(%q) = %v, want %v", tt.modelID, got, tt.expected)
		}
	}
}

func TestTruncateBody(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello...(truncated)"},
		{"", 10, ""},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc...(truncated)"},
	}
	for _, tt := range tests {
		got := truncateBody([]byte(tt.input), tt.maxLen)
		if got != tt.expected {
			t.Errorf("truncateBody(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.expected)
		}
	}
}

func TestHandleModels_FallbackWhenNoAuth(t *testing.T) {
	// Server without auth — should return fallback models.
	srv := New(WithPort(0))
	req := httptest.NewRequest("GET", "/api/models", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp modelsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if len(resp.Models) == 0 {
		t.Error("expected at least 1 fallback model")
	}

	// Should include Haiku.
	foundHaiku := false
	for _, m := range resp.Models {
		if m.ID == "claude-haiku-4-5-20251001" {
			foundHaiku = true
		}
	}
	if !foundHaiku {
		t.Error("expected Haiku 4.5 in fallback models")
	}

	if len(resp.ThinkingTypes) != 3 {
		t.Errorf("expected 3 thinking types, got %d", len(resp.ThinkingTypes))
	}
}

func TestHandleModels_CacheHit(t *testing.T) {
	srv := New(WithPort(0))

	// Warm the cache by hitting the endpoint once.
	req1 := httptest.NewRequest("GET", "/api/models", nil)
	rec1 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec1, req1)

	// Second request should hit cache.
	req2 := httptest.NewRequest("GET", "/api/models", nil)
	rec2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 on cache hit, got %d", rec2.Code)
	}

	var resp1, resp2 modelsResponse
	json.NewDecoder(rec1.Body).Decode(&resp1)
	json.NewDecoder(rec2.Body).Decode(&resp2)

	if len(resp1.Models) != len(resp2.Models) {
		t.Errorf("cache miss: first=%d, second=%d models", len(resp1.Models), len(resp2.Models))
	}
}
