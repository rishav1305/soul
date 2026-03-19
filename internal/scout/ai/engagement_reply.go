package ai

import (
	"context"
	"fmt"
	"strings"
)

// EngagementReply generates a thoughtful reply to a LinkedIn or X post.
// Does NOT require profiledb or a lead — standalone content tool.
func (s *Service) EngagementReply(ctx context.Context, postContent string, authorContext string) (string, error) {
	if strings.TrimSpace(postContent) == "" {
		return "", fmt.Errorf("post content is required")
	}
	if strings.TrimSpace(authorContext) == "" {
		return "", fmt.Errorf("author context is required")
	}

	system := `You are writing a thoughtful reply to a LinkedIn or X post as a senior AI engineer. Add genuine value — share experience, ask a good question, or provide a counterpoint. Never be generic or sycophantic. Under 150 words.`

	userMsg := fmt.Sprintf("Post:\n%s\n\nAuthor Context: %s", postContent, authorContext)

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(text), nil
}
