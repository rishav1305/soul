package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// PrepBrief is the structured output from call preparation.
type PrepBrief struct {
	CompanyBackground  string   `json:"company_background"`
	LikelyQuestions    []string `json:"likely_questions"`
	RelevantExperience []string `json:"relevant_experience"`
	KeyDataPoints      []string `json:"key_data_points"`
}

// CallPrepBrief generates a consulting call preparation brief for a lead.
// The brief is stored as a lead artifact of type "call_prep".
func (s *Service) CallPrepBrief(ctx context.Context, leadID int64) (*PrepBrief, error) {
	lead, err := s.store.GetLead(leadID)
	if err != nil {
		return nil, fmt.Errorf("get lead: %w", err)
	}

	system := `You are preparing an AI expert for a consulting call. Given the company and topic, create a prep brief. Return ONLY valid JSON: {"company_background": "...", "likely_questions": [...], "relevant_experience": [...], "key_data_points": [...]}`

	userMsg := fmt.Sprintf("Company: %s\nIndustry: %s\nEmployees: %d\nTopic/Description: %s",
		lead.Company, lead.CompanyIndustry, lead.CompanyEmployeeCount, lead.Description)

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return nil, err
	}

	cleaned := strings.TrimSpace(extractJSON(text))
	var brief PrepBrief
	if err := json.Unmarshal([]byte(cleaned), &brief); err != nil {
		return nil, fmt.Errorf("parse prep brief: %w (raw: %s)", err, text)
	}

	content, _ := json.Marshal(brief)
	if _, err := s.store.AddArtifact(leadID, "call_prep", string(content)); err != nil {
		return &brief, fmt.Errorf("prep brief generated but failed to persist: %w", err)
	}

	return &brief, nil
}
