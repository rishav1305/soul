package ai

import (
	"context"
	"fmt"
	"strings"
)

// ConsultingFollowUp generates a follow-up message for a consulting engagement.
// It uses the lead's stage and interaction history to craft a contextual message.
// The result is stored as a lead artifact of type "consulting_followup".
func (s *Service) ConsultingFollowUp(ctx context.Context, leadID int64) (string, error) {
	lead, err := s.store.GetLead(leadID)
	if err != nil {
		return "", fmt.Errorf("get lead: %w", err)
	}

	interactions, err := s.store.GetInteractions(leadID)
	if err != nil {
		return "", fmt.Errorf("get interactions: %w", err)
	}

	system := "You are an expert consulting relationship manager. Write a follow-up message for a consulting engagement. Reference previous interactions. Be professional but warm."

	var interactionSummary strings.Builder
	for i, ix := range interactions {
		if i >= 10 {
			break // Cap at 10 most recent interactions
		}
		fmt.Fprintf(&interactionSummary, "- [%s] %s (%s): %s\n", ix.CreatedAt, ix.Type, ix.Channel, ix.Description)
	}

	userMsg := fmt.Sprintf("Company: %s\nIndustry: %s\nStage: %s\nJob/Topic: %s\nDescription: %s\nContact: %s\nWarmth: %s\n\nInteraction History:\n%s",
		lead.Company, lead.CompanyIndustry, lead.Stage, lead.JobTitle, lead.Description,
		lead.HiringManager, lead.Warmth, interactionSummary.String())

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return "", err
	}

	if _, err := s.store.AddArtifact(leadID, "consulting_followup", text); err != nil {
		return text, fmt.Errorf("follow-up generated but failed to persist: %w", err)
	}

	return text, nil
}
