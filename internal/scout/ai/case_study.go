package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// CaseStudy is the structured output from a case study draft.
type CaseStudy struct {
	Title             string `json:"title"`
	Challenge         string `json:"challenge"`
	Approach          string `json:"approach"`
	Results           string `json:"results"`
	TestimonialPrompt string `json:"testimonial_prompt"`
}

// CaseStudyDraft generates a case study draft for a completed or in-progress
// contract engagement. The case study is stored as a lead artifact of type "case_study".
func (s *Service) CaseStudyDraft(ctx context.Context, leadID int64) (*CaseStudy, error) {
	lead, err := s.store.GetLead(leadID)
	if err != nil {
		return nil, fmt.Errorf("get lead: %w", err)
	}

	system := `You are an expert at drafting AI project case studies that showcase technical depth and business impact. Given the project details, create a compelling case study. Return ONLY valid JSON: {"title": "...", "challenge": "...", "approach": "...", "results": "...", "testimonial_prompt": "..."}`

	userMsg := fmt.Sprintf("Company: %s\nIndustry: %s\nEmployees: %d\nProject/Description: %s\nJob Title: %s\nTechnologies: %s\nPipeline: %s\nStage: %s",
		lead.Company, lead.CompanyIndustry, lead.CompanyEmployeeCount,
		lead.Description, lead.JobTitle, lead.TechnologySlugs,
		lead.Pipeline, lead.Stage)

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return nil, err
	}

	cleaned := strings.TrimSpace(extractJSON(text))
	var cs CaseStudy
	if err := json.Unmarshal([]byte(cleaned), &cs); err != nil {
		return nil, fmt.Errorf("parse case study: %w (raw: %s)", err, text)
	}

	content, _ := json.Marshal(cs)
	if _, err := s.store.AddArtifact(leadID, "case_study", string(content)); err != nil {
		return &cs, fmt.Errorf("case study generated but failed to persist: %w", err)
	}

	return &cs, nil
}
