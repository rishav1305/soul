package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// HookResult is the structured output from hook generation.
type HookResult struct {
	Hooks []string `json:"hooks"`
}

// HookWriter generates 5 alternative first lines for a draft post.
// Does NOT require profiledb or a lead — takes raw text input.
func (s *Service) HookWriter(ctx context.Context, draft string) (*HookResult, error) {
	system := `You are a LinkedIn hook writing expert. Given a draft post, generate 5 alternative first lines using these formulas: counterintuitive claim, specific number, hard-won lesson, provocative question, confession. Return ONLY valid JSON: {"hooks": ["hook1", "hook2", "hook3", "hook4", "hook5"]}`

	text, err := s.sendAndExtractText(ctx, system, draft)
	if err != nil {
		return nil, err
	}

	cleaned := strings.TrimSpace(extractJSON(text))
	var result HookResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("parse hook result: %w (raw: %s)", err, text)
	}
	return &result, nil
}
