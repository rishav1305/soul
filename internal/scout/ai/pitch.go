package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rishav1305/soul-v2/internal/scout/agent"
)

// CompanyPitch generates a team augmentation pitch for a target company.
// Runs asynchronously — returns run_id immediately, poll GET /api/agent/status for results.
func (s *Service) CompanyPitch(ctx context.Context, leadID int64) (int64, error) {
	lead, err := s.store.GetLead(leadID)
	if err != nil {
		return 0, fmt.Errorf("get lead: %w", err)
	}

	var profileCtx string
	if s.profileDB != nil {
		profile, err := s.fetchProfile()
		if err == nil {
			pJSON, _ := json.Marshal(profile)
			profileCtx = fmt.Sprintf("\n\nOur Team Profile:\n%s", string(pJSON))
		}
	}

	prompt := fmt.Sprintf(`Generate a team augmentation pitch for %s.
Company: %s (%s)
Industry: %s
Size: %d employees
Funding: $%.0f (%s)
Job they're hiring for: %s
Job Description: %s%s

Create a multi-section pitch document:
1. Company Research — what they do, recent news, growth signals
2. Pain Points — what challenges they likely face based on the job posting
3. Proposed Engagement — how our team can help
4. Relevant Portfolio — projects that demonstrate our capabilities
5. Pricing — suggested engagement model and rate range

Return the pitch as structured text ready to send.`,
		lead.Company, lead.Company, lead.CompanyDomain,
		lead.CompanyIndustry, lead.CompanyEmployeeCount,
		lead.CompanyTotalFundingUSD, lead.CompanyFundingStage,
		lead.JobTitle, lead.Description, profileCtx)

	cfg := agent.LaunchConfig{
		Mode:    "pitch",
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
