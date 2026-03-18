package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// MatchResult is the structured output from resume matching.
type MatchResult struct {
	Score       int      `json:"score"`
	Strengths   []string `json:"strengths"`
	Gaps        []string `json:"gaps"`
	Suggestions []string `json:"suggestions"`
}

// ResumeMatch scores a lead's JD against the user's profile.
func (s *Service) ResumeMatch(ctx context.Context, leadID int64) (*MatchResult, error) {
	lead, err := s.store.GetLead(leadID)
	if err != nil {
		return nil, fmt.Errorf("get lead: %w", err)
	}

	var profileCtx string
	if s.profileDB != nil {
		profile, err := s.fetchProfile()
		if err == nil {
			pJSON, _ := json.Marshal(profile)
			profileCtx = fmt.Sprintf("\n\nCandidate Profile:\n%s", string(pJSON))
		}
	}

	system := "You are a resume-to-JD matching expert. Score the candidate against the job description. Return ONLY valid JSON: {\"score\": 0-100, \"strengths\": [...], \"gaps\": [...], \"suggestions\": [...]}"

	userMsg := fmt.Sprintf("Job Title: %s\nSeniority: %s\nLocation: %s\nTechnologies: %s\n\nJob Description:\n%s%s",
		lead.JobTitle, lead.Seniority, lead.Location, lead.TechnologySlugs,
		lead.Description, profileCtx)

	text, err := s.sendAndExtractText(ctx, system, userMsg)
	if err != nil {
		return nil, err
	}

	cleaned := strings.TrimSpace(extractJSON(text))
	var result MatchResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("parse match result: %w (raw: %s)", err, text)
	}

	if err := s.store.UpdateLead(leadID, map[string]interface{}{"match_score": float64(result.Score)}); err != nil {
		return &result, fmt.Errorf("scored %d but failed to persist: %w", result.Score, err)
	}

	return &result, nil
}

// ScoreLead implements sweep.Scorer interface.
// Uses a 30s timeout to prevent hanging the sweep on a stuck Claude call.
func (s *Service) ScoreLead(leadID int64) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := s.ResumeMatch(ctx, leadID)
	if err != nil {
		return 0, err
	}
	return float64(result.Score), nil
}
