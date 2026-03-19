package ai

import (
	"context"
	"errors"
	"testing"
)

func TestFreelanceScore(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	lead.SalaryString = "$100-150/hr"
	lead.CompanyEmployeeCount = 50
	id, _ := st.AddLead(lead)

	sender := &mockSender{
		response: `{"score": 78, "skill_match": 90, "budget_fit": 70, "scope_clarity": 75, "client_quality": 80, "time_fit": 65, "reasoning": "Good match for Go skills"}`,
	}
	svc := New(st, nil, sender, t.TempDir())

	result, err := svc.FreelanceScore(context.Background(), id)
	if err != nil {
		t.Fatalf("freelance score: %v", err)
	}
	if result.Score != 78 {
		t.Errorf("score = %d, want 78", result.Score)
	}
	if result.SkillMatch != 90 {
		t.Errorf("skill_match = %d, want 90", result.SkillMatch)
	}
	if result.BudgetFit != 70 {
		t.Errorf("budget_fit = %d, want 70", result.BudgetFit)
	}
	if result.ScopeClarity != 75 {
		t.Errorf("scope_clarity = %d, want 75", result.ScopeClarity)
	}
	if result.ClientQuality != 80 {
		t.Errorf("client_quality = %d, want 80", result.ClientQuality)
	}
	if result.TimeFit != 65 {
		t.Errorf("time_fit = %d, want 65", result.TimeFit)
	}
	if result.Reasoning != "Good match for Go skills" {
		t.Errorf("reasoning = %q", result.Reasoning)
	}

	// Verify score was persisted.
	got, _ := st.GetLead(id)
	if got.MatchScore != 78 {
		t.Errorf("db match_score = %f, want 78", got.MatchScore)
	}
}

func TestFreelanceScore_CodeFence(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &mockSender{
		response: "```json\n{\"score\": 62, \"skill_match\": 80, \"budget_fit\": 50, \"scope_clarity\": 60, \"client_quality\": 70, \"time_fit\": 55, \"reasoning\": \"Decent gig\"}\n```",
	}
	svc := New(st, nil, sender, t.TempDir())

	result, err := svc.FreelanceScore(context.Background(), id)
	if err != nil {
		t.Fatalf("freelance score with code fence: %v", err)
	}
	if result.Score != 62 {
		t.Errorf("score = %d, want 62", result.Score)
	}
}

func TestFreelanceScore_LeadNotFound(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "anything"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.FreelanceScore(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for missing lead")
	}
}

func TestFreelanceScore_SenderError(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &errSender{err: errors.New("claude unavailable")}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.FreelanceScore(context.Background(), id)
	if err == nil {
		t.Fatal("expected error from sender")
	}
}

func TestFreelanceScore_BadJSON(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &mockSender{response: "not valid json at all"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.FreelanceScore(context.Background(), id)
	if err == nil {
		t.Fatal("expected error for bad JSON response")
	}
}
