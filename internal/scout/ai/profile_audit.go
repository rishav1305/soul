package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ProfileAuditResult is the structured output from a profile audit.
type ProfileAuditResult struct {
	Score              int      `json:"score"`
	Strengths          []string `json:"strengths"`
	Gaps               []string `json:"gaps"`
	Recommendations    []string `json:"recommendations"`
	KeywordSuggestions []string `json:"keyword_suggestions"`
}

// ProfileAudit audits a professional profile for completeness, SEO, and positioning.
// This is a standalone tool that does not require a lead or store persistence.
func (s *Service) ProfileAudit(ctx context.Context, platform string, currentProfile string) (*ProfileAuditResult, error) {
	system := "You are a professional profile audit expert. Analyze this profile for completeness, SEO optimization, and positioning for AI/ML senior roles. Score 0-100 and provide specific, actionable recommendations. Return ONLY valid JSON: {\"score\": 0-100, \"strengths\": [...], \"gaps\": [...], \"recommendations\": [...], \"keyword_suggestions\": [...]}"

	userMsg := fmt.Sprintf("Platform: %s\n\nCurrent Profile:\n%s", platform, currentProfile)

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return nil, err
	}

	cleaned := strings.TrimSpace(extractJSON(text))
	var result ProfileAuditResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("parse profile audit result: %w (raw: %s)", err, text)
	}

	return &result, nil
}
