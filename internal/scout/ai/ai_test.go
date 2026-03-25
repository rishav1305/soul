package ai

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul/internal/chat/stream"
	"github.com/rishav1305/soul/internal/scout/store"
)

type mockSender struct {
	response string
}

func (m *mockSender) Send(ctx context.Context, req *stream.Request) (*stream.Response, error) {
	return &stream.Response{
		Content: []stream.ContentBlock{{Type: "text", Text: m.response}},
	}, nil
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func makeTestLead() store.Lead {
	return store.Lead{
		Source:          "theirstack",
		Pipeline:        "job",
		Stage:           "discovered",
		JobTitle:        "Senior Go Engineer",
		Company:         "Stripe",
		Location:        "Remote",
		CountryCode:     "US",
		Remote:          true,
		Seniority:       "senior",
		Description:     "Build distributed systems in Go",
		TechnologySlugs: `["go","react"]`,
	}
}

func TestResumeMatch(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &mockSender{
		response: `{"score": 85, "strengths": ["Go expertise"], "gaps": ["AWS"], "suggestions": ["Add AWS certs"]}`,
	}

	svc := New(st, nil, sender, t.TempDir())
	result, err := svc.ResumeMatch(context.Background(), id)
	if err != nil {
		t.Fatalf("resume match: %v", err)
	}
	if result.Score != 85 {
		t.Errorf("score = %d, want 85", result.Score)
	}
	if len(result.Strengths) != 1 || result.Strengths[0] != "Go expertise" {
		t.Errorf("strengths = %v", result.Strengths)
	}

	got, _ := st.GetLead(id)
	if got.MatchScore != 85 {
		t.Errorf("db match_score = %f, want 85", got.MatchScore)
	}
}

func TestResumeMatch_CodeFence(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &mockSender{
		response: "```json\n{\"score\": 72, \"strengths\": [], \"gaps\": [], \"suggestions\": []}\n```",
	}

	svc := New(st, nil, sender, t.TempDir())
	result, err := svc.ResumeMatch(context.Background(), id)
	if err != nil {
		t.Fatalf("resume match with code fence: %v", err)
	}
	if result.Score != 72 {
		t.Errorf("score = %d, want 72", result.Score)
	}
}

func TestScoreLead(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &mockSender{
		response: `{"score": 90, "strengths": [], "gaps": [], "suggestions": []}`,
	}

	svc := New(st, nil, sender, t.TempDir())
	score, err := svc.ScoreLead(id)
	if err != nil {
		t.Fatalf("score lead: %v", err)
	}
	if score != 90 {
		t.Errorf("score = %f, want 90", score)
	}
}

func TestProposalGen_InvalidPlatform(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &mockSender{response: "proposal text"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ProposalGen(context.Background(), id, "invalid")
	if err == nil {
		t.Fatal("expected error for invalid platform")
	}
}

func TestProposalGen_Valid(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &mockSender{response: "Here's my proposal for the Go Engineer role..."}
	// ProposalGen degrades gracefully when profiledb is nil — generates from JD alone.
	svc := New(st, nil, sender, t.TempDir())
	result, err := svc.ProposalGen(context.Background(), id, "upwork")
	if err != nil {
		t.Fatalf("ProposalGen with nil profiledb: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty proposal when profiledb is nil")
	}
}

func TestCoverLetter_NoProfileDB(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	// CoverLetter degrades gracefully when profiledb is nil — generates from JD alone.
	sender := &mockSender{response: "cover letter from JD only"}
	svc := New(st, nil, sender, t.TempDir())

	result, err := svc.CoverLetter(context.Background(), id)
	if err != nil {
		t.Fatalf("CoverLetter with nil profiledb: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty cover letter when profiledb is nil")
	}
}

func TestColdOutreach(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	lead.CompanyDomain = "stripe.com"
	lead.CompanyIndustry = "Fintech"
	lead.CompanyEmployeeCount = 8000
	id, _ := st.AddLead(lead)

	sender := &mockSender{response: "Hi, I noticed Stripe is hiring..."}
	svc := New(st, nil, sender, t.TempDir())

	text, err := svc.ColdOutreach(context.Background(), id)
	if err != nil {
		t.Fatalf("cold outreach: %v", err)
	}
	if text == "" {
		t.Error("expected non-empty outreach text")
	}
}

func TestSalaryLookup(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &mockSender{
		response: `{"min": 150000, "median": 185000, "max": 220000, "currency": "USD", "reasoning": "Based on market data", "sources": ["levels.fyi"]}`,
	}
	svc := New(st, nil, sender, t.TempDir())

	result, err := svc.SalaryLookup(context.Background(), id)
	if err != nil {
		t.Fatalf("salary lookup: %v", err)
	}
	if result.Median != 185000 {
		t.Errorf("median = %f, want 185000", result.Median)
	}
	if result.Currency != "USD" {
		t.Errorf("currency = %q, want USD", result.Currency)
	}
}
