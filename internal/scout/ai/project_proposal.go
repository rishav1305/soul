package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ProjectProposal is the structured output from project proposal generation.
type ProjectProposal struct {
	ProblemStatement string   `json:"problem_statement"`
	ProposedSolution string   `json:"proposed_solution"`
	Approach         string   `json:"approach"`
	Milestones       []string `json:"milestones"`
	Budget           string   `json:"budget"`
	Timeline         string   `json:"timeline"`
}

// ProjectProposalGen generates an AI/ML project proposal for a consulting lead.
// The proposal is stored as a lead artifact of type "project_proposal".
func (s *Service) ProjectProposalGen(ctx context.Context, leadID int64) (*ProjectProposal, error) {
	lead, err := s.store.GetLead(leadID)
	if err != nil {
		return nil, fmt.Errorf("get lead: %w", err)
	}

	system := `You are an expert at writing AI/ML project proposals. Be specific about technical approach, milestones, and deliverables. Return ONLY valid JSON: {"problem_statement": "...", "proposed_solution": "...", "approach": "...", "milestones": [...], "budget": "...", "timeline": "..."}`

	userMsg := fmt.Sprintf("Company: %s\nIndustry: %s\nEmployees: %d\nTopic/Description: %s\nTechnologies: %s\nContact: %s",
		lead.Company, lead.CompanyIndustry, lead.CompanyEmployeeCount,
		lead.Description, lead.TechnologySlugs, lead.HiringManager)

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return nil, err
	}

	cleaned := strings.TrimSpace(extractJSON(text))
	var proposal ProjectProposal
	if err := json.Unmarshal([]byte(cleaned), &proposal); err != nil {
		return nil, fmt.Errorf("parse project proposal: %w (raw: %s)", err, text)
	}

	content, _ := json.Marshal(proposal)
	if _, err := s.store.AddArtifact(leadID, "project_proposal", string(content)); err != nil {
		return &proposal, fmt.Errorf("project proposal generated but failed to persist: %w", err)
	}

	return &proposal, nil
}
