package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// TopicSuggestion is a single content topic suggestion.
type TopicSuggestion struct {
	Topic  string `json:"topic"`
	Pillar string `json:"pillar"`
	Angle  string `json:"angle"`
}

// TopicSuggestions is the structured output from topic generation.
type TopicSuggestions struct {
	Suggestions []TopicSuggestion `json:"suggestions"`
}

// ContentTopicGen suggests content topics based on a week's work summary.
// Does NOT require profiledb or a lead — takes raw text input.
func (s *Service) ContentTopicGen(ctx context.Context, weekSummary string) (*TopicSuggestions, error) {
	system := `You are a content strategist for a senior AI engineer. Suggest 3 content topics based on this week's work. Each topic should map to a content pillar (builder_insights, technical_takes, career_consulting, personal_process). Return ONLY valid JSON: {"suggestions": [{"topic": "...", "pillar": "...", "angle": "..."}]}`

	userMsg := fmt.Sprintf("This week I worked on: %s", weekSummary)

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return nil, err
	}

	cleaned := strings.TrimSpace(extractJSON(text))
	var result TopicSuggestions
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("parse topic suggestions: %w (raw: %s)", err, text)
	}
	return &result, nil
}
