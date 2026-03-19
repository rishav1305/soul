package ai

import (
	"context"
	"errors"
	"testing"
)

func TestContractUpsellDetector(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	lead.Pipeline = "contract"
	lead.Stage = "engaged"
	lead.CompanyIndustry = "Fintech"
	id, _ := st.AddLead(lead)

	// Add an existing artifact for context.
	if _, err := st.AddArtifact(id, "sow", "Statement of work for Go microservices"); err != nil {
		t.Fatal(err)
	}

	sender := &mockSender{
		response: `{"upsell_score": 82, "opportunities": [{"type": "scope_expansion", "description": "Add monitoring dashboard", "confidence": 75}, {"type": "new_service", "description": "ML pipeline integration", "confidence": 60}], "next_action": "Schedule expansion call with CTO", "urgency": "medium"}`,
	}

	svc := New(st, nil, sender, t.TempDir())
	result, err := svc.ContractUpsellDetector(context.Background(), id)
	if err != nil {
		t.Fatalf("upsell detector: %v", err)
	}
	if result.UpsellScore != 82 {
		t.Errorf("upsell_score = %d, want 82", result.UpsellScore)
	}
	if len(result.Opportunities) != 2 {
		t.Errorf("opportunities count = %d, want 2", len(result.Opportunities))
	}
	if result.NextAction == "" {
		t.Error("next_action is empty")
	}
	if result.Urgency != "medium" {
		t.Errorf("urgency = %q, want medium", result.Urgency)
	}

	// Verify artifact was stored.
	artifacts, err := st.GetArtifactsByType(id, "upsell_detection")
	if err != nil {
		t.Fatalf("get artifacts: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("artifact count = %d, want 1", len(artifacts))
	}
}

func TestContractUpsellDetector_CodeFence(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	lead.Pipeline = "contract"
	id, _ := st.AddLead(lead)

	sender := &mockSender{
		response: "```json\n{\"upsell_score\": 45, \"opportunities\": [{\"type\": \"referral\", \"description\": \"Intro to sister company\", \"confidence\": 50}], \"next_action\": \"Ask for intro\", \"urgency\": \"low\"}\n```",
	}

	svc := New(st, nil, sender, t.TempDir())
	result, err := svc.ContractUpsellDetector(context.Background(), id)
	if err != nil {
		t.Fatalf("upsell detector with code fence: %v", err)
	}
	if result.UpsellScore != 45 {
		t.Errorf("upsell_score = %d, want 45", result.UpsellScore)
	}
}

func TestContractUpsellDetector_LeadNotFound(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "anything"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ContractUpsellDetector(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for missing lead")
	}
}

func TestContractUpsellDetector_SenderError(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &errSender{err: errors.New("claude unavailable")}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ContractUpsellDetector(context.Background(), id)
	if err == nil {
		t.Fatal("expected error from sender")
	}
}

func TestContractUpsellDetector_BadJSON(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &mockSender{response: "not valid json at all"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ContractUpsellDetector(context.Background(), id)
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}
