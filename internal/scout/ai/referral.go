package ai

import (
	"context"
	"fmt"

	"github.com/rishav1305/soul-v2/internal/scout/agent"
)

// ReferralFinder searches for LinkedIn connections at a target company.
// Runs asynchronously — returns run_id immediately, poll GET /api/agent/status for results.
func (s *Service) ReferralFinder(ctx context.Context, leadID int64) (int64, error) {
	lead, err := s.store.GetLead(leadID)
	if err != nil {
		return 0, fmt.Errorf("get lead: %w", err)
	}

	prompt := fmt.Sprintf(`Find mutual LinkedIn connections at %s (%s).
Company LinkedIn: %s
Hiring Manager: %s (%s)

Search for people at this company who might be connections or mutual contacts.
Return JSON: {"connections": [{"name": "...", "role": "...", "relationship": "..."}], "referral_template": "..."}`,
		lead.Company, lead.CompanyDomain,
		lead.CompanyLinkedInURL,
		lead.HiringManager, lead.HiringManagerLinkedIn)

	cfg := agent.LaunchConfig{
		Mode:    "referral",
		LeadID:  leadID,
		Prompt:  prompt,
		DataDir: s.dataDir,
	}

	result, err := agent.Launch(ctx, s.store, cfg)
	if err != nil {
		return 0, err
	}
	return result.RunID, nil
}
