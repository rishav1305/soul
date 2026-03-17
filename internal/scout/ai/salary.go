package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SalaryResult is the structured output from salary estimation.
type SalaryResult struct {
	Min       float64  `json:"min"`
	Median    float64  `json:"median"`
	Max       float64  `json:"max"`
	Currency  string   `json:"currency"`
	Reasoning string   `json:"reasoning"`
	Sources   []string `json:"sources"`
}

// SalaryLookup estimates market rate for a lead's role.
// Does NOT require profiledb.
func (s *Service) SalaryLookup(ctx context.Context, leadID int64) (*SalaryResult, error) {
	lead, err := s.store.GetLead(leadID)
	if err != nil {
		return nil, fmt.Errorf("get lead: %w", err)
	}

	system := "You are a compensation analyst. Estimate the market rate for this role. Return ONLY valid JSON: {\"min\": number, \"median\": number, \"max\": number, \"currency\": \"USD\", \"reasoning\": \"...\", \"sources\": [...]}"
	userMsg := fmt.Sprintf("Role: %s\nSeniority: %s\nLocation: %s\nCountry: %s\nCompany: %s\nIndustry: %s\nCompany Size: %d employees",
		lead.JobTitle, lead.Seniority, lead.Location, lead.Country,
		lead.Company, lead.CompanyIndustry, lead.CompanyEmployeeCount)

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return nil, err
	}

	cleaned := strings.TrimSpace(extractJSON(text))
	var result SalaryResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("parse salary result: %w", err)
	}
	return &result, nil
}
