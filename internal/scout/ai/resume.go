package ai

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// ResumeTailor reads a baseline resume, tailors it to a lead's JD via Claude,
// stores the result as a lead artifact, and returns the tailored markdown.
func (s *Service) ResumeTailor(ctx context.Context, leadID int64) (string, error) {
	lead, err := s.store.GetLead(leadID)
	if err != nil {
		return "", fmt.Errorf("get lead: %w", err)
	}

	baselinePath := filepath.Join(s.dataDir, "resume-baseline.md")
	baseline, err := os.ReadFile(baselinePath)
	if err != nil {
		return "", fmt.Errorf("read baseline resume: %w", err)
	}

	system := "You are an expert resume tailor. Given a baseline resume and a job description, rewrite the resume to match the JD's keywords, skills, and requirements. Preserve all factual information. Emphasize relevant experience. Output the complete tailored resume in markdown format."

	userMsg := fmt.Sprintf("Baseline Resume:\n%s\n\nJob Title: %s\nCompany: %s\nSeniority: %s\nTechnologies: %s\n\nJob Description:\n%s",
		string(baseline), lead.JobTitle, lead.Company, lead.Seniority, lead.TechnologySlugs, lead.Description)

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return "", err
	}

	_, err = s.store.DB().ExecContext(ctx,
		"INSERT INTO lead_artifacts (lead_id, type, content) VALUES (?, 'resume', ?)",
		leadID, text)
	if err != nil {
		return text, fmt.Errorf("tailored resume generated but failed to persist: %w", err)
	}

	return text, nil
}
