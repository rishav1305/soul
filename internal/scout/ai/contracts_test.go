package ai

import (
	"context"
	"fmt"
	"testing"
)

// mockSender, errSender, newTestStore, and makeTestLead are defined in
// ai_test.go and content_test.go — reused here without redeclaration.

// --- SOWGenerator ---

func TestSOWGenerator(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	lead.Company = "Acme Corp"
	lead.CompanyIndustry = "Healthcare"
	lead.CompanyEmployeeCount = 200
	lead.Description = "Build AI-powered diagnostic tools using RAG pipeline"
	id, _ := st.AddLead(lead)

	sender := &mockSender{
		response: `{"scope": "Design and implement a RAG-based diagnostic AI system", "deliverables": ["RAG pipeline architecture", "Prototype deployment", "Performance benchmarks"], "timeline": "8 weeks", "pricing": "$45,000 fixed-price", "assumptions": ["Client provides medical dataset", "AWS infrastructure available"]}`,
	}
	svc := New(st, nil, sender, t.TempDir())

	sow, err := svc.SOWGenerator(context.Background(), id)
	if err != nil {
		t.Fatalf("sow generator: %v", err)
	}
	if sow.Scope == "" {
		t.Error("scope is empty")
	}
	if len(sow.Deliverables) != 3 {
		t.Errorf("deliverables count = %d, want 3", len(sow.Deliverables))
	}
	if sow.Timeline != "8 weeks" {
		t.Errorf("timeline = %q, want '8 weeks'", sow.Timeline)
	}
	if sow.Pricing != "$45,000 fixed-price" {
		t.Errorf("pricing = %q, want '$45,000 fixed-price'", sow.Pricing)
	}
	if len(sow.Assumptions) != 2 {
		t.Errorf("assumptions count = %d, want 2", len(sow.Assumptions))
	}

	// Verify artifact was stored.
	artifact, err := st.GetLatestArtifact(id, "sow")
	if err != nil {
		t.Fatalf("get artifact: %v", err)
	}
	if artifact == nil {
		t.Fatal("expected sow artifact to be stored")
	}
}

func TestSOWGenerator_CodeFence(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &mockSender{
		response: "```json\n{\"scope\": \"Build ML pipeline\", \"deliverables\": [\"Model training\"], \"timeline\": \"4 weeks\", \"pricing\": \"$20,000\", \"assumptions\": [\"Data provided\"]}\n```",
	}
	svc := New(st, nil, sender, t.TempDir())

	sow, err := svc.SOWGenerator(context.Background(), id)
	if err != nil {
		t.Fatalf("sow generator with code fence: %v", err)
	}
	if sow.Scope != "Build ML pipeline" {
		t.Errorf("scope = %q, want 'Build ML pipeline'", sow.Scope)
	}
	if len(sow.Deliverables) != 1 {
		t.Errorf("deliverables count = %d, want 1", len(sow.Deliverables))
	}
}

func TestSOWGenerator_LeadNotFound(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "anything"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.SOWGenerator(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for missing lead")
	}
}

func TestSOWGenerator_SenderError(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &errSender{err: fmt.Errorf("api down")}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.SOWGenerator(context.Background(), id)
	if err == nil {
		t.Fatal("expected error when sender fails")
	}
}

func TestSOWGenerator_BadJSON(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &mockSender{response: "not valid json at all"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.SOWGenerator(context.Background(), id)
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}

// --- ContractFollowUp ---

func TestContractFollowUp(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	lead.Company = "TechVentures Inc"
	lead.CompanyIndustry = "SaaS"
	lead.Pipeline = "contract"
	lead.Stage = "proposal"
	lead.Warmth = "warm"
	lead.InteractionCount = 2
	id, _ := st.AddLead(lead)

	// Add some interactions.
	st.AddInteraction(id, "email", "email", "Sent initial proposal document")
	st.AddInteraction(id, "call", "zoom", "Discovery call — discussed RAG requirements")

	sender := &mockSender{
		response: "Hi team at TechVentures, following up on our productive discovery call last week. I've refined the proposal based on our discussion about RAG requirements...",
	}
	svc := New(st, nil, sender, t.TempDir())

	text, err := svc.ContractFollowUp(context.Background(), id)
	if err != nil {
		t.Fatalf("contract follow up: %v", err)
	}
	if text == "" {
		t.Error("expected non-empty follow-up text")
	}

	// Verify artifact was stored.
	artifact, err := st.GetLatestArtifact(id, "contract_followup")
	if err != nil {
		t.Fatalf("get artifact: %v", err)
	}
	if artifact == nil {
		t.Fatal("expected contract_followup artifact to be stored")
	}
}

func TestContractFollowUp_NoInteractions(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	lead.Pipeline = "contract"
	lead.Stage = "discovered"
	lead.Warmth = "new"
	id, _ := st.AddLead(lead)

	sender := &mockSender{
		response: "Hello, I noticed your company is hiring for Go expertise. I specialize in...",
	}
	svc := New(st, nil, sender, t.TempDir())

	text, err := svc.ContractFollowUp(context.Background(), id)
	if err != nil {
		t.Fatalf("contract follow up with no interactions: %v", err)
	}
	if text == "" {
		t.Error("expected non-empty follow-up text")
	}
}

func TestContractFollowUp_LeadNotFound(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "anything"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ContractFollowUp(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for missing lead")
	}
}

func TestContractFollowUp_SenderError(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &errSender{err: fmt.Errorf("timeout")}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ContractFollowUp(context.Background(), id)
	if err == nil {
		t.Fatal("expected error when sender fails")
	}
}

// --- CaseStudyDraft ---

func TestCaseStudyDraft(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	lead.Company = "DataFlow Labs"
	lead.CompanyIndustry = "Data Analytics"
	lead.CompanyEmployeeCount = 50
	lead.Description = "Migrated legacy ETL to real-time streaming with Go"
	lead.Pipeline = "contract"
	lead.Stage = "delivered"
	id, _ := st.AddLead(lead)

	sender := &mockSender{
		response: `{"title": "Real-Time Data Pipeline Migration at DataFlow Labs", "challenge": "DataFlow Labs relied on batch ETL processes that created 6-hour data delays", "approach": "Designed a Go-based streaming architecture using worker pools and channel pipelines", "results": "Reduced data latency from 6 hours to under 30 seconds, processing 10k events/sec", "testimonial_prompt": "How has the new real-time pipeline impacted your team's decision-making speed?"}`,
	}
	svc := New(st, nil, sender, t.TempDir())

	cs, err := svc.CaseStudyDraft(context.Background(), id)
	if err != nil {
		t.Fatalf("case study draft: %v", err)
	}
	if cs.Title == "" {
		t.Error("title is empty")
	}
	if cs.Challenge == "" {
		t.Error("challenge is empty")
	}
	if cs.Approach == "" {
		t.Error("approach is empty")
	}
	if cs.Results == "" {
		t.Error("results is empty")
	}
	if cs.TestimonialPrompt == "" {
		t.Error("testimonial_prompt is empty")
	}

	// Verify artifact was stored.
	artifact, err := st.GetLatestArtifact(id, "case_study")
	if err != nil {
		t.Fatalf("get artifact: %v", err)
	}
	if artifact == nil {
		t.Fatal("expected case_study artifact to be stored")
	}
}

func TestCaseStudyDraft_CodeFence(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &mockSender{
		response: "```json\n{\"title\": \"Test Case Study\", \"challenge\": \"Legacy system\", \"approach\": \"Modern Go stack\", \"results\": \"3x throughput\", \"testimonial_prompt\": \"How did this help?\"}\n```",
	}
	svc := New(st, nil, sender, t.TempDir())

	cs, err := svc.CaseStudyDraft(context.Background(), id)
	if err != nil {
		t.Fatalf("case study draft with code fence: %v", err)
	}
	if cs.Title != "Test Case Study" {
		t.Errorf("title = %q, want 'Test Case Study'", cs.Title)
	}
}

func TestCaseStudyDraft_LeadNotFound(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "anything"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.CaseStudyDraft(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for missing lead")
	}
}

func TestCaseStudyDraft_SenderError(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &errSender{err: fmt.Errorf("connection refused")}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.CaseStudyDraft(context.Background(), id)
	if err == nil {
		t.Fatal("expected error when sender fails")
	}
}

func TestCaseStudyDraft_BadJSON(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &mockSender{response: "invalid json response"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.CaseStudyDraft(context.Background(), id)
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}
