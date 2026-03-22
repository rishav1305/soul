package ai

import (
	"context"
	"encoding/json"
	"fmt"
)

// CoverLetter generates a tailored cover letter for a lead.
// If profiledb is not configured, generates from JD data alone.
func (s *Service) CoverLetter(ctx context.Context, leadID int64) (string, error) {
	lead, err := s.store.GetLead(leadID)
	if err != nil {
		return "", fmt.Errorf("get lead: %w", err)
	}

	var profileSection string
	if s.profileDB != nil {
		if profile, err := s.fetchProfile(); err == nil {
			profileJSON, _ := json.Marshal(profile)
			profileSection = fmt.Sprintf("\n\nCandidate Profile:\n%s", string(profileJSON))
		}
	}

	system := "You are an expert cover letter writer. Match the candidate's experience to the job description keywords. Write a compelling, personalized cover letter ready to paste."
	userMsg := fmt.Sprintf("Job Title: %s\nCompany: %s\nDescription: %s%s",
		lead.JobTitle, lead.Company, lead.Description, profileSection)

	return s.sendAndExtractText(ctx, system, userMsg)
}
