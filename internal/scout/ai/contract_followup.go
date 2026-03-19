package ai

import (
	"context"
	"fmt"
	"strings"
)

// ContractFollowUp generates a follow-up message for a contract lead based on
// the current stage and interaction history. The message is stored as a lead
// artifact of type "contract_followup".
func (s *Service) ContractFollowUp(ctx context.Context, leadID int64) (string, error) {
	lead, err := s.store.GetLead(leadID)
	if err != nil {
		return "", fmt.Errorf("get lead: %w", err)
	}

	interactions, err := s.store.GetInteractions(leadID)
	if err != nil {
		return "", fmt.Errorf("get interactions: %w", err)
	}

	// Build interaction summary for the prompt.
	var interactionSummary string
	if len(interactions) == 0 {
		interactionSummary = "No prior interactions recorded."
	} else {
		var parts []string
		for _, ix := range interactions {
			parts = append(parts, fmt.Sprintf("- [%s] %s via %s: %s", ix.CreatedAt, ix.Type, ix.Channel, ix.Description))
		}
		interactionSummary = strings.Join(parts, "\n")
	}

	system := `You are an expert contract follow-up specialist for AI/ML consulting engagements. Given the lead details, current pipeline stage, and interaction history, draft a professional follow-up message. The message should be appropriate for the current stage — early stages need discovery questions, mid stages need value reinforcement, late stages need closing language. Output the follow-up message text only — no JSON wrapping.`

	userMsg := fmt.Sprintf("Company: %s\nIndustry: %s\nJob/Project: %s\nDescription: %s\nPipeline: %s\nStage: %s\nWarmth: %s\nInteraction Count: %d\n\nInteraction History:\n%s",
		lead.Company, lead.CompanyIndustry, lead.JobTitle, lead.Description,
		lead.Pipeline, lead.Stage, lead.Warmth, lead.InteractionCount,
		interactionSummary)

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return "", err
	}

	if _, err := s.store.AddArtifact(leadID, "contract_followup", text); err != nil {
		return text, fmt.Errorf("follow-up generated but failed to persist: %w", err)
	}

	return text, nil
}
