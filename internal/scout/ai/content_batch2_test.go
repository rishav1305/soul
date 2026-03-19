package ai

import (
	"context"
	"fmt"
	"testing"
)

// --- ThreadConverter ---

func TestThreadConverter(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{
		response: `{"thread": ["1/ Hook: I spent 6 months building a Go monorepo. Here's what I learned.", "2/ First lesson: specs beat docs every time.", "3/ Second lesson: verification layers catch what tests miss.", "4/ Third lesson: let the AI maintain the boring parts.", "5/ Follow me for more engineering insights. Link in bio."], "summary_tweet": "6 months building a Go monorepo taught me 3 things: specs > docs, verification > tests, AI > manual. Thread below."}`,
	}

	svc := New(st, nil, sender, t.TempDir())
	result, err := svc.ThreadConverter(context.Background(), "I spent 6 months building a Go monorepo and learned three critical lessons about spec-driven development...")
	if err != nil {
		t.Fatalf("thread converter: %v", err)
	}
	if len(result.Thread) != 5 {
		t.Errorf("thread count = %d, want 5", len(result.Thread))
	}
	if result.SummaryTweet == "" {
		t.Error("summary_tweet is empty")
	}
}

func TestThreadConverter_CodeFence(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{
		response: "```json\n{\"thread\": [\"t1\", \"t2\", \"t3\"], \"summary_tweet\": \"summary\"}\n```",
	}

	svc := New(st, nil, sender, t.TempDir())
	result, err := svc.ThreadConverter(context.Background(), "A LinkedIn post about Go patterns")
	if err != nil {
		t.Fatalf("thread converter with code fence: %v", err)
	}
	if len(result.Thread) != 3 {
		t.Errorf("thread count = %d, want 3", len(result.Thread))
	}
	if result.SummaryTweet != "summary" {
		t.Errorf("summary_tweet = %q, want %q", result.SummaryTweet, "summary")
	}
}

func TestThreadConverter_BadJSON(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "not valid json at all"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ThreadConverter(context.Background(), "some post")
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}

func TestThreadConverter_SendError(t *testing.T) {
	st := newTestStore(t)
	sender := &errSender{err: fmt.Errorf("api down")}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ThreadConverter(context.Background(), "some post")
	if err == nil {
		t.Fatal("expected error when sender fails")
	}
}

func TestThreadConverter_EmptyInput(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: `{"thread": [], "summary_tweet": ""}`}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ThreadConverter(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty post")
	}
}

// --- SubstackExpander ---

func TestSubstackExpander(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{
		response: "# Go Concurrency Patterns in Production\n\nWhen I first started building distributed systems in Go, I made every mistake in the book...\n\n## The Problem\n\nMost developers reach for goroutines without understanding the implications.\n\n## Framework 1: The Worker Pool\n\nHere's how to build a proper worker pool...\n\n## Conclusion\n\nConcurrency in Go isn't hard — it's misunderstood.",
	}

	svc := New(st, nil, sender, t.TempDir())
	result, err := svc.SubstackExpander(context.Background(), "I built a Go worker pool that handles 10k req/s. Here are 3 patterns that made it work.", "Go concurrency patterns")
	if err != nil {
		t.Fatalf("substack expander: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty article")
	}
	if len(result) < 50 {
		t.Errorf("article too short: %d chars", len(result))
	}
}

func TestSubstackExpander_SendError(t *testing.T) {
	st := newTestStore(t)
	sender := &errSender{err: fmt.Errorf("timeout")}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.SubstackExpander(context.Background(), "some post", "some topic")
	if err == nil {
		t.Fatal("expected error when sender fails")
	}
}

func TestSubstackExpander_EmptyPost(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "article content"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.SubstackExpander(context.Background(), "", "topic")
	if err == nil {
		t.Fatal("expected error for empty post")
	}
}

func TestSubstackExpander_EmptyTopic(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "article content"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.SubstackExpander(context.Background(), "post content", "")
	if err == nil {
		t.Fatal("expected error for empty topic")
	}
}

// --- ReactiveContentGen ---

func TestReactiveContentGen(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{
		response: `{"linkedin_post": "OpenAI just released GPT-5. Here's what most people are missing about this release...\n\nAs someone who's built production AI systems for 3 years, the real story isn't the benchmark improvements — it's the inference cost reduction.\n\nThis changes the economics of AI deployment fundamentally.", "x_post": "GPT-5 is out. Everyone's talking about benchmarks.\n\nBut the real story? 60% inference cost reduction.\n\nThat changes everything for production AI. Here's why:", "timing_note": "Post within 2 hours of announcement for maximum visibility. LinkedIn audience peaks 8-10am EST."}`,
	}

	svc := New(st, nil, sender, t.TempDir())
	result, err := svc.ReactiveContentGen(context.Background(), "OpenAI released GPT-5 with 60% cost reduction", "production AI economics angle")
	if err != nil {
		t.Fatalf("reactive content gen: %v", err)
	}
	if result.LinkedInPost == "" {
		t.Error("linkedin_post is empty")
	}
	if result.XPost == "" {
		t.Error("x_post is empty")
	}
	if result.TimingNote == "" {
		t.Error("timing_note is empty")
	}
}

func TestReactiveContentGen_CodeFence(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{
		response: "```json\n{\"linkedin_post\": \"post\", \"x_post\": \"tweet\", \"timing_note\": \"post now\"}\n```",
	}

	svc := New(st, nil, sender, t.TempDir())
	result, err := svc.ReactiveContentGen(context.Background(), "some news", "some angle")
	if err != nil {
		t.Fatalf("reactive content gen with code fence: %v", err)
	}
	if result.LinkedInPost != "post" {
		t.Errorf("linkedin_post = %q, want %q", result.LinkedInPost, "post")
	}
	if result.XPost != "tweet" {
		t.Errorf("x_post = %q, want %q", result.XPost, "tweet")
	}
}

func TestReactiveContentGen_BadJSON(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "not json"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ReactiveContentGen(context.Background(), "news", "angle")
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}

func TestReactiveContentGen_SendError(t *testing.T) {
	st := newTestStore(t)
	sender := &errSender{err: fmt.Errorf("connection refused")}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ReactiveContentGen(context.Background(), "news", "angle")
	if err == nil {
		t.Fatal("expected error when sender fails")
	}
}

func TestReactiveContentGen_EmptyNewsContext(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: `{"linkedin_post": "", "x_post": "", "timing_note": ""}`}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ReactiveContentGen(context.Background(), "", "angle")
	if err == nil {
		t.Fatal("expected error for empty news context")
	}
}

func TestReactiveContentGen_EmptyAngle(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: `{"linkedin_post": "", "x_post": "", "timing_note": ""}`}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ReactiveContentGen(context.Background(), "news", "")
	if err == nil {
		t.Fatal("expected error for empty angle")
	}
}

// --- EngagementReply ---

func TestEngagementReply(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{
		response: "Great point about worker pools. In my experience building a similar system at scale, the key insight was using buffered channels with a semaphore pattern rather than unbounded goroutines. We saw a 3x throughput improvement. Have you experimented with the errgroup pattern for coordinating shutdown?",
	}

	svc := New(st, nil, sender, t.TempDir())
	result, err := svc.EngagementReply(context.Background(), "Just deployed our new Go service with worker pools handling 10k req/s", "CTO at a Series B startup, Go enthusiast")
	if err != nil {
		t.Fatalf("engagement reply: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty reply")
	}
}

func TestEngagementReply_SendError(t *testing.T) {
	st := newTestStore(t)
	sender := &errSender{err: fmt.Errorf("rate limited")}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.EngagementReply(context.Background(), "some post", "some author")
	if err == nil {
		t.Fatal("expected error when sender fails")
	}
}

func TestEngagementReply_EmptyPostContent(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "a reply"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.EngagementReply(context.Background(), "", "author context")
	if err == nil {
		t.Fatal("expected error for empty post content")
	}
}

func TestEngagementReply_EmptyAuthorContext(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "a reply"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.EngagementReply(context.Background(), "post content", "")
	if err == nil {
		t.Fatal("expected error for empty author context")
	}
}

func TestEngagementReply_WhitespaceInput(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "a reply"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.EngagementReply(context.Background(), "   ", "author")
	if err == nil {
		t.Fatal("expected error for whitespace-only post content")
	}
}
