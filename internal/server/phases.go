package server

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/rishav1305/soul/internal/ai"
	"github.com/rishav1305/soul/internal/planner"
	"github.com/rishav1305/soul/internal/products"
	"github.com/rishav1305/soul/internal/session"
)

// PhaseConfig holds the model to use for each phase of the pipeline.
type PhaseConfig struct {
	PlanModel   string
	ImplModel   string
	ReviewModel string
	FixModel    string
}

// DefaultPhaseConfig returns the standard model routing:
// Opus for planning, review, and fixes; Sonnet for implementation.
func DefaultPhaseConfig() PhaseConfig {
	return PhaseConfig{
		PlanModel:   "claude-opus-4-6",
		ImplModel:   "claude-sonnet-4-6",
		ReviewModel: "claude-opus-4-6",
		FixModel:    "claude-opus-4-6",
	}
}

// PhaseRunner orchestrates the step-verify-fix pipeline, routing different
// models per phase and running an Opus diff review after implementation.
type PhaseRunner struct {
	aiClient     *ai.Client
	products     *products.Manager
	sessions     *session.Store
	planner      *planner.Store
	broadcast    func(WSMessage)
	config       PhaseConfig
	projectRoot  string
	taskRoot     string
	workflow     string
	sendEvent    func(WSMessage)
	sendActivity func(int64, string, string)

	serverURL     string
	e2eHost       string
	e2eRunnerPath string
}

// NewPhaseRunner creates a PhaseRunner with the given dependencies.
func NewPhaseRunner(
	aiClient *ai.Client,
	pm *products.Manager,
	sessions *session.Store,
	plannerStore *planner.Store,
	broadcast func(WSMessage),
	config PhaseConfig,
	projectRoot, taskRoot, workflow string,
	sendEvent func(WSMessage),
	sendActivity func(int64, string, string),
	serverURL, e2eHost, e2eRunnerPath string,
) *PhaseRunner {
	return &PhaseRunner{
		aiClient:      aiClient,
		products:      pm,
		sessions:      sessions,
		planner:       plannerStore,
		broadcast:     broadcast,
		config:        config,
		projectRoot:   projectRoot,
		taskRoot:      taskRoot,
		workflow:      workflow,
		sendEvent:     sendEvent,
		sendActivity:  sendActivity,
		serverURL:     serverURL,
		e2eHost:       e2eHost,
		e2eRunnerPath: e2eRunnerPath,
	}
}

// RunTask is the main entry point. For micro tasks it runs a simple Sonnet
// agent; for quick/full workflows it runs the full implementation pipeline
// with Opus diff review and optional fix phase.
func (pr *PhaseRunner) RunTask(ctx context.Context, taskID int64, sessionID string, task planner.Task, prompt string) *AgentLoop {
	if pr.workflow == "micro" {
		return pr.runSimple(ctx, taskID, sessionID, prompt, pr.config.ImplModel, 15)
	}
	return pr.runImplementation(ctx, taskID, sessionID, prompt)
}

// runSimple creates and runs a single agent pass with the given model and
// iteration limit. Used for micro tasks and fix phases.
func (pr *PhaseRunner) runSimple(ctx context.Context, taskID int64, sessionID, prompt, model string, maxIter int) *AgentLoop {
	agent := NewAgentLoop(pr.aiClient, pr.products, pr.sessions, pr.planner, pr.broadcast, model, pr.taskRoot)
	agent.autonomous = true
	agent.modelOverride = model
	agent.maxIter = maxIter

	agent.Run(ctx, sessionID, prompt, "code", nil, false, pr.sendEvent)
	return agent
}

// runImplementation runs the full implementation pipeline:
// 1. Sonnet implements the task
// 2. Opus reviews the git diff
// 3. If issues found, Opus fix agent runs with thinking enabled
func (pr *PhaseRunner) runImplementation(ctx context.Context, taskID int64, sessionID, prompt string) *AgentLoop {
	maxIter := 30
	if pr.workflow == "full" {
		maxIter = 40
	}

	// Phase 1: Implementation with Sonnet.
	pr.sendActivity(taskID, "status", fmt.Sprintf("Phase 1: Implementation (%s, max %d iterations)", pr.config.ImplModel, maxIter))
	agent := NewAgentLoop(pr.aiClient, pr.products, pr.sessions, pr.planner, pr.broadcast, pr.config.ImplModel, pr.taskRoot)
	agent.autonomous = true
	agent.modelOverride = pr.config.ImplModel
	agent.maxIter = maxIter

	agent.Run(ctx, sessionID, prompt, "code", nil, false, pr.sendEvent)

	// Check for cancellation before review.
	if ctx.Err() != nil {
		return agent
	}

	// Phase 2: Opus diff review.
	pr.sendActivity(taskID, "status", "Phase 2: Opus diff review...")
	issues := pr.opusDiffReview(ctx, taskID)
	if issues == "" {
		pr.sendActivity(taskID, "status", "Diff review: LGTM — no issues found")
		return agent
	}

	// Phase 3: Opus fix agent for issues found.
	pr.sendActivity(taskID, "status", fmt.Sprintf("Phase 3: Opus fix agent — %d chars of issues", len(issues)))
	log.Printf("[phases] task %d: opus review found issues, running fix agent", taskID)

	fixPrompt := fmt.Sprintf("The implementation was reviewed and the following issues were found. Fix them:\n\n%s", issues)
	fixAgent := NewAgentLoop(pr.aiClient, pr.products, pr.sessions, pr.planner, pr.broadcast, pr.config.FixModel, pr.taskRoot)
	fixAgent.autonomous = true
	fixAgent.modelOverride = pr.config.FixModel
	fixAgent.maxIter = 20

	fixAgent.Run(ctx, sessionID, fixPrompt, "code", nil, true, pr.sendEvent)

	// Merge fix agent stats into the main agent for accurate fingerprinting.
	for f := range fixAgent.filesRead {
		agent.filesRead[f] = true
	}
	agent.iterationsUsed += fixAgent.iterationsUsed
	agent.totalInputTokens += fixAgent.totalInputTokens
	agent.totalOutputTokens += fixAgent.totalOutputTokens

	return agent
}

// opusDiffReview runs a git diff in the task root and sends it to Opus for
// review. Returns the issues text, or "" if LGTM.
func (pr *PhaseRunner) opusDiffReview(ctx context.Context, taskID int64) string {
	// Get the diff — try against dev first, fall back to HEAD~1.
	diff := pr.getGitDiff(ctx)
	if strings.TrimSpace(diff) == "" {
		log.Printf("[phases] task %d: no diff to review", taskID)
		return ""
	}

	// Truncate large diffs to avoid blowing context.
	const maxDiffChars = 15000
	if len(diff) > maxDiffChars {
		diff = diff[:maxDiffChars] + "\n... [truncated]"
	}

	reviewPrompt := fmt.Sprintf(`Review this git diff for correctness issues. Check for:
- Removed declarations (functions, variables, types, imports) that still have references elsewhere
- Logic errors or off-by-one mistakes
- Missing imports or unused imports
- Type mismatches or incorrect function signatures
- Broken references (calling functions that don't exist, wrong argument counts)

If everything looks correct, respond with exactly "LGTM" (nothing else).
If there are issues, list each one concisely with the file and line context.

Diff:
%s`, diff)

	resp, err := pr.aiClient.Complete(ctx, ai.CompleteRequest{
		Model:  pr.config.ReviewModel,
		Prompt: reviewPrompt,
		System: "You are a senior code reviewer. Be precise and only flag real issues, not style preferences.",
		Thinking: &ai.ThinkingConfig{
			Type:         "enabled",
			BudgetTokens: 8000,
		},
		MaxTokens: 4096,
	})
	if err != nil {
		log.Printf("[phases] task %d: opus review failed: %v", taskID, err)
		return ""
	}

	resp = strings.TrimSpace(resp)
	if strings.HasPrefix(resp, "LGTM") {
		return ""
	}
	return resp
}

// getGitDiff runs git diff in the task root directory. Tries diffing against
// dev first, falls back to HEAD~1.
func (pr *PhaseRunner) getGitDiff(ctx context.Context) string {
	// Try diff against dev branch.
	cmd := exec.CommandContext(ctx, "git", "diff", "dev", "--", ".")
	cmd.Dir = pr.taskRoot
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		return string(out)
	}

	// Fallback: diff against previous commit.
	cmd = exec.CommandContext(ctx, "git", "diff", "HEAD~1", "--", ".")
	cmd.Dir = pr.taskRoot
	out, err = cmd.Output()
	if err != nil {
		log.Printf("[phases] git diff failed in %s: %v", pr.taskRoot, err)
		return ""
	}
	return string(out)
}
