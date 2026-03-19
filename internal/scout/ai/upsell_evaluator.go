package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// UpsellEvaluation is the structured output from upsell evaluation.
type UpsellEvaluation struct {
	Score               int      `json:"score"`
	Opportunities       []string `json:"opportunities"`
	RecommendedApproach string   `json:"recommended_approach"`
	Timing              string   `json:"timing"`
}

// ConsultingUpsellEvaluator evaluates upsell potential for a consulting engagement.
// It considers engagement depth, interaction count, and existing artifacts.
// The evaluation is stored as a lead artifact of type "upsell_evaluation".
func (s *Service) ConsultingUpsellEvaluator(ctx context.Context, leadID int64) (*UpsellEvaluation, error) {
	lead, err := s.store.GetLead(leadID)
	if err != nil {
		return nil, fmt.Errorf("get lead: %w", err)
	}

	interactionCount, err := s.store.GetInteractionCount(leadID)
	if err != nil {
		return nil, fmt.Errorf("get interaction count: %w", err)
	}

	artifacts, err := s.store.GetArtifacts(leadID)
	if err != nil {
		return nil, fmt.Errorf("get artifacts: %w", err)
	}

	system := `You are an expert at identifying upsell opportunities in AI consulting engagements. Analyze the engagement depth, client satisfaction signals, and technical expansion areas. Return ONLY valid JSON: {"score": 0, "opportunities": [...], "recommended_approach": "...", "timing": "..."} where score is 0-100.`

	var artifactSummary strings.Builder
	for _, a := range artifacts {
		fmt.Fprintf(&artifactSummary, "- [%s] %s (%d chars)\n", a.CreatedAt, a.Type, len(a.Content))
	}

	userMsg := fmt.Sprintf("Company: %s\nIndustry: %s\nEmployees: %d\nStage: %s\nPipeline: %s\nTopic/Description: %s\nInteraction Count: %d\nWarmth: %s\nContact: %s\n\nExisting Artifacts:\n%s",
		lead.Company, lead.CompanyIndustry, lead.CompanyEmployeeCount,
		lead.Stage, lead.Pipeline, lead.Description,
		interactionCount, lead.Warmth, lead.HiringManager,
		artifactSummary.String())

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return nil, err
	}

	cleaned := strings.TrimSpace(extractJSON(text))
	var eval UpsellEvaluation
	if err := json.Unmarshal([]byte(cleaned), &eval); err != nil {
		return nil, fmt.Errorf("parse upsell evaluation: %w (raw: %s)", err, text)
	}

	content, _ := json.Marshal(eval)
	if _, err := s.store.AddArtifact(leadID, "upsell_evaluation", string(content)); err != nil {
		return &eval, fmt.Errorf("upsell evaluation generated but failed to persist: %w", err)
	}

	return &eval, nil
}
