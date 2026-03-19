package ai

import (
	"context"
	"fmt"
)

// TestimonialRequest generates a warm testimonial request message for a completed engagement.
// The message is stored as a lead artifact of type "testimonial_request".
func (s *Service) TestimonialRequest(ctx context.Context, leadID int64) (string, error) {
	lead, err := s.store.GetLead(leadID)
	if err != nil {
		return "", fmt.Errorf("get lead: %w", err)
	}

	system := "You are an expert at requesting client testimonials. Write a warm, specific request that makes it easy for the client to respond. Reference the specific project and outcomes. Include suggested talking points. Keep under 200 words."

	userMsg := fmt.Sprintf("Client/Company: %s\nProject/Role: %s\nDescription: %s",
		lead.Company, lead.JobTitle, lead.Description)

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return "", err
	}

	if _, err := s.store.AddArtifact(leadID, "testimonial_request", text); err != nil {
		return text, fmt.Errorf("testimonial request generated but failed to persist: %w", err)
	}

	return text, nil
}
