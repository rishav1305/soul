package ai

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul-v2/internal/chat/stream"
	"github.com/rishav1305/soul-v2/internal/scout/store"
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
