package ai

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExpertApplication(t *testing.T) {
	st := newTestStore(t)
	dataDir := t.TempDir()

	sender := &mockSender{response: "Expert in AI/ML with 8+ years building production systems..."}
	svc := New(st, nil, sender, dataDir)

	text, err := svc.ExpertApplication(context.Background(), "GLG", "AI Strategy")
	if err != nil {
		t.Fatalf("expert application: %v", err)
	}
	if text != "Expert in AI/ML with 8+ years building production systems..." {
		t.Errorf("text = %q, want mock response", text)
	}

	// Verify file was written.
	outPath := filepath.Join(dataDir, "expert-applications", "GLG.md")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != text {
		t.Errorf("file content = %q, want %q", string(data), text)
	}
}

func TestExpertApplication_SenderError(t *testing.T) {
	st := newTestStore(t)

	sender := &errSender{err: os.ErrClosed}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ExpertApplication(context.Background(), "GLG", "AI Strategy")
	if err == nil {
		t.Fatal("expected error from sender")
	}
}

func TestCallPrepBrief(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	lead.Company = "Acme Corp"
	lead.CompanyIndustry = "Healthcare"
	lead.CompanyEmployeeCount = 200
	lead.Description = "Build AI-powered diagnostic tools"
	id, _ := st.AddLead(lead)

	sender := &mockSender{
		response: `{"company_background": "Acme Corp is a healthcare company", "likely_questions": ["How does RAG work?", "What LLM do you recommend?"], "relevant_experience": ["Built production RAG systems", "Healthcare AI consulting"], "key_data_points": ["GPT-4 costs $0.03/1k tokens"]}`,
	}
	svc := New(st, nil, sender, t.TempDir())

	brief, err := svc.CallPrepBrief(context.Background(), id)
	if err != nil {
		t.Fatalf("call prep brief: %v", err)
	}
	if brief.CompanyBackground != "Acme Corp is a healthcare company" {
		t.Errorf("company_background = %q", brief.CompanyBackground)
	}
	if len(brief.LikelyQuestions) != 2 {
		t.Errorf("likely_questions count = %d, want 2", len(brief.LikelyQuestions))
	}
	if len(brief.RelevantExperience) != 2 {
		t.Errorf("relevant_experience count = %d, want 2", len(brief.RelevantExperience))
	}
	if len(brief.KeyDataPoints) != 1 {
		t.Errorf("key_data_points count = %d, want 1", len(brief.KeyDataPoints))
	}

	// Verify artifact was stored.
	artifact, err := st.GetLatestArtifact(id, "call_prep")
	if err != nil {
		t.Fatalf("get artifact: %v", err)
	}
	if artifact == nil {
		t.Fatal("expected call_prep artifact to be stored")
	}
}

func TestCallPrepBrief_CodeFence(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &mockSender{
		response: "```json\n{\"company_background\": \"test\", \"likely_questions\": [], \"relevant_experience\": [], \"key_data_points\": []}\n```",
	}
	svc := New(st, nil, sender, t.TempDir())

	brief, err := svc.CallPrepBrief(context.Background(), id)
	if err != nil {
		t.Fatalf("call prep brief with code fence: %v", err)
	}
	if brief.CompanyBackground != "test" {
		t.Errorf("company_background = %q, want test", brief.CompanyBackground)
	}
}

func TestCallPrepBrief_LeadNotFound(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "anything"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.CallPrepBrief(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for missing lead")
	}
}

func TestCallPrepBrief_SenderError(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &errSender{err: os.ErrClosed}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.CallPrepBrief(context.Background(), id)
	if err == nil {
		t.Fatal("expected error from sender")
	}
}
