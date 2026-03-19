package ai

import (
	"context"
	"fmt"
	"testing"

	"github.com/rishav1305/soul-v2/internal/scout/store"
)

// --- ProfileAudit ---

func TestProfileAudit(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{
		response: `{"score": 78, "strengths": ["Strong Go experience", "Clear headline"], "gaps": ["Missing certifications section"], "recommendations": ["Add AWS certification", "Update summary with AI keywords"], "keyword_suggestions": ["distributed systems", "LLM", "RAG"]}`,
	}

	svc := New(st, nil, sender, t.TempDir())
	result, err := svc.ProfileAudit(context.Background(), "linkedin", "Senior Go Engineer with 8 years experience in distributed systems.")
	if err != nil {
		t.Fatalf("profile audit: %v", err)
	}
	if result.Score != 78 {
		t.Errorf("score = %d, want 78", result.Score)
	}
	if len(result.Strengths) != 2 {
		t.Errorf("strengths count = %d, want 2", len(result.Strengths))
	}
	if len(result.Gaps) != 1 {
		t.Errorf("gaps count = %d, want 1", len(result.Gaps))
	}
	if len(result.Recommendations) != 2 {
		t.Errorf("recommendations count = %d, want 2", len(result.Recommendations))
	}
	if len(result.KeywordSuggestions) != 3 {
		t.Errorf("keyword_suggestions count = %d, want 3", len(result.KeywordSuggestions))
	}
}

func TestProfileAudit_CodeFence(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{
		response: "```json\n{\"score\": 65, \"strengths\": [\"Good headline\"], \"gaps\": [\"No skills section\"], \"recommendations\": [\"Add skills\"], \"keyword_suggestions\": [\"AI\"]}\n```",
	}

	svc := New(st, nil, sender, t.TempDir())
	result, err := svc.ProfileAudit(context.Background(), "github", "Developer profile")
	if err != nil {
		t.Fatalf("profile audit with code fence: %v", err)
	}
	if result.Score != 65 {
		t.Errorf("score = %d, want 65", result.Score)
	}
	if len(result.Strengths) != 1 {
		t.Errorf("strengths count = %d, want 1", len(result.Strengths))
	}
}

func TestProfileAudit_SendError(t *testing.T) {
	st := newTestStore(t)
	sender := &errSender{err: fmt.Errorf("api timeout")}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ProfileAudit(context.Background(), "linkedin", "some profile")
	if err == nil {
		t.Fatal("expected error when sender fails")
	}
}

func TestProfileAudit_BadJSON(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "this is not valid json at all"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ProfileAudit(context.Background(), "linkedin", "some profile")
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}

// --- TestimonialRequest ---

func TestTestimonialRequest(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	lead.Company = "Acme Corp"
	lead.JobTitle = "AI Consulting Engagement"
	lead.Description = "Built a RAG pipeline for document search"
	id, _ := st.AddLead(lead)

	sender := &mockSender{response: "Hi team at Acme Corp, it was a pleasure working on the RAG pipeline project..."}
	svc := New(st, nil, sender, t.TempDir())

	text, err := svc.TestimonialRequest(context.Background(), id)
	if err != nil {
		t.Fatalf("testimonial request: %v", err)
	}
	if text == "" {
		t.Error("expected non-empty testimonial request text")
	}
	if text != "Hi team at Acme Corp, it was a pleasure working on the RAG pipeline project..." {
		t.Errorf("text = %q, want mock response", text)
	}

	// Verify artifact was stored.
	artifact, err := st.GetLatestArtifact(id, "testimonial_request")
	if err != nil {
		t.Fatalf("get artifact: %v", err)
	}
	if artifact == nil {
		t.Fatal("expected testimonial_request artifact to be stored")
	}
}

func TestTestimonialRequest_LeadNotFound(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "anything"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.TestimonialRequest(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for missing lead")
	}
}

func TestTestimonialRequest_SendError(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	sender := &errSender{err: fmt.Errorf("claude unavailable")}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.TestimonialRequest(context.Background(), id)
	if err == nil {
		t.Fatal("expected error when sender fails")
	}
}

// --- PinRecommendation ---

func TestPinRecommendation(t *testing.T) {
	st := newTestStore(t)

	// Add published content posts.
	st.AddContentPost(store.ContentPost{
		Platform: "linkedin", Pillar: "builder_insights", Topic: "AI Agents",
		Status: "published", Content: "Post about agents...", Impressions: 1000,
		Likes: 50, Comments: 20, Shares: 15,
	})
	st.AddContentPost(store.ContentPost{
		Platform: "linkedin", Pillar: "technical_takes", Topic: "Go Concurrency",
		Status: "published", Content: "Post about Go patterns...", Impressions: 800,
		Likes: 40, Comments: 10, Shares: 8,
	})

	sender := &mockSender{
		response: `{"recommendations": [{"post_id": 1, "topic": "AI Agents", "reason": "Highest engagement and relevant to consulting positioning", "engagement_score": 85}], "pin_strategy": "Pin the AI Agents post as it showcases expertise and has strong engagement metrics."}`,
	}

	svc := New(st, nil, sender, t.TempDir())
	result, err := svc.PinRecommendation(context.Background(), "linkedin")
	if err != nil {
		t.Fatalf("pin recommendation: %v", err)
	}
	if len(result.Recommendations) != 1 {
		t.Errorf("recommendations count = %d, want 1", len(result.Recommendations))
	}
	if result.Recommendations[0].PostID != 1 {
		t.Errorf("post_id = %d, want 1", result.Recommendations[0].PostID)
	}
	if result.Recommendations[0].Topic != "AI Agents" {
		t.Errorf("topic = %q, want AI Agents", result.Recommendations[0].Topic)
	}
	if result.Recommendations[0].EngagementScore != 85 {
		t.Errorf("engagement_score = %d, want 85", result.Recommendations[0].EngagementScore)
	}
	if result.PinStrategy == "" {
		t.Error("pin_strategy is empty")
	}
}

func TestPinRecommendation_NoPosts(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "should not be called"}
	svc := New(st, nil, sender, t.TempDir())

	result, err := svc.PinRecommendation(context.Background(), "linkedin")
	if err != nil {
		t.Fatalf("pin recommendation with no posts: %v", err)
	}
	if len(result.Recommendations) != 0 {
		t.Errorf("recommendations count = %d, want 0", len(result.Recommendations))
	}
	if result.PinStrategy == "" {
		t.Error("expected non-empty pin_strategy message for no posts")
	}
}

func TestPinRecommendation_SendError(t *testing.T) {
	st := newTestStore(t)

	// Add a post so we actually call the sender.
	st.AddContentPost(store.ContentPost{
		Platform: "linkedin", Pillar: "builder_insights", Topic: "Test",
		Status: "published", Content: "test content",
	})

	sender := &errSender{err: fmt.Errorf("connection refused")}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.PinRecommendation(context.Background(), "linkedin")
	if err == nil {
		t.Fatal("expected error when sender fails")
	}
}
