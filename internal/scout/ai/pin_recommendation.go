package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// PinRecommendationItem represents a single post pin recommendation.
type PinRecommendationItem struct {
	PostID          int64  `json:"post_id"`
	Topic           string `json:"topic"`
	Reason          string `json:"reason"`
	EngagementScore int    `json:"engagement_score"`
}

// PinRecommendationResult is the structured output from pin recommendation analysis.
type PinRecommendationResult struct {
	Recommendations []PinRecommendationItem `json:"recommendations"`
	PinStrategy     string                  `json:"pin_strategy"`
}

// PinRecommendation analyzes published content posts on a platform and recommends
// which ones to pin/feature based on engagement and relevance.
func (s *Service) PinRecommendation(ctx context.Context, platform string) (*PinRecommendationResult, error) {
	posts, err := s.store.ListContentPosts(platform, "published")
	if err != nil {
		return nil, fmt.Errorf("list content posts: %w", err)
	}

	if len(posts) == 0 {
		return &PinRecommendationResult{
			Recommendations: []PinRecommendationItem{},
			PinStrategy:     "No published posts found for " + platform + ". Create and publish content first.",
		}, nil
	}

	postsJSON, err := json.Marshal(posts)
	if err != nil {
		return nil, fmt.Errorf("marshal posts: %w", err)
	}

	system := "You are a content curation expert. Analyze these published posts and recommend which ones to pin/feature on the profile. Consider: engagement metrics, topic relevance for AI consulting, recency, and professional positioning. Return ONLY valid JSON: {\"recommendations\": [{\"post_id\": N, \"topic\": \"...\", \"reason\": \"...\", \"engagement_score\": N}], \"pin_strategy\": \"...\"}"

	userMsg := fmt.Sprintf("Platform: %s\n\nPublished Posts:\n%s", platform, string(postsJSON))

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return nil, err
	}

	cleaned := strings.TrimSpace(extractJSON(text))
	var result PinRecommendationResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("parse pin recommendation result: %w (raw: %s)", err, text)
	}

	return &result, nil
}
