package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// AdvisoryProposal is the structured output from advisory proposal generation.
type AdvisoryProposal struct {
	ExecutiveSummary string   `json:"executive_summary"`
	Scope            string   `json:"scope"`
	Deliverables     []string `json:"deliverables"`
	PricingModel     string   `json:"pricing_model"`
	Terms            string   `json:"terms"`
}

// AdvisoryProposalGen generates an advisory/retainer proposal for a consulting lead.
// The proposal is stored as a lead artifact of type "advisory_proposal".
func (s *Service) AdvisoryProposalGen(ctx context.Context, leadID int64) (*AdvisoryProposal, error) {
	lead, err := s.store.GetLead(leadID)
	if err != nil {
		return nil, fmt.Errorf("get lead: %w", err)
	}

	system := `You are an expert at writing advisory retainer proposals for AI consulting. Focus on ongoing strategic value, not project-based work. Return ONLY valid JSON: {"executive_summary": "...", "scope": "...", "deliverables": [...], "pricing_model": "...", "terms": "..."}`

	userMsg := fmt.Sprintf("Company: %s\nIndustry: %s\nEmployees: %d\nTopic/Description: %s\nContact: %s\nSeniority: %s",
		lead.Company, lead.CompanyIndustry, lead.CompanyEmployeeCount,
		lead.Description, lead.HiringManager, lead.Seniority)

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return nil, err
	}

	cleaned := strings.TrimSpace(extractJSON(text))
	var proposal AdvisoryProposal
	if err := json.Unmarshal([]byte(cleaned), &proposal); err != nil {
		return nil, fmt.Errorf("parse advisory proposal: %w (raw: %s)", err, text)
	}

	content, _ := json.Marshal(proposal)
	if _, err := s.store.AddArtifact(leadID, "advisory_proposal", string(content)); err != nil {
		return &proposal, fmt.Errorf("advisory proposal generated but failed to persist: %w", err)
	}

	return &proposal, nil
}
