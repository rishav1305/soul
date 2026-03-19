package ai

import (
	"context"
	"fmt"
	"os"
	"testing"
)

// --- ConsultingFollowUp ---

func TestConsultingFollowUp(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	lead.Company = "Acme Corp"
	lead.CompanyIndustry = "Healthcare"
	lead.Stage = "engaged"
	lead.HiringManager = "Jane Smith"
	lead.Warmth = "warm"
	lead.Description = "AI-powered diagnostic pipeline"
	id, _ := st.AddLead(lead)

	// Add some interactions so follow-up references them.
	st.AddInteraction(id, "call", "phone", "Initial discovery call about AI needs")
	st.AddInteraction(id, "email", "email", "Sent architecture overview document")

	sender := &mockSender{response: "Hi Jane, following up on our architecture discussion last week. The diagnostic pipeline approach we outlined looks promising..."}
	svc := New(st, nil, sender, t.TempDir())

	text, err := svc.ConsultingFollowUp(context.Background(), id)
	if err != nil {
		t.Fatalf("consulting follow-up: %v", err)
	}
	if text == "" {
		t.Error("expected non-empty follow-up text")
	}

	// Verify artifact was stored.
	artifact, err := st.GetLatestArtifact(id, "consulting_followup")
	if err != nil {
		t.Fatalf("get artifact: %v", err)
	}
	if artifact == nil {
		t.Fatal("expected consulting_followup artifact to be stored")
	}
	if artifact.Content != text {
		t.Errorf("artifact content = %q, want %q", artifact.Content, text)
	}
}

func TestConsultingFollowUp_NoInteractions(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	lead.Company = "Solo Corp"
	lead.Stage = "discovered"
	id, _ := st.AddLead(lead)

	sender := &mockSender{response: "Hi, I wanted to reach out about potential consulting opportunities..."}
	svc := New(st, nil, sender, t.TempDir())

	text, err := svc.ConsultingFollowUp(context.Background(), id)
	if err != nil {
		t.Fatalf("consulting follow-up with no interactions: %v", err)
	}
	if text == "" {
		t.Error("expected non-empty follow-up text")
	}
}

func TestConsultingFollowUp_LeadNotFound(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "anything"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ConsultingFollowUp(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for missing lead")
	}
}

func TestConsultingFollowUp_SenderError(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &errSender{err: os.ErrClosed}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ConsultingFollowUp(context.Background(), id)
	if err == nil {
		t.Fatal("expected error from sender")
	}
}

// --- AdvisoryProposalGen ---

func TestAdvisoryProposalGen(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	lead.Company = "FinTech Inc"
	lead.CompanyIndustry = "Financial Services"
	lead.CompanyEmployeeCount = 500
	lead.Description = "AI strategy and model governance advisory"
	lead.HiringManager = "VP Engineering"
	lead.Seniority = "executive"
	id, _ := st.AddLead(lead)

	sender := &mockSender{
		response: `{"executive_summary": "Strategic AI advisory retainer for FinTech Inc", "scope": "Monthly strategic sessions, architecture reviews, vendor evaluation", "deliverables": ["Monthly AI strategy brief", "Quarterly architecture review", "Vendor evaluation reports"], "pricing_model": "$5,000/month retainer with 20 hours included", "terms": "6-month minimum commitment, quarterly review"}`,
	}
	svc := New(st, nil, sender, t.TempDir())

	proposal, err := svc.AdvisoryProposalGen(context.Background(), id)
	if err != nil {
		t.Fatalf("advisory proposal gen: %v", err)
	}
	if proposal.ExecutiveSummary == "" {
		t.Error("executive_summary is empty")
	}
	if proposal.Scope == "" {
		t.Error("scope is empty")
	}
	if len(proposal.Deliverables) != 3 {
		t.Errorf("deliverables count = %d, want 3", len(proposal.Deliverables))
	}
	if proposal.PricingModel == "" {
		t.Error("pricing_model is empty")
	}
	if proposal.Terms == "" {
		t.Error("terms is empty")
	}

	// Verify artifact was stored.
	artifact, err := st.GetLatestArtifact(id, "advisory_proposal")
	if err != nil {
		t.Fatalf("get artifact: %v", err)
	}
	if artifact == nil {
		t.Fatal("expected advisory_proposal artifact to be stored")
	}
}

func TestAdvisoryProposalGen_CodeFence(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &mockSender{
		response: "```json\n{\"executive_summary\": \"test\", \"scope\": \"test scope\", \"deliverables\": [\"d1\"], \"pricing_model\": \"$3k/mo\", \"terms\": \"3 months\"}\n```",
	}
	svc := New(st, nil, sender, t.TempDir())

	proposal, err := svc.AdvisoryProposalGen(context.Background(), id)
	if err != nil {
		t.Fatalf("advisory proposal gen with code fence: %v", err)
	}
	if proposal.ExecutiveSummary != "test" {
		t.Errorf("executive_summary = %q, want test", proposal.ExecutiveSummary)
	}
	if len(proposal.Deliverables) != 1 {
		t.Errorf("deliverables count = %d, want 1", len(proposal.Deliverables))
	}
}

func TestAdvisoryProposalGen_LeadNotFound(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "anything"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.AdvisoryProposalGen(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for missing lead")
	}
}

func TestAdvisoryProposalGen_SenderError(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &errSender{err: fmt.Errorf("api timeout")}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.AdvisoryProposalGen(context.Background(), id)
	if err == nil {
		t.Fatal("expected error from sender")
	}
}

func TestAdvisoryProposalGen_BadJSON(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &mockSender{response: "not valid json at all"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.AdvisoryProposalGen(context.Background(), id)
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}

// --- ProjectProposalGen ---

func TestProjectProposalGen(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	lead.Company = "MedTech Corp"
	lead.CompanyIndustry = "Healthcare"
	lead.CompanyEmployeeCount = 300
	lead.Description = "Build an AI-powered medical image classification system"
	lead.TechnologySlugs = `["python","pytorch","docker"]`
	lead.HiringManager = "CTO"
	id, _ := st.AddLead(lead)

	sender := &mockSender{
		response: `{"problem_statement": "MedTech needs automated medical image classification to reduce radiologist workload", "proposed_solution": "Fine-tuned vision transformer with DICOM integration", "approach": "Transfer learning on pre-trained ViT model with domain-specific augmentation", "milestones": ["Data pipeline setup", "Model training and validation", "API integration", "Clinical trial deployment"], "budget": "$85,000 - $120,000", "timeline": "16 weeks"}`,
	}
	svc := New(st, nil, sender, t.TempDir())

	proposal, err := svc.ProjectProposalGen(context.Background(), id)
	if err != nil {
		t.Fatalf("project proposal gen: %v", err)
	}
	if proposal.ProblemStatement == "" {
		t.Error("problem_statement is empty")
	}
	if proposal.ProposedSolution == "" {
		t.Error("proposed_solution is empty")
	}
	if proposal.Approach == "" {
		t.Error("approach is empty")
	}
	if len(proposal.Milestones) != 4 {
		t.Errorf("milestones count = %d, want 4", len(proposal.Milestones))
	}
	if proposal.Budget == "" {
		t.Error("budget is empty")
	}
	if proposal.Timeline == "" {
		t.Error("timeline is empty")
	}

	// Verify artifact was stored.
	artifact, err := st.GetLatestArtifact(id, "project_proposal")
	if err != nil {
		t.Fatalf("get artifact: %v", err)
	}
	if artifact == nil {
		t.Fatal("expected project_proposal artifact to be stored")
	}
}

func TestProjectProposalGen_CodeFence(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &mockSender{
		response: "```json\n{\"problem_statement\": \"test problem\", \"proposed_solution\": \"test solution\", \"approach\": \"test approach\", \"milestones\": [\"m1\", \"m2\"], \"budget\": \"$50k\", \"timeline\": \"8 weeks\"}\n```",
	}
	svc := New(st, nil, sender, t.TempDir())

	proposal, err := svc.ProjectProposalGen(context.Background(), id)
	if err != nil {
		t.Fatalf("project proposal gen with code fence: %v", err)
	}
	if proposal.ProblemStatement != "test problem" {
		t.Errorf("problem_statement = %q, want test problem", proposal.ProblemStatement)
	}
	if len(proposal.Milestones) != 2 {
		t.Errorf("milestones count = %d, want 2", len(proposal.Milestones))
	}
}

func TestProjectProposalGen_LeadNotFound(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "anything"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ProjectProposalGen(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for missing lead")
	}
}

func TestProjectProposalGen_SenderError(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &errSender{err: fmt.Errorf("connection refused")}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ProjectProposalGen(context.Background(), id)
	if err == nil {
		t.Fatal("expected error from sender")
	}
}

func TestProjectProposalGen_BadJSON(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &mockSender{response: "invalid json response"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ProjectProposalGen(context.Background(), id)
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}

// --- ConsultingUpsellEvaluator ---

func TestConsultingUpsellEvaluator(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	lead.Company = "BigCorp"
	lead.CompanyIndustry = "Enterprise Software"
	lead.CompanyEmployeeCount = 5000
	lead.Stage = "engaged"
	lead.Pipeline = "consulting"
	lead.Description = "AI integration for customer support automation"
	lead.Warmth = "warm"
	lead.HiringManager = "Director of AI"
	id, _ := st.AddLead(lead)

	// Add interactions and artifacts to provide context.
	st.AddInteraction(id, "call", "phone", "Discovery call")
	st.AddInteraction(id, "call", "phone", "Architecture review")
	st.AddInteraction(id, "email", "email", "Sent proposal draft")
	st.AddArtifact(id, "call_prep", `{"company_background": "BigCorp"}`)
	st.AddArtifact(id, "advisory_proposal", `{"executive_summary": "AI advisory"}`)

	sender := &mockSender{
		response: `{"score": 78, "opportunities": ["Expand from support to sales AI", "Data pipeline modernization", "ML ops infrastructure"], "recommended_approach": "Propose a Phase 2 engagement during the quarterly review, focusing on the sales AI use case they mentioned in the last call", "timing": "Next quarterly review in 3 weeks"}`,
	}
	svc := New(st, nil, sender, t.TempDir())

	eval, err := svc.ConsultingUpsellEvaluator(context.Background(), id)
	if err != nil {
		t.Fatalf("consulting upsell evaluator: %v", err)
	}
	if eval.Score != 78 {
		t.Errorf("score = %d, want 78", eval.Score)
	}
	if len(eval.Opportunities) != 3 {
		t.Errorf("opportunities count = %d, want 3", len(eval.Opportunities))
	}
	if eval.RecommendedApproach == "" {
		t.Error("recommended_approach is empty")
	}
	if eval.Timing == "" {
		t.Error("timing is empty")
	}

	// Verify artifact was stored.
	artifact, err := st.GetLatestArtifact(id, "upsell_evaluation")
	if err != nil {
		t.Fatalf("get artifact: %v", err)
	}
	if artifact == nil {
		t.Fatal("expected upsell_evaluation artifact to be stored")
	}
}

func TestConsultingUpsellEvaluator_CodeFence(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &mockSender{
		response: "```json\n{\"score\": 45, \"opportunities\": [\"Expand scope\"], \"recommended_approach\": \"Wait for milestone completion\", \"timing\": \"2 months\"}\n```",
	}
	svc := New(st, nil, sender, t.TempDir())

	eval, err := svc.ConsultingUpsellEvaluator(context.Background(), id)
	if err != nil {
		t.Fatalf("consulting upsell evaluator with code fence: %v", err)
	}
	if eval.Score != 45 {
		t.Errorf("score = %d, want 45", eval.Score)
	}
	if len(eval.Opportunities) != 1 {
		t.Errorf("opportunities count = %d, want 1", len(eval.Opportunities))
	}
}

func TestConsultingUpsellEvaluator_LeadNotFound(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "anything"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ConsultingUpsellEvaluator(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for missing lead")
	}
}

func TestConsultingUpsellEvaluator_SenderError(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &errSender{err: os.ErrClosed}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ConsultingUpsellEvaluator(context.Background(), id)
	if err == nil {
		t.Fatal("expected error from sender")
	}
}

func TestConsultingUpsellEvaluator_BadJSON(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &mockSender{response: "not json"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ConsultingUpsellEvaluator(context.Background(), id)
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}
