package ai

import (
	"context"
	"fmt"

	"github.com/rishav1305/soul/internal/chat/stream"
	"github.com/rishav1305/soul/internal/scout/profiledb"
	"github.com/rishav1305/soul/internal/scout/store"
)

// Sender sends a non-streaming request to Claude and returns the response.
// Implemented by stream.Client.Send. Tests inject a mock.
type Sender interface {
	Send(ctx context.Context, req *stream.Request) (*stream.Response, error)
}

// Service provides AI-powered tools for scout leads.
type Service struct {
	store     *store.Store
	profileDB *profiledb.Client // may be nil
	sender    Sender
	dataDir   string
}

// New creates a new AI service. profileDB may be nil.
func New(st *store.Store, pdb *profiledb.Client, sender Sender, dataDir string) *Service {
	return &Service{store: st, profileDB: pdb, sender: sender, dataDir: dataDir}
}

// HasProfileDB returns true if the profile database is configured.
func (s *Service) HasProfileDB() bool {
	return s.profileDB != nil
}

// fetchProfile returns the full profile or an error if profiledb is nil.
func (s *Service) fetchProfile() (map[string]interface{}, error) {
	if s.profileDB == nil {
		return nil, fmt.Errorf("profiledb not configured — set SOUL_SCOUT_PG_URL")
	}
	return s.profileDB.GetFullProfile()
}

// sendAndExtractText sends a request and returns the text content.
// Model is left empty so the stream client uses its default (currently
// claude-haiku-4-5-20251001, the only model accessible via OAuth beta).
func (s *Service) sendAndExtractText(ctx context.Context, system string, userMsg string) (string, error) {
	req := &stream.Request{
		MaxTokens: 4096,
		System:    system,
		Messages: []stream.Message{
			{Role: "user", Content: []stream.ContentBlock{{Type: "text", Text: userMsg}}},
		},
	}
	resp, err := s.sender.Send(ctx, req)
	if err != nil {
		return "", err
	}
	for _, block := range resp.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}
	return "", fmt.Errorf("no text content in response")
}

// extractJSON tries to extract JSON from a response that may contain markdown code fences.
func extractJSON(text string) string {
	// Try to find JSON in code fences first
	if start := indexOf(text, "```json"); start >= 0 {
		text = text[start+7:]
		if end := indexOf(text, "```"); end >= 0 {
			return text[:end]
		}
	}
	if start := indexOf(text, "```"); start >= 0 {
		text = text[start+3:]
		if end := indexOf(text, "```"); end >= 0 {
			return text[:end]
		}
	}
	return text
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
