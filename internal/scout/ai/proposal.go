package ai

import (
	"context"
	"fmt"
)

var validPlatforms = map[string]bool{
	"upwork":     true,
	"freelancer": true,
	"general":    true,
}

// ProposalGen generates a platform-tailored proposal for a lead.
func (s *Service) ProposalGen(ctx context.Context, leadID int64, platform string) (string, error) {
	if !validPlatforms[platform] {
		return "", fmt.Errorf("invalid platform %q — must be upwork, freelancer, or general", platform)
	}

	lead, err := s.store.GetLead(leadID)
	if err != nil {
		return "", fmt.Errorf("get lead: %w", err)
	}

	var profileSection string
	if s.profileDB != nil {
		if profile, err := s.fetchProfile(); err == nil {
			profileSection = fmt.Sprintf("\n\nProfile: %v", profile)
		}
	}

	var platformGuide string
	switch platform {
	case "upwork":
		platformGuide = "Write a short, punchy Upwork proposal. Mention the client's budget if available. Keep it under 200 words. Start with a hook that shows you read the job description."
	case "freelancer":
		platformGuide = "Write a competitive Freelancer.com bid. Emphasize value for money and fast delivery. Be specific about timeline and approach."
	default:
		platformGuide = "Write a professional cover letter style proposal. Be thorough but concise."
	}

	system := fmt.Sprintf("You are a proposal writing expert. %s", platformGuide)
	userMsg := fmt.Sprintf("Job: %s at %s\nDescription: %s%s", lead.JobTitle, lead.Company, lead.Description, profileSection)

	return s.sendAndExtractText(ctx, system, userMsg)
}
