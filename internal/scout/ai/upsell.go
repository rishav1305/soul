package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// UpsellResult is the structured output from upsell detection analysis.
type UpsellResult struct {
	UpsellScore   int              `json:"upsell_score"`
	Opportunities []UpsellOpportunity `json:"opportunities"`
	NextAction    string           `json:"next_action"`
	Urgency       string           `json:"urgency"`
}

// UpsellOpportunity describes a single upsell opportunity.
type UpsellOpportunity struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Confidence  int    `json:"confidence"`
}

// ContractUpsellDetector analyzes a contract lead for upsell opportunities.
// It examines the lead data and any existing artifacts to identify scope
// expansion, new service lines, or referral potential.
func (s *Service) ContractUpsellDetector(ctx context.Context, leadID int64) (*UpsellResult, error) {
	lead, err := s.store.GetLead(leadID)
	if err != nil {
		return nil, fmt.Errorf("get lead: %w", err)
	}

	// Gather existing artifacts for context.
	var artifactCtx string
	artifacts, err := s.store.GetArtifacts(leadID)
	if err == nil && len(artifacts) > 0 {
		var summaries []string
		for _, a := range artifacts {
			summary := fmt.Sprintf("- %s: %s", a.Type, truncate(a.Content, 200))
			summaries = append(summaries, summary)
		}
		artifactCtx = fmt.Sprintf("\n\nExisting Artifacts:\n%s", strings.Join(summaries, "\n"))
	}

	system := `You are an expert at identifying upsell opportunities in AI/ML contracts. Analyze the engagement and identify areas for scope expansion, new service lines, or referral opportunities. Return ONLY valid JSON: {"upsell_score": 0-100, "opportunities": [{"type": "scope_expansion|new_service|referral", "description": "...", "confidence": 0-100}], "next_action": "...", "urgency": "low|medium|high"}`

	userMsg := fmt.Sprintf("Company: %s\nJob Title: %s\nPipeline: %s\nStage: %s\nDescription: %s\nTechnologies: %s\nIndustry: %s%s",
		lead.Company, lead.JobTitle, lead.Pipeline, lead.Stage,
		lead.Description, lead.TechnologySlugs, lead.CompanyIndustry,
		artifactCtx)

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return nil, err
	}

	cleaned := strings.TrimSpace(extractJSON(text))
	var result UpsellResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("parse upsell result: %w (raw: %s)", err, text)
	}

	content, _ := json.Marshal(result)
	if _, err := s.store.AddArtifact(leadID, "upsell_detection", string(content)); err != nil {
		return &result, fmt.Errorf("result generated but failed to persist: %w", err)
	}

	return &result, nil
}

// truncate shortens a string to maxLen, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
