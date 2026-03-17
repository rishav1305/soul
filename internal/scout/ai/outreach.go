package ai

import (
	"context"
	"fmt"
)

// ColdOutreach generates a personalized outreach email for a lead.
// Does NOT require profiledb — uses company data from the lead.
func (s *Service) ColdOutreach(ctx context.Context, leadID int64) (string, error) {
	lead, err := s.store.GetLead(leadID)
	if err != nil {
		return "", fmt.Errorf("get lead: %w", err)
	}

	system := "You are a cold outreach expert. Research the company context from the data provided and draft a personalized email. Don't pitch — offer value first. Identify a specific gap or opportunity and lead with that."
	userMsg := fmt.Sprintf("Company: %s\nDomain: %s\nIndustry: %s\nEmployees: %d\nFunding: $%.0f (%s)\nJob: %s\nDescription: %s",
		lead.Company, lead.CompanyDomain, lead.CompanyIndustry,
		lead.CompanyEmployeeCount, lead.CompanyTotalFundingUSD, lead.CompanyFundingStage,
		lead.JobTitle, lead.Description)

	return s.sendAndExtractText(ctx, system, userMsg)
}
