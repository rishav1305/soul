package ai

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// ExpertApplication generates a compelling expertise description for applying to
// an expert network. The result is saved to disk under expert-applications/.
func (s *Service) ExpertApplication(ctx context.Context, networkName string, focusArea string) (string, error) {
	system := "You are an expert network application writer. Write a compelling expertise description for an AI/ML expert applying to an expert network. Tailor to the network's client base. Be specific about domains: AI/LLM production, strategy, legal AI, healthcare AI, sales AI, e-commerce AI. Under 300 words."

	userMsg := fmt.Sprintf("Network: %s\nFocus Area: %s\nExpert Profile: Senior AI Engineer, 8+ years, built enterprise agentic AI platform (5000+ users), expertise in RAG, agents, LLM evaluation, production ML systems.",
		networkName, focusArea)

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return "", err
	}

	dir := filepath.Join(s.dataDir, "expert-applications")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return text, fmt.Errorf("expert application generated but failed to create directory: %w", err)
	}

	outPath := filepath.Join(dir, networkName+".md")
	if err := os.WriteFile(outPath, []byte(text), 0644); err != nil {
		return text, fmt.Errorf("expert application generated but failed to write file: %w", err)
	}

	return text, nil
}
