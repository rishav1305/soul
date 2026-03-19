package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// FreelanceScoreResult is the structured output from freelance gig scoring.
type FreelanceScoreResult struct {
	Score         int    `json:"score"`
	SkillMatch    int    `json:"skill_match"`
	BudgetFit     int    `json:"budget_fit"`
	ScopeClarity  int    `json:"scope_clarity"`
	ClientQuality int    `json:"client_quality"`
	TimeFit       int    `json:"time_fit"`
	Reasoning     string `json:"reasoning"`
}

// FreelanceScore evaluates a freelance gig lead on 5 criteria via Claude,
// persists the overall score, and returns the full breakdown.
func (s *Service) FreelanceScore(ctx context.Context, leadID int64) (*FreelanceScoreResult, error) {
	lead, err := s.store.GetLead(leadID)
	if err != nil {
		return nil, fmt.Errorf("get lead: %w", err)
	}

	system := `You are a freelance gig evaluator. Score this gig 0-100 on 5 criteria. Return ONLY valid JSON: {"score": 0-100, "skill_match": 0-100, "budget_fit": 0-100, "scope_clarity": 0-100, "client_quality": 0-100, "time_fit": 0-100, "reasoning": "..."}`

	userMsg := fmt.Sprintf("Job Title: %s\nCompany: %s\nBudget/Salary: %s\nCompany Size: %d employees\n\nDescription:\n%s",
		lead.JobTitle, lead.Company, lead.SalaryString, lead.CompanyEmployeeCount, lead.Description)

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return nil, err
	}

	cleaned := strings.TrimSpace(extractJSON(text))
	var result FreelanceScoreResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("parse freelance score: %w (raw: %s)", err, text)
	}

	if err := s.store.UpdateLead(leadID, map[string]interface{}{"match_score": float64(result.Score)}); err != nil {
		return &result, fmt.Errorf("scored %d but failed to persist: %w", result.Score, err)
	}

	return &result, nil
}
