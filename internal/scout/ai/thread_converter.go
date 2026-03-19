package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ThreadResult is the structured output from converting a LinkedIn post to an X/Twitter thread.
type ThreadResult struct {
	Thread       []string `json:"thread"`
	SummaryTweet string   `json:"summary_tweet"`
}

// ThreadConverter converts a LinkedIn post into an engaging X/Twitter thread.
// Does NOT require profiledb or a lead — standalone content tool.
func (s *Service) ThreadConverter(ctx context.Context, post string) (*ThreadResult, error) {
	if strings.TrimSpace(post) == "" {
		return nil, fmt.Errorf("post content is required")
	}

	system := `You are an expert at converting long-form LinkedIn posts into engaging X/Twitter threads. Each tweet under 280 chars. Use a hook tweet first, numbered thread, end with CTA. Also provide a single summary tweet. Return ONLY valid JSON: {"thread": ["tweet1", "tweet2", ...], "summary_tweet": "..."}`

	text, err := s.sendAndExtractText(ctx, system, post)
	if err != nil {
		return nil, err
	}

	cleaned := strings.TrimSpace(extractJSON(text))
	var result ThreadResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("parse thread result: %w (raw: %s)", err, text)
	}
	return &result, nil
}
