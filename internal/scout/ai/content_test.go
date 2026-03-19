package ai

import (
	"context"
	"fmt"
	"testing"

	"github.com/rishav1305/soul-v2/internal/chat/stream"
)

// errSender returns an error on Send.
type errSender struct {
	err error
}

func (e *errSender) Send(ctx context.Context, req *stream.Request) (*stream.Response, error) {
	return nil, e.err
}

// --- ContentSeriesGen ---

func TestContentSeriesGen(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{
		response: `{"linkedin_posts": ["Part 1: Hook about Go concurrency patterns...", "Part 2: Deep dive into goroutine pools...", "Part 3: Key takeaways from production usage..."], "x_posts": ["Go concurrency isn't hard, it's misunderstood.", "Thread: I rewrote our pipeline with worker pools. Here's what happened.", "Hot take: most Go devs use channels wrong."], "carousel_outline": "Slide 1: Title — Go Concurrency Done Right\nSlide 2: The Problem\nSlide 3: Pattern 1\nSlide 4: Pattern 2\nSlide 5: Results"}`,
	}

	svc := New(st, nil, sender, t.TempDir())
	result, err := svc.ContentSeriesGen(context.Background(), "Go concurrency patterns", "I built a worker pool that handles 10k req/s")
	if err != nil {
		t.Fatalf("content series gen: %v", err)
	}
	if len(result.LinkedInPosts) != 3 {
		t.Errorf("linkedin_posts count = %d, want 3", len(result.LinkedInPosts))
	}
	if len(result.XPosts) != 3 {
		t.Errorf("x_posts count = %d, want 3", len(result.XPosts))
	}
	if result.CarouselOutline == "" {
		t.Error("carousel_outline is empty")
	}
}

func TestContentSeriesGen_CodeFence(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{
		response: "```json\n{\"linkedin_posts\": [\"p1\", \"p2\", \"p3\"], \"x_posts\": [\"x1\", \"x2\", \"x3\"], \"carousel_outline\": \"outline\"}\n```",
	}

	svc := New(st, nil, sender, t.TempDir())
	result, err := svc.ContentSeriesGen(context.Background(), "topic", "insights")
	if err != nil {
		t.Fatalf("content series gen with code fence: %v", err)
	}
	if len(result.LinkedInPosts) != 3 {
		t.Errorf("linkedin_posts count = %d, want 3", len(result.LinkedInPosts))
	}
}

func TestContentSeriesGen_BadJSON(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "not valid json at all"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ContentSeriesGen(context.Background(), "topic", "insights")
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}

func TestContentSeriesGen_SendError(t *testing.T) {
	st := newTestStore(t)
	sender := &errSender{err: fmt.Errorf("api down")}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ContentSeriesGen(context.Background(), "topic", "insights")
	if err == nil {
		t.Fatal("expected error when sender fails")
	}
}

func TestContentSeriesGen_EmptyResponse(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: `{"linkedin_posts": [], "x_posts": [], "carousel_outline": ""}`}
	svc := New(st, nil, sender, t.TempDir())

	result, err := svc.ContentSeriesGen(context.Background(), "topic", "insights")
	if err != nil {
		t.Fatalf("content series gen: %v", err)
	}
	if len(result.LinkedInPosts) != 0 {
		t.Errorf("linkedin_posts count = %d, want 0", len(result.LinkedInPosts))
	}
}

// --- HookWriter ---

func TestHookWriter(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{
		response: `{"hooks": ["I spent 3 years writing Go the wrong way.", "87% of Go services have this concurrency bug.", "The hardest lesson I learned shipping Go to production.", "What if everything you know about goroutines is wrong?", "I'll admit it: I used to think channels solved everything."]}`,
	}

	svc := New(st, nil, sender, t.TempDir())
	result, err := svc.HookWriter(context.Background(), "Here's my draft post about Go concurrency patterns...")
	if err != nil {
		t.Fatalf("hook writer: %v", err)
	}
	if len(result.Hooks) != 5 {
		t.Errorf("hooks count = %d, want 5", len(result.Hooks))
	}
}

func TestHookWriter_CodeFence(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{
		response: "```json\n{\"hooks\": [\"h1\", \"h2\", \"h3\", \"h4\", \"h5\"]}\n```",
	}

	svc := New(st, nil, sender, t.TempDir())
	result, err := svc.HookWriter(context.Background(), "draft post")
	if err != nil {
		t.Fatalf("hook writer with code fence: %v", err)
	}
	if len(result.Hooks) != 5 {
		t.Errorf("hooks count = %d, want 5", len(result.Hooks))
	}
}

func TestHookWriter_BadJSON(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "invalid json"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.HookWriter(context.Background(), "draft post")
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}

func TestHookWriter_SendError(t *testing.T) {
	st := newTestStore(t)
	sender := &errSender{err: fmt.Errorf("timeout")}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.HookWriter(context.Background(), "draft post")
	if err == nil {
		t.Fatal("expected error when sender fails")
	}
}

func TestHookWriter_EmptyResponse(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: `{"hooks": []}`}
	svc := New(st, nil, sender, t.TempDir())

	result, err := svc.HookWriter(context.Background(), "draft post")
	if err != nil {
		t.Fatalf("hook writer: %v", err)
	}
	if len(result.Hooks) != 0 {
		t.Errorf("hooks count = %d, want 0", len(result.Hooks))
	}
}

// --- ContentTopicGen ---

func TestContentTopicGen(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{
		response: `{"suggestions": [{"topic": "Building a spec-driven Go monorepo", "pillar": "builder_insights", "angle": "How specs replace docs as source of truth"}, {"topic": "Claude as a coding partner", "pillar": "technical_takes", "angle": "When AI pair programming actually works"}, {"topic": "The 7-layer verification stack", "pillar": "personal_process", "angle": "How I ensure quality without a QA team"}]}`,
	}

	svc := New(st, nil, sender, t.TempDir())
	result, err := svc.ContentTopicGen(context.Background(), "spec-driven development, Claude integration, verification pipeline")
	if err != nil {
		t.Fatalf("content topic gen: %v", err)
	}
	if len(result.Suggestions) != 3 {
		t.Errorf("suggestions count = %d, want 3", len(result.Suggestions))
	}

	validPillars := map[string]bool{
		"builder_insights":   true,
		"technical_takes":    true,
		"career_consulting":  true,
		"personal_process":   true,
	}
	for i, s := range result.Suggestions {
		if s.Topic == "" {
			t.Errorf("suggestion[%d].topic is empty", i)
		}
		if !validPillars[s.Pillar] {
			t.Errorf("suggestion[%d].pillar = %q, not a valid pillar", i, s.Pillar)
		}
		if s.Angle == "" {
			t.Errorf("suggestion[%d].angle is empty", i)
		}
	}
}

func TestContentTopicGen_CodeFence(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{
		response: "```json\n{\"suggestions\": [{\"topic\": \"t1\", \"pillar\": \"builder_insights\", \"angle\": \"a1\"}, {\"topic\": \"t2\", \"pillar\": \"technical_takes\", \"angle\": \"a2\"}, {\"topic\": \"t3\", \"pillar\": \"career_consulting\", \"angle\": \"a3\"}]}\n```",
	}

	svc := New(st, nil, sender, t.TempDir())
	result, err := svc.ContentTopicGen(context.Background(), "weekly summary")
	if err != nil {
		t.Fatalf("content topic gen with code fence: %v", err)
	}
	if len(result.Suggestions) != 3 {
		t.Errorf("suggestions count = %d, want 3", len(result.Suggestions))
	}
}

func TestContentTopicGen_BadJSON(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "not json"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ContentTopicGen(context.Background(), "weekly summary")
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}

func TestContentTopicGen_SendError(t *testing.T) {
	st := newTestStore(t)
	sender := &errSender{err: fmt.Errorf("connection refused")}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ContentTopicGen(context.Background(), "weekly summary")
	if err == nil {
		t.Fatal("expected error when sender fails")
	}
}

func TestContentTopicGen_EmptyResponse(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: `{"suggestions": []}`}
	svc := New(st, nil, sender, t.TempDir())

	result, err := svc.ContentTopicGen(context.Background(), "weekly summary")
	if err != nil {
		t.Fatalf("content topic gen: %v", err)
	}
	if len(result.Suggestions) != 0 {
		t.Errorf("suggestions count = %d, want 0", len(result.Suggestions))
	}
}
