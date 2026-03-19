package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ReactiveContent is the structured output from reactive content generation.
type ReactiveContent struct {
	LinkedInPost string `json:"linkedin_post"`
	XPost        string `json:"x_post"`
	TimingNote   string `json:"timing_note"`
}

// ReactiveContentGen generates reactive content based on a news event or industry development.
// Does NOT require profiledb or a lead — standalone content tool.
func (s *Service) ReactiveContentGen(ctx context.Context, newsContext string, angle string) (*ReactiveContent, error) {
	if strings.TrimSpace(newsContext) == "" {
		return nil, fmt.Errorf("news context is required")
	}
	if strings.TrimSpace(angle) == "" {
		return nil, fmt.Errorf("angle is required")
	}

	system := `You are a reactive content expert for a senior AI engineer. Given a news event or industry development, create timely content that adds expert perspective. Be opinionated but substantive. Return ONLY valid JSON: {"linkedin_post": "...", "x_post": "...", "timing_note": "..."}`

	userMsg := fmt.Sprintf("News/Event: %s\nAngle: %s", newsContext, angle)

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return nil, err
	}

	cleaned := strings.TrimSpace(extractJSON(text))
	var result ReactiveContent
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("parse reactive content: %w (raw: %s)", err, text)
	}
	return &result, nil
}
