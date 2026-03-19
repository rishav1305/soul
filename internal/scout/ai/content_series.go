package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ContentSeries is the structured output from content series generation.
type ContentSeries struct {
	LinkedInPosts   []string `json:"linkedin_posts"`
	XPosts          []string `json:"x_posts"`
	CarouselOutline string   `json:"carousel_outline"`
}

// ContentSeriesGen creates a multi-platform content series from a topic and raw insights.
// Does NOT require profiledb or a lead — takes raw text input.
func (s *Service) ContentSeriesGen(ctx context.Context, topic string, insights string) (*ContentSeries, error) {
	system := `You are an expert content strategist for a senior AI engineer. Given a topic and raw insights, create a 3-part LinkedIn content series + 3 X/Twitter posts + carousel outline. Return ONLY valid JSON: {"linkedin_posts": ["Part 1: hook post (100-150 words)", "Part 2: deep dive (300-500 words)", "Part 3: takeaway (200-300 words)"], "x_posts": ["single tweet", "thread opener for deep dive", "hot take tweet"], "carousel_outline": "slide-by-slide outline"}`

	userMsg := fmt.Sprintf("Topic: %s\nRaw Insights: %s", topic, insights)

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return nil, err
	}

	cleaned := strings.TrimSpace(extractJSON(text))
	var result ContentSeries
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("parse content series: %w (raw: %s)", err, text)
	}
	return &result, nil
}
