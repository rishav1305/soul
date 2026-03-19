package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SOW is the structured output from a Statement of Work generation.
type SOW struct {
	Scope        string   `json:"scope"`
	Deliverables []string `json:"deliverables"`
	Timeline     string   `json:"timeline"`
	Pricing      string   `json:"pricing"`
	Assumptions  []string `json:"assumptions"`
}

// SOWGenerator generates a Statement of Work for an AI/ML consulting project.
// The SOW is stored as a lead artifact of type "sow".
func (s *Service) SOWGenerator(ctx context.Context, leadID int64) (*SOW, error) {
	lead, err := s.store.GetLead(leadID)
	if err != nil {
		return nil, fmt.Errorf("get lead: %w", err)
	}

	system := `You are an expert at writing Statements of Work for AI/ML consulting projects. Given the company and project details, create a professional SOW. Return ONLY valid JSON: {"scope": "...", "deliverables": ["..."], "timeline": "...", "pricing": "...", "assumptions": ["..."]}`

	userMsg := fmt.Sprintf("Company: %s\nIndustry: %s\nEmployees: %d\nProject/Description: %s\nJob Title: %s\nSeniority: %s\nTechnologies: %s",
		lead.Company, lead.CompanyIndustry, lead.CompanyEmployeeCount,
		lead.Description, lead.JobTitle, lead.Seniority, lead.TechnologySlugs)

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return nil, err
	}

	cleaned := strings.TrimSpace(extractJSON(text))
	var sow SOW
	if err := json.Unmarshal([]byte(cleaned), &sow); err != nil {
		return nil, fmt.Errorf("parse sow: %w (raw: %s)", err, text)
	}

	content, _ := json.Marshal(sow)
	if _, err := s.store.AddArtifact(leadID, "sow", string(content)); err != nil {
		return &sow, fmt.Errorf("sow generated but failed to persist: %w", err)
	}

	return &sow, nil
}
