package ai

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// LinkedInUpdate rewrites a LinkedIn profile section to maximize visibility
// for AI/ML roles and consulting opportunities.
func (s *Service) LinkedInUpdate(ctx context.Context, section string, currentContent string) (string, error) {
	system := "You are a LinkedIn profile optimization expert for a senior AI engineer. Rewrite the given profile section to maximize visibility for AI/ML roles and consulting opportunities. Use keywords: AI, LLM, production ML, agentic AI, RAG, Claude. Be specific about achievements."

	userMsg := fmt.Sprintf("Section: %s\n\nCurrent Content:\n%s", section, currentContent)

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return "", err
	}

	// Write to disk
	dir := filepath.Join(s.dataDir, "linkedin-updates")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create linkedin-updates dir: %w", err)
	}
	outPath := filepath.Join(dir, section+".md")
	if err := os.WriteFile(outPath, []byte(text), 0644); err != nil {
		return "", fmt.Errorf("write linkedin update: %w", err)
	}

	return text, nil
}
