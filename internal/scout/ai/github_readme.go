package ai

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// GitHubREADMEGen generates a compelling README.md for an AI/ML project.
func (s *Service) GitHubREADMEGen(ctx context.Context, repoName string, description string) (string, error) {
	system := "You are a GitHub README expert. Write a compelling README for an AI/ML project. Include: badges placeholder, clear description, features, quick start, architecture overview, tech stack, contributing guidelines. Make it stand out."

	userMsg := fmt.Sprintf("Repository: %s\n\nDescription:\n%s", repoName, description)

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return "", err
	}

	// Write to disk
	dir := filepath.Join(s.dataDir, "github-readmes")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create github-readmes dir: %w", err)
	}
	outPath := filepath.Join(dir, repoName+".md")
	if err := os.WriteFile(outPath, []byte(text), 0644); err != nil {
		return "", fmt.Errorf("write github readme: %w", err)
	}

	return text, nil
}
