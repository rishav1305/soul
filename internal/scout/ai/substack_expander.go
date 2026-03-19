package ai

import (
	"context"
	"fmt"
	"strings"
)

// SubstackExpander expands a LinkedIn post into a long-form Substack article.
// Does NOT require profiledb or a lead — standalone content tool.
func (s *Service) SubstackExpander(ctx context.Context, post string, topic string) (string, error) {
	if strings.TrimSpace(post) == "" {
		return "", fmt.Errorf("post content is required")
	}
	if strings.TrimSpace(topic) == "" {
		return "", fmt.Errorf("topic is required")
	}

	system := `You are an expert at expanding short LinkedIn posts into long-form Substack articles. Add depth, examples, data points, and actionable frameworks. Target 1500-2000 words. Return the full article in markdown format.`

	userMsg := fmt.Sprintf("Topic: %s\n\nLinkedIn Post:\n%s", topic, post)

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(text), nil
}
