package phases

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/rishav1305/soul-v2/internal/chat/stream"
)

// PhaseConfig holds model selections for each phase of task execution.
type PhaseConfig struct {
	PlanModel   string
	ImplModel   string
	ReviewModel string
	FixModel    string
}

// DefaultConfig returns the default phase configuration.
func DefaultConfig() PhaseConfig {
	return PhaseConfig{
		PlanModel:   "claude-opus-4-6",
		ImplModel:   "claude-sonnet-4-6",
		ReviewModel: "claude-opus-4-6",
		FixModel:    "claude-opus-4-6",
	}
}

// MaxIterations returns the maximum agent loop iterations for a given workflow.
func MaxIterations(workflow string) int {
	switch workflow {
	case "micro":
		return 15
	case "full":
		return 40
	default: // "quick" and anything else
		return 30
	}
}

// Sender matches stream.Client.Send for dependency injection.
type Sender interface {
	Send(ctx context.Context, req *stream.Request) (*stream.Response, error)
}

// PhaseResult holds the outcome of a task execution pipeline.
type PhaseResult struct {
	Text              string
	Iterations        int
	TotalInputTokens  int
	TotalOutputTokens int
}

// agentRun holds the result of a single agent loop execution.
type agentRun struct {
	text         string
	iterations   int
	inputTokens  int
	outputTokens int
}

// PhaseRunner executes tasks through a multi-phase pipeline.
type PhaseRunner struct {
	sender   Sender
	config   PhaseConfig
	taskRoot string
}

// NewPhaseRunner creates a PhaseRunner with the given sender, config, and task root directory.
func NewPhaseRunner(sender Sender, config PhaseConfig, taskRoot string) *PhaseRunner {
	return &PhaseRunner{
		sender:   sender,
		config:   config,
		taskRoot: taskRoot,
	}
}

// RunTask executes a task through the appropriate pipeline based on workflow type.
// micro: single runAgent call. quick/full: 3-phase pipeline (impl → review → fix).
func (pr *PhaseRunner) RunTask(ctx context.Context, workflow, prompt, systemPrompt string) (*PhaseResult, error) {
	maxIter := MaxIterations(workflow)

	if workflow == "micro" {
		run, err := pr.runAgent(ctx, systemPrompt, prompt, maxIter)
		if err != nil {
			return nil, fmt.Errorf("micro agent: %w", err)
		}
		return &PhaseResult{
			Text:              run.text,
			Iterations:        run.iterations,
			TotalInputTokens:  run.inputTokens,
			TotalOutputTokens: run.outputTokens,
		}, nil
	}

	// Phase 1: Implementation
	implRun, err := pr.runAgent(ctx, systemPrompt, prompt, maxIter)
	if err != nil {
		return nil, fmt.Errorf("implementation phase: %w", err)
	}

	result := &PhaseResult{
		Text:              implRun.text,
		Iterations:        implRun.iterations,
		TotalInputTokens:  implRun.inputTokens,
		TotalOutputTokens: implRun.outputTokens,
	}

	// Phase 2: Diff review
	diff := pr.getGitDiff(ctx)
	reviewPrompt := fmt.Sprintf(
		"Review this diff for correctness and completeness:\n\n```diff\n%s\n```\n\nIf correct, respond with exactly 'LGTM'. Else list issues.",
		diff,
	)

	reviewReq := &stream.Request{
		Model:     pr.config.ReviewModel,
		MaxTokens: 4096,
		System:    "You are a code reviewer. Be concise.",
		Messages: []stream.Message{
			{
				Role: "user",
				Content: []stream.ContentBlock{
					{Type: "text", Text: reviewPrompt},
				},
			},
		},
	}

	reviewResp, err := pr.sender.Send(ctx, reviewReq)
	if err != nil {
		return nil, fmt.Errorf("review phase: %w", err)
	}

	result.Iterations++
	if reviewResp.Usage != nil {
		result.TotalInputTokens += reviewResp.Usage.InputTokens
		result.TotalOutputTokens += reviewResp.Usage.OutputTokens
	}

	reviewText := extractText(reviewResp)

	// Phase 3: Fix (only if review found issues)
	if strings.HasPrefix(reviewText, "LGTM") {
		result.Text = reviewText
		return result, nil
	}

	fixPrompt := fmt.Sprintf(
		"The reviewer found these issues with your implementation:\n\n%s\n\nPlease fix all issues.",
		reviewText,
	)

	fixRun, err := pr.runAgent(ctx, systemPrompt, fixPrompt, 20)
	if err != nil {
		return nil, fmt.Errorf("fix phase: %w", err)
	}

	result.Text = fixRun.text
	result.Iterations += fixRun.iterations
	result.TotalInputTokens += fixRun.inputTokens
	result.TotalOutputTokens += fixRun.outputTokens

	return result, nil
}

// runAgent executes a simple agent loop: send message, extract text, repeat until end_turn.
func (pr *PhaseRunner) runAgent(ctx context.Context, system, prompt string, maxIter int) (*agentRun, error) {
	run := &agentRun{}

	messages := []stream.Message{
		{
			Role: "user",
			Content: []stream.ContentBlock{
				{Type: "text", Text: prompt},
			},
		},
	}

	for i := 0; i < maxIter; i++ {
		req := &stream.Request{
			Model:     pr.config.ImplModel,
			MaxTokens: 16384,
			System:    system,
			Messages:  messages,
		}

		resp, err := pr.sender.Send(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("iteration %d: %w", i, err)
		}

		run.iterations++
		if resp.Usage != nil {
			run.inputTokens += resp.Usage.InputTokens
			run.outputTokens += resp.Usage.OutputTokens
		}

		text := extractText(resp)
		run.text = text

		if resp.StopReason == "end_turn" {
			return run, nil
		}

		// Append assistant response and a follow-up user message for continuation.
		messages = append(messages,
			stream.Message{
				Role:    "assistant",
				Content: resp.Content,
			},
			stream.Message{
				Role: "user",
				Content: []stream.ContentBlock{
					{Type: "text", Text: "Continue."},
				},
			},
		)
	}

	return run, nil
}

// getGitDiff returns the git diff in taskRoot, truncated to 15000 chars.
func (pr *PhaseRunner) getGitDiff(ctx context.Context) string {
	diff := pr.execGit(ctx, "diff", "HEAD")
	if strings.TrimSpace(diff) == "" {
		diff = pr.execGit(ctx, "diff", "dev")
	}
	if len(diff) > 15000 {
		diff = diff[:15000]
	}
	return diff
}

// execGit runs a git command in taskRoot and returns stdout.
func (pr *PhaseRunner) execGit(ctx context.Context, args ...string) string {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = pr.taskRoot
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// extractText concatenates all text blocks from a response.
func extractText(resp *stream.Response) string {
	var parts []string
	for _, b := range resp.Content {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "")
}
