package ai

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rishav1305/soul/internal/scout/store"
)

// --- ContentMetrics ---

func TestContentMetrics(t *testing.T) {
	st := newTestStore(t)

	// Add published posts
	st.AddContentPost(store.ContentPost{
		Platform: "linkedin", Pillar: "builder_insights", Topic: "Go patterns",
		Status: "published", Content: "Post about Go...", Impressions: 500, Likes: 30,
		Comments: 5, Shares: 10,
	})
	st.AddContentPost(store.ContentPost{
		Platform: "linkedin", Pillar: "technical_takes", Topic: "RAG pipelines",
		Status: "published", Content: "Post about RAG...", Impressions: 1200, Likes: 80,
		Comments: 15, Shares: 25,
	})
	st.AddContentPost(store.ContentPost{
		Platform: "x", Pillar: "career_consulting", Topic: "Freelancing tips",
		Status: "published", Content: "Post about freelancing...", Impressions: 300, Likes: 20,
		Comments: 3, Shares: 5,
	})
	// Add a draft — should NOT be included
	st.AddContentPost(store.ContentPost{
		Platform: "linkedin", Pillar: "builder_insights", Topic: "Draft post",
		Status: "draft", Content: "Not yet published...", Impressions: 0, Likes: 0,
		Comments: 0, Shares: 0,
	})

	sender := &mockSender{
		response: `{"analysis": "LinkedIn content performs best with technical deep dives.", "recommendations": ["Post more RAG content", "Increase posting frequency"]}`,
	}

	svc := New(st, nil, sender, t.TempDir())
	result, err := svc.ContentMetrics(context.Background(), "")
	if err != nil {
		t.Fatalf("content metrics: %v", err)
	}
	if result.TotalPosts != 3 {
		t.Errorf("total_posts = %d, want 3", result.TotalPosts)
	}
	if result.TotalImpressions != 2000 {
		t.Errorf("total_impressions = %d, want 2000", result.TotalImpressions)
	}
	// total_engagement = likes(130) + comments(23) + shares(40) = 193
	if result.TotalEngagement != 193 {
		t.Errorf("total_engagement = %d, want 193", result.TotalEngagement)
	}
	if result.AvgEngagement != "9.65%" {
		t.Errorf("avg_engagement_rate = %q, want 9.65%%", result.AvgEngagement)
	}
	if len(result.TopPerforming) != 3 {
		t.Errorf("top_performing count = %d, want 3", len(result.TopPerforming))
	}
	// Top should be RAG pipelines (1200 impressions)
	if len(result.TopPerforming) > 0 && result.TopPerforming[0].Topic != "RAG pipelines" {
		t.Errorf("top_performing[0].topic = %q, want RAG pipelines", result.TopPerforming[0].Topic)
	}
	if result.Analysis == "" {
		t.Error("analysis is empty")
	}
	if len(result.Recommendations) != 2 {
		t.Errorf("recommendations count = %d, want 2", len(result.Recommendations))
	}
}

func TestContentMetrics_PlatformFilter(t *testing.T) {
	st := newTestStore(t)

	st.AddContentPost(store.ContentPost{
		Platform: "linkedin", Pillar: "builder_insights", Topic: "Go patterns",
		Status: "published", Content: "Post about Go...", Impressions: 500, Likes: 30,
		Comments: 5, Shares: 10,
	})
	st.AddContentPost(store.ContentPost{
		Platform: "x", Pillar: "technical_takes", Topic: "Quick tip",
		Status: "published", Content: "Short tweet...", Impressions: 100, Likes: 10,
		Comments: 2, Shares: 3,
	})

	sender := &mockSender{
		response: `{"analysis": "LinkedIn performs well.", "recommendations": ["Keep posting"]}`,
	}

	svc := New(st, nil, sender, t.TempDir())
	result, err := svc.ContentMetrics(context.Background(), "linkedin")
	if err != nil {
		t.Fatalf("content metrics (linkedin filter): %v", err)
	}
	if result.TotalPosts != 1 {
		t.Errorf("total_posts = %d, want 1 (linkedin only)", result.TotalPosts)
	}
	if result.TotalImpressions != 500 {
		t.Errorf("total_impressions = %d, want 500", result.TotalImpressions)
	}
}

func TestContentMetrics_NoPosts(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "should not be called"}
	svc := New(st, nil, sender, t.TempDir())

	result, err := svc.ContentMetrics(context.Background(), "")
	if err != nil {
		t.Fatalf("content metrics (no posts): %v", err)
	}
	if result.TotalPosts != 0 {
		t.Errorf("total_posts = %d, want 0", result.TotalPosts)
	}
	if result.TotalImpressions != 0 {
		t.Errorf("total_impressions = %d, want 0", result.TotalImpressions)
	}
	if result.AvgEngagement != "0.00%" {
		t.Errorf("avg_engagement_rate = %q, want 0.00%%", result.AvgEngagement)
	}
	if result.Analysis == "" {
		t.Error("analysis should be set even with no posts")
	}
}

func TestContentMetrics_SendError(t *testing.T) {
	st := newTestStore(t)
	st.AddContentPost(store.ContentPost{
		Platform: "linkedin", Pillar: "builder_insights", Topic: "Test",
		Status: "published", Content: "...", Impressions: 100, Likes: 5,
		Comments: 1, Shares: 1,
	})

	sender := &errSender{err: fmt.Errorf("api down")}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ContentMetrics(context.Background(), "")
	if err == nil {
		t.Fatal("expected error when sender fails")
	}
}

// --- LinkedInUpdate ---

func TestLinkedInUpdate(t *testing.T) {
	dataDir := t.TempDir()
	st := newTestStore(t)
	sender := &mockSender{
		response: "Senior AI Engineer | Building production LLM systems | Expert in Claude, RAG, agentic AI | 8+ years shipping ML at scale",
	}

	svc := New(st, nil, sender, dataDir)
	result, err := svc.LinkedInUpdate(context.Background(), "headline", "Software Engineer at Acme")
	if err != nil {
		t.Fatalf("linkedin update: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
	if !strings.Contains(result, "AI") {
		t.Errorf("result should mention AI, got: %s", result)
	}

	// Verify file was written
	outPath := filepath.Join(dataDir, "linkedin-updates", "headline.md")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != result {
		t.Errorf("file content mismatch: got %q, want %q", string(data), result)
	}
}

func TestLinkedInUpdate_SendError(t *testing.T) {
	st := newTestStore(t)
	sender := &errSender{err: fmt.Errorf("timeout")}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.LinkedInUpdate(context.Background(), "about", "Current about text")
	if err == nil {
		t.Fatal("expected error when sender fails")
	}
}

// --- GitHubREADMEGen ---

func TestGitHubREADMEGen(t *testing.T) {
	dataDir := t.TempDir()
	st := newTestStore(t)
	sender := &mockSender{
		response: "# soul-v2\n\nAI-powered chat interface with multi-agent orchestration.\n\n## Features\n- Claude OAuth\n- Multi-session support\n- 7-layer verification\n\n## Quick Start\n```bash\nmake build && make serve\n```\n",
	}

	svc := New(st, nil, sender, dataDir)
	result, err := svc.GitHubREADMEGen(context.Background(), "soul-v2", "AI chat interface with Claude integration")
	if err != nil {
		t.Fatalf("github readme gen: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
	if !strings.Contains(result, "soul-v2") {
		t.Errorf("result should mention repo name, got: %s", result)
	}

	// Verify file was written
	outPath := filepath.Join(dataDir, "github-readmes", "soul-v2.md")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != result {
		t.Errorf("file content mismatch: got %q, want %q", string(data), result)
	}
}

func TestGitHubREADMEGen_SendError(t *testing.T) {
	st := newTestStore(t)
	sender := &errSender{err: fmt.Errorf("connection refused")}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.GitHubREADMEGen(context.Background(), "my-repo", "A cool project")
	if err == nil {
		t.Fatal("expected error when sender fails")
	}
}
