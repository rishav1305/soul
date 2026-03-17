package ai

import (
	"context"
	"encoding/json"
	"fmt"
)

// CoverLetter generates a tailored cover letter for a lead.
// Requires profiledb to be configured.
func (s *Service) CoverLetter(ctx context.Context, leadID int64) (string, error) {
	lead, err := s.store.GetLead(leadID)
	if err != nil {
		return "", fmt.Errorf("get lead: %w", err)
	}

	profile, err := s.fetchProfile()
	if err != nil {
		return "", err // "profiledb not configured" error
	}

	profileJSON, _ := json.Marshal(profile)
	system := "You are an expert cover letter writer. Match the candidate's experience to the job description keywords. Write a compelling, personalized cover letter ready to paste."
	userMsg := fmt.Sprintf("Job Title: %s\nCompany: %s\nDescription: %s\n\nCandidate Profile:\n%s",
		lead.JobTitle, lead.Company, lead.Description, string(profileJSON))

	return s.sendAndExtractText(ctx, system, userMsg)
}
