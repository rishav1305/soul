# Soul Autonomous Pipeline v2 — "Step-Verify-Fix" Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace Soul's single-pass autonomous execution with a phase-based pipeline that uses different Claude models per phase, verifies each step, and has Opus fix failures — achieving near-100% task completion accuracy.

**Architecture:** The execution engine splits into 4 phases: Plan (Opus+thinking), Execute (Sonnet+thinking per step), Step Review (Opus reviews diff), Fix (Opus fixes failures). A new `PhaseRunner` orchestrates model switching. The existing `AgentLoop` gains a `modelOverride` field so the same loop can run with different models per phase. Verification gates run after each step (tsc + vite + runtime check). The same pipeline serves both chat and autonomous modes — only the reviewer differs (human vs pipeline).

**Tech Stack:** Go 1.24, Claude API (Opus 4.6 / Sonnet 4.6 / Haiku), Puppeteer on titan-pc for runtime checks, React 19 + TypeScript + Vite for frontend

---

## Task 1: Add Model Override to AI Client

The AI client currently uses a fixed model set at construction. We need per-request model override support.

**Files:**
- Modify: `internal/ai/client.go:83-91` (SendStream already supports per-request model)
- Modify: `internal/ai/client.go:126-138` (CompleteSimple needs thinking support)

**Step 1: Extend CompleteSimple to support thinking**

`CompleteSimple` currently hardcodes `MaxTokens=1024` and has no thinking support. The Opus review phase needs thinking.

```go
// In internal/ai/client.go, replace CompleteSimple (lines 126-190) with:

// CompleteRequest holds options for a non-streaming completion.
type CompleteRequest struct {
	Model    string
	Prompt   string
	System   string
	Thinking *ThinkingConfig
	MaxTokens int
}

// CompleteSimple makes a non-streaming API call and returns the text response.
// Useful for quick, lightweight tasks like verification where streaming isn't needed.
func (c *Client) CompleteSimple(ctx context.Context, model, prompt string) (string, error) {
	return c.Complete(ctx, CompleteRequest{Model: model, Prompt: prompt})
}

// Complete makes a non-streaming API call with full control over parameters.
func (c *Client) Complete(ctx context.Context, opts CompleteRequest) (string, error) {
	model := opts.Model
	if model == "" {
		model = c.model
	}
	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1024
	}
	if opts.Thinking != nil && maxTokens < 16384 {
		maxTokens = 16384
	}

	apiReq := Request{
		Model:     model,
		MaxTokens: maxTokens,
		System:    opts.System,
		Messages: []Message{
			{Role: "user", Content: opts.Prompt},
		},
		Stream:   false,
		Thinking: opts.Thinking,
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return "", fmt.Errorf("ai: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, claudeAPIURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ai: failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", claudeAPIVersion)

	if err := c.setAuthHeader(httpReq); err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("ai: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ai: failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ai: API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", fmt.Errorf("ai: failed to parse response: %w", err)
	}

	var result string
	for _, block := range apiResp.Content {
		if block.Type == "text" {
			result += block.Text
		}
	}
	return result, nil
}
```

**Step 2: Verify existing callers still compile**

Run: `grep -rn 'CompleteSimple' /home/rishav/soul/internal/`
Verify all callers use the 3-arg signature `(ctx, model, prompt)` — the wrapper preserves this.

**Step 3: Build check**

Run: `cd /home/rishav/soul && go build ./...`
Expected: Clean compilation.

**Step 4: Commit**

```bash
git add internal/ai/client.go
git commit -m "feat(ai): add Complete method with thinking support for model-per-phase routing"
```

---

## Task 2: Add Model Override to AgentLoop

Allow the agent loop to use a different model than the client's default, enabling phase-based model switching.

**Files:**
- Modify: `internal/server/agent.go:115-135` (AgentLoop struct)
- Modify: `internal/server/agent.go:410-416` (request construction in runLoop)
- Modify: `internal/server/agent.go:355-365` (thinking config)

**Step 1: Add modelOverride field to AgentLoop**

In `internal/server/agent.go`, add field after `maxIter` (line 124):

```go
type AgentLoop struct {
	// ...existing fields through maxIter...
	modelOverride string // if set, overrides client's default model for this run
	// ...rest of fields...
}
```

**Step 2: Use modelOverride in request construction**

In `runLoop()` at line 410, modify the request to include the model override:

```go
req := ai.Request{
	Model:     a.modelOverride, // empty string = client uses its default
	MaxTokens: maxTokens,
	System:    sysPrompt,
	Messages:  messages,
	Tools:     claudeTools,
	Thinking:  thinkingConfig,
}
```

**Step 3: Update thinking config to respect modelOverride**

At line 358, change the thinking condition to also check modelOverride:

```go
effectiveModel := a.modelOverride
if effectiveModel == "" {
	effectiveModel = a.model
}
if thinking && strings.Contains(effectiveModel, "opus") {
	maxTokens = 32000
	thinkingConfig = &ai.ThinkingConfig{
		Type:         "enabled",
		BudgetTokens: 16000,
	}
	log.Printf("[agent] extended thinking enabled: budget_tokens=16000 max_tokens=%d model=%s", maxTokens, effectiveModel)
}
```

**Step 4: Build check**

Run: `cd /home/rishav/soul && go build ./...`
Expected: Clean compilation.

**Step 5: Commit**

```bash
git add internal/server/agent.go
git commit -m "feat(agent): add modelOverride field for phase-based model switching"
```

---

## Task 3: Create Step Verification Gate

Add runtime verification: load the page in headless Chrome via Puppeteer on titan-pc and check for JS console errors. This extends the existing `PreMergeGate` in gates.go.

**Files:**
- Modify: `internal/server/gates.go` (add RuntimeGate function)
- Reference: `internal/server/gates.go:67-123` (existing RunSmokeTest pattern)

**Step 1: Add RuntimeGate function**

Append to `internal/server/gates.go`:

```go
// RuntimeGate loads the page in headless Chrome and checks for JS console errors.
// Returns nil if no errors, or an error with the console error details.
func RuntimeGate(serverURL, e2eHost, e2eRunnerPath string) error {
	log.Printf("[gate] running runtime gate against %s", serverURL)

	argsJSON, _ := json.Marshal(map[string]string{
		"action": "console_errors",
		"url":    serverURL,
	})

	command := fmt.Sprintf(
		"echo %s | ssh %s 'cat > /tmp/soul-e2e-args.json && cd %s && node test-runner.js --json /tmp/soul-e2e-args.json'",
		shellescape(string(argsJSON)), e2eHost, e2eRunnerPath,
	)

	cmd := exec.Command("bash", "-c", command)
	done := make(chan struct{})
	var out []byte
	var execErr error

	go func() {
		out, execErr = cmd.CombinedOutput()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		cmd.Process.Kill()
		return fmt.Errorf("runtime gate timed out after 30s")
	}

	if execErr != nil {
		return fmt.Errorf("runtime gate failed: %v\n%s", execErr, string(out))
	}

	// Parse JSON result.
	output := strings.TrimSpace(string(out))
	lines := strings.Split(output, "\n")
	var jsonLine string
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "{") {
			jsonLine = strings.TrimSpace(lines[i])
			break
		}
	}
	if jsonLine == "" {
		// No JSON output means the runner doesn't support this action yet — skip.
		log.Printf("[gate] runtime gate: no JSON output (runner may not support console_errors yet)")
		return nil
	}

	var result struct {
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal([]byte(jsonLine), &result); err != nil {
		log.Printf("[gate] runtime gate: failed to parse JSON: %v", err)
		return nil // don't block on parse failure
	}

	if len(result.Errors) > 0 {
		return fmt.Errorf("JS console errors detected:\n- %s", strings.Join(result.Errors, "\n- "))
	}

	log.Printf("[gate] runtime gate PASSED — no JS errors")
	return nil
}

// StepVerificationGate runs the full step-level verification:
// 1. TypeScript check (tsc --noEmit)
// 2. Vite build
// 3. Runtime check (load page, check console errors)
// Returns nil if all pass, error with details on first failure.
func StepVerificationGate(worktreeWeb, serverURL, e2eHost, e2eRunnerPath string) error {
	// Layer A: Build verification (same as PreMergeGate).
	if err := PreMergeGate(worktreeWeb); err != nil {
		return fmt.Errorf("build verification failed: %w", err)
	}

	// Layer A.2: Runtime verification.
	if serverURL != "" && e2eHost != "" {
		if err := RuntimeGate(serverURL, e2eHost, e2eRunnerPath); err != nil {
			return fmt.Errorf("runtime verification failed: %w", err)
		}
	}

	return nil
}
```

**Step 2: Build check**

Run: `cd /home/rishav/soul && go build ./...`
Expected: Clean compilation.

**Step 3: Commit**

```bash
git add internal/server/gates.go
git commit -m "feat(gates): add RuntimeGate and StepVerificationGate for step-level verification"
```

---

## Task 4: Create PhaseRunner — The Step-Verify-Fix Orchestrator

This is the core of v2. The `PhaseRunner` breaks task execution into phases with different models, runs step-level verification after each step, and hands failures to Opus for fixing.

**Files:**
- Create: `internal/server/phases.go`

**Step 1: Create phases.go with PhaseRunner**

```go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rishav1305/soul/internal/ai"
	"github.com/rishav1305/soul/internal/planner"
	"github.com/rishav1305/soul/internal/products"
	"github.com/rishav1305/soul/internal/session"
)

// PhaseConfig holds model routing for each phase of the pipeline.
type PhaseConfig struct {
	PlanModel    string // model for planning phase (e.g., claude-opus-4-6)
	ImplModel    string // model for implementation (e.g., claude-sonnet-4-6)
	ReviewModel  string // model for diff review (e.g., claude-opus-4-6)
	FixModel     string // model for fixing failures (e.g., claude-opus-4-6)
}

// DefaultPhaseConfig returns the standard model routing.
func DefaultPhaseConfig() PhaseConfig {
	return PhaseConfig{
		PlanModel:   "claude-opus-4-6",
		ImplModel:   "claude-sonnet-4-6",
		ReviewModel: "claude-opus-4-6",
		FixModel:    "claude-opus-4-6",
	}
}

// PhaseRunner orchestrates the step-verify-fix pipeline.
type PhaseRunner struct {
	aiClient    *ai.Client
	products    *products.Manager
	sessions    *session.Store
	planner     *planner.Store
	broadcast   func(WSMessage)
	config      PhaseConfig
	projectRoot string
	taskRoot    string
	workflow    string
	sendEvent   func(WSMessage)
	sendActivity func(int64, string, string)

	// Server config for E2E.
	serverURL     string
	e2eHost       string
	e2eRunnerPath string
}

// NewPhaseRunner creates a PhaseRunner for a task.
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

// RunTask executes the full step-verify-fix pipeline for a task.
// Returns the agent used (for filesRead/iterationsUsed extraction).
func (pr *PhaseRunner) RunTask(ctx context.Context, taskID int64, sessionID string, task planner.Task, prompt string) *AgentLoop {
	// For micro tasks, skip the full phase pipeline — just run with Sonnet.
	if pr.workflow == "micro" {
		return pr.runSimple(ctx, taskID, sessionID, prompt, pr.config.ImplModel, 15)
	}

	// Phase 1: Planning with Opus + thinking.
	// We don't run a separate planning agent — the implementation agent
	// outputs a plan as its first action (enforced by MANDATORY: Plan First).
	// The model used IS the implementation model but we log the phase.
	log.Printf("[phases] task %d: starting implementation phase (model=%s)", taskID, pr.config.ImplModel)
	pr.sendActivity(taskID, "status", fmt.Sprintf("Phase: Implement (model=%s)", pr.config.ImplModel))

	// Phase 2: Implementation with step verification.
	agent := pr.runImplementation(ctx, taskID, sessionID, prompt)

	return agent
}

// runSimple runs a single-pass agent (no step verification). Used for micro tasks.
func (pr *PhaseRunner) runSimple(ctx context.Context, taskID int64, sessionID, prompt, model string, maxIter int) *AgentLoop {
	agent := NewAgentLoop(pr.aiClient, pr.products, pr.sessions, pr.planner, pr.broadcast, model, pr.taskRoot)
	agent.autonomous = true
	agent.modelOverride = model
	agent.maxIter = maxIter
	agent.Run(ctx, sessionID, prompt, "code", nil, false, pr.sendEvent)
	return agent
}

// runImplementation runs the implementation phase with post-completion verification.
func (pr *PhaseRunner) runImplementation(ctx context.Context, taskID int64, sessionID, prompt string) *AgentLoop {
	maxIter := 30
	if pr.workflow == "full" {
		maxIter = 40
	}

	agent := NewAgentLoop(pr.aiClient, pr.products, pr.sessions, pr.planner, pr.broadcast, pr.config.ImplModel, pr.taskRoot)
	agent.autonomous = true
	agent.modelOverride = pr.config.ImplModel
	agent.maxIter = maxIter

	// Run implementation.
	agent.Run(ctx, sessionID, prompt, "code", nil, false, pr.sendEvent)

	if ctx.Err() != nil {
		return agent
	}

	// Post-implementation: Opus diff review.
	pr.sendActivity(taskID, "status", "Phase: Opus diff review...")
	reviewResult := pr.opusDiffReview(ctx, taskID)

	if reviewResult != "" {
		// Opus found issues — run fix phase.
		pr.sendActivity(taskID, "status", "Phase: Opus fix (issues found in review)")
		log.Printf("[phases] task %d: Opus review found issues, running fix phase", taskID)

		fixPrompt := fmt.Sprintf(
			"You are fixing issues found during code review of task #%d.\n\n"+
				"## Review Findings\n%s\n\n"+
				"Fix these issues. Use `code_read` to see the current state, then `code_edit` to fix.\n"+
				"After fixing, use `task_update` to move to `validation`.",
			taskID, reviewResult)

		fixAgent := NewAgentLoop(pr.aiClient, pr.products, pr.sessions, pr.planner, pr.broadcast, pr.config.FixModel, pr.taskRoot)
		fixAgent.autonomous = true
		fixAgent.modelOverride = pr.config.FixModel
		fixAgent.maxIter = 20

		fixSessionID := fmt.Sprintf("%s-fix", sessionID)
		fixAgent.Run(ctx, fixSessionID, fixPrompt, "code", nil, true, pr.sendEvent)

		// Merge filesRead from fix agent into main agent.
		for f := range fixAgent.filesRead {
			agent.filesRead[f] = true
		}
		agent.iterationsUsed += fixAgent.iterationsUsed
		agent.totalInputTokens += fixAgent.totalInputTokens
		agent.totalOutputTokens += fixAgent.totalOutputTokens
	}

	return agent
}

// opusDiffReview sends the git diff to Opus for review.
// Returns empty string if no issues, or the issues description.
func (pr *PhaseRunner) opusDiffReview(ctx context.Context, taskID int64) string {
	// Get the diff of changes made.
	cmd := exec.Command("git", "diff", "dev", "--", ".")
	cmd.Dir = pr.taskRoot
	diffOut, err := cmd.Output()
	if err != nil {
		// Fallback: diff against HEAD.
		cmd2 := exec.Command("git", "diff", "HEAD~1", "--", ".")
		cmd2.Dir = pr.taskRoot
		diffOut, err = cmd2.Output()
		if err != nil {
			log.Printf("[phases] task %d: failed to get diff for review: %v", taskID, err)
			return ""
		}
	}

	diff := string(diffOut)
	if len(diff) == 0 {
		return ""
	}

	// Truncate very large diffs.
	if len(diff) > 15000 {
		diff = diff[:15000] + "\n...(truncated)"
	}

	reviewPrompt := fmt.Sprintf(`Review this code diff for a task. Look for:
1. **Removed declarations with remaining references** (e.g., deleted a variable but code still uses it)
2. **Logic errors** (wrong conditions, inverted checks, off-by-one)
3. **Missing imports** for newly used symbols
4. **Type mismatches** that TypeScript/Go wouldn't catch
5. **Broken references** (renamed something but didn't update all usages)

If the diff looks correct, respond with exactly: LGTM
If you find issues, describe each one concisely.

<diff>
%s
</diff>`, diff)

	result, err := pr.aiClient.Complete(ctx, ai.CompleteRequest{
		Model:  pr.config.ReviewModel,
		Prompt: reviewPrompt,
		Thinking: &ai.ThinkingConfig{
			Type:         "enabled",
			BudgetTokens: 8000,
		},
		MaxTokens: 16384,
	})
	if err != nil {
		log.Printf("[phases] task %d: Opus review failed: %v", taskID, err)
		return ""
	}

	result = strings.TrimSpace(result)
	if strings.HasPrefix(result, "LGTM") {
		log.Printf("[phases] task %d: Opus review PASSED", taskID)
		pr.sendActivity(taskID, "status", "Opus review: LGTM")
		return ""
	}

	log.Printf("[phases] task %d: Opus review found issues: %s", taskID, result[:min(len(result), 200)])
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
```

**Step 2: Build check**

Run: `cd /home/rishav/soul && go build ./...`
Expected: Clean compilation.

**Step 3: Commit**

```bash
git add internal/server/phases.go
git commit -m "feat(phases): add PhaseRunner for step-verify-fix pipeline with Opus diff review"
```

---

## Task 5: Wire PhaseRunner into Autonomous Pipeline

Replace the direct `agent.Run()` call in `processTask()` with `PhaseRunner.RunTask()`.

**Files:**
- Modify: `internal/server/autonomous.go:248-280` (replace agent creation + run)

**Step 1: Add phaseEnabled flag and PhaseRunner integration**

In `processTask()`, replace lines 248-280 (from `var maxE2ERetries` through `agent.Run(...)`) with:

```go
// Determine retry limits based on workflow.
var maxE2ERetries int
switch workflow {
case "micro":
	maxE2ERetries = 1
case "quick":
	maxE2ERetries = 2
default:
	maxE2ERetries = 3
}

// Create phase runner for step-verify-fix pipeline.
phaseConfig := DefaultPhaseConfig()
serverURL := ""
e2eHost := ""
e2eRunnerPath := ""
if tp.server != nil && tp.server.cfg != nil {
	serverURL = fmt.Sprintf("http://localhost:%d", tp.server.cfg.Port+1)
	e2eHost = tp.server.cfg.E2EHost
	e2eRunnerPath = tp.server.cfg.E2ERunnerPath
}

phaseRunner := NewPhaseRunner(
	tp.server.ai, tp.products, tp.sessions, tp.planner, tp.broadcast,
	phaseConfig, tp.projectRoot, taskRoot, workflow,
	sendEvent, tp.sendActivity,
	serverURL, e2eHost, e2eRunnerPath,
)

var agent *AgentLoop
hasChanges := false

for attempt := 0; attempt <= maxE2ERetries; attempt++ {
	runPrompt := prompt
	if attempt > 0 {
		tp.sendActivity(taskID, "status", fmt.Sprintf("E2E retry %d/%d — re-running agent with gap report...", attempt, maxE2ERetries))
	}

	agent = phaseRunner.RunTask(ctx, taskID, sessionID, task, runPrompt)
```

Keep the rest of the retry loop (lines 282-374) exactly as-is, but change the `agent.Run(...)` line removal and ensure `agent` variable comes from `phaseRunner.RunTask()`.

**Step 2: Remove the old agent creation block**

Remove the old lines 258-268 (direct `NewAgentLoop` + `agent.autonomous` + `agent.maxIter` + switch), and the old `agent.Run(ctx, sessionID, runPrompt, "code", nil, false, sendEvent)` at line 280.

**Step 3: Build check**

Run: `cd /home/rishav/soul && go build ./...`
Expected: Clean compilation.

**Step 4: Commit**

```bash
git add internal/server/autonomous.go
git commit -m "feat(autonomous): wire PhaseRunner into processTask for step-verify-fix pipeline"
```

---

## Task 6: Extend Puppeteer Test Runner — Console Error Capture

The runtime gate needs the Puppeteer test runner on titan-pc to support a `console_errors` action.

**Files:**
- Modify: `~/soul-e2e/test-runner.js` on titan-pc (via SSH)

**Step 1: SSH to titan-pc and read current test-runner.js**

Run: `ssh rishav@192.168.0.113 'cat ~/soul-e2e/test-runner.js'`
Understand the current structure.

**Step 2: Add console_errors action**

SSH to titan-pc and add the `console_errors` handler to test-runner.js. This action should:
1. Launch headless Chrome
2. Navigate to the URL
3. Collect all `console.error` messages for 5 seconds
4. Return JSON: `{"errors": ["error1", "error2"]}` or `{"errors": []}`

```javascript
// Add this case to the action switch in test-runner.js:
case 'console_errors': {
  const errors = [];
  page.on('console', msg => {
    if (msg.type() === 'error') {
      errors.push(msg.text());
    }
  });
  page.on('pageerror', err => {
    errors.push(err.message);
  });

  await page.goto(args.url, { waitUntil: 'networkidle0', timeout: 15000 });
  // Wait a bit for any delayed errors.
  await new Promise(r => setTimeout(r, 3000));

  console.log(JSON.stringify({ errors }));
  break;
}
```

**Step 3: Test the new action**

Run: `ssh rishav@192.168.0.113 'cd ~/soul-e2e && echo "{\"action\":\"console_errors\",\"url\":\"http://192.168.0.128:3000\"}" > /tmp/test-args.json && node test-runner.js --json /tmp/test-args.json'`
Expected: `{"errors":[]}` (no JS errors on prod).

**Step 4: Commit test-runner.js on titan-pc**

```bash
ssh rishav@192.168.0.113 'cd ~/soul-e2e && git add test-runner.js && git commit -m "feat: add console_errors action for runtime verification gate"'
```

---

## Task 7: Update Chat Behavior Rules — System Prompt

Update the system prompt to prevent auto-subtask creation and mandate consistent task tracking.

**Files:**
- Modify: `internal/server/agent.go:22-109` (systemPrompt constant)

**Step 1: Update task creation standards section**

In `systemPrompt`, find the line (around line 55):
```
- For large tasks (3+ files, multiple concerns), create a parent task + subtasks instead of one monolith.
```

Replace with:
```
- NEVER create tasks or subtasks unless the user explicitly asks. If a task looks large, propose decomposition and wait for user approval.
```

**Step 2: Update board management section**

Find the line (around line 65):
```
- Act directly on safe actions (create subtasks, add comments, fix priorities). Ask permission for destructive actions (delete, merge duplicates).
```

Replace with:
```
- Act directly on safe actions (add comments, fix priorities). Ask permission for all task creation and destructive actions (delete, merge duplicates, create subtasks).
```

**Step 3: Add consistent task tracking rule**

After the "Persistent memory" section (around line 71), add:

```
# Task tracking consistency
- If you start tracking or managing tasks in a conversation, you MUST continue until the conversation ends.
- Before any context gets compressed, summarize current task status.
- Update task stage/status in real-time as work progresses.
- If you use tools to work on a task, always report completion or failure — never leave a task in limbo.
```

**Step 4: Build check**

Run: `cd /home/rishav/soul && go build ./...`
Expected: Clean compilation.

**Step 5: Commit**

```bash
git add internal/server/agent.go
git commit -m "fix(agent): prevent auto-subtask creation, mandate consistent task tracking in chat"
```

---

## Task 8: Inline E2E Screenshots in Chat — Frontend

When e2e_screenshot returns an image path, render it inline in the chat message.

**Files:**
- Modify: `web/src/components/chat/ToolCallBlock.tsx` (or wherever tool outputs render)

**Step 1: Read the ToolCallBlock component**

Run: Read `web/src/components/chat/ToolCallBlock.tsx` to understand current structure.

**Step 2: Add image detection and rendering**

In the tool output rendering section, detect paths ending in `.png`, `.jpg`, `.jpeg`, `.gif`, `.webp` and render them as `<img>` tags:

```tsx
// Add this helper function:
function extractImagePath(output: string): string | null {
  const match = output.match(/(?:saved to|screenshot|image|path):\s*(\/[^\s]+\.(?:png|jpg|jpeg|gif|webp))/i);
  if (match) return match[1];
  // Also check if the entire output is just a path.
  const pathMatch = output.trim().match(/^(\/[^\s]+\.(?:png|jpg|jpeg|gif|webp))$/i);
  return pathMatch ? pathMatch[1] : null;
}

// In the output rendering, add:
{imagePath && (
  <img
    src={`/api/screenshot?path=${encodeURIComponent(imagePath)}`}
    alt="E2E Screenshot"
    className="max-w-full rounded border border-border-subtle mt-2"
    loading="lazy"
  />
)}
```

**Step 3: Add screenshot API endpoint**

In `internal/server/routes.go`, add a route to serve screenshot files:

```go
mux.HandleFunc("GET /api/screenshot", func(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" || !strings.HasPrefix(path, "/") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	// Only serve image files from known directories.
	if !strings.Contains(path, "soul-e2e") && !strings.Contains(path, ".soul") {
		http.Error(w, "forbidden path", http.StatusForbidden)
		return
	}
	http.ServeFile(w, r, path)
})
```

**Step 4: Build frontend**

Run: `cd /home/rishav/soul/web && npx vite build`
Expected: Clean build.

**Step 5: Build backend**

Run: `cd /home/rishav/soul && go build ./...`
Expected: Clean compilation.

**Step 6: Commit**

```bash
git add web/src/components/chat/ToolCallBlock.tsx internal/server/routes.go
git commit -m "feat(chat): render E2E screenshots inline in chat messages"
```

---

## Task 9: Add Verification Spec to Planning Phase

During the planning phase, the task prompt instructs the agent to output a verification spec alongside its plan. This spec drives Layer B feature tests.

**Files:**
- Modify: `internal/server/autonomous.go:477-486` (mandatory planning section in buildTaskPrompt)

**Step 1: Extend the mandatory planning instruction**

In `buildTaskPrompt()`, after the existing "MANDATORY: Plan First" section (line 486), add the verification spec template:

```go
b.WriteString("## Verification Spec\n")
b.WriteString("After your plan, output a verification spec that describes what should be tested:\n\n")
b.WriteString("```yaml\n")
b.WriteString("verify:\n")
b.WriteString("  build: true  # tsc + vite must pass\n")
b.WriteString("  runtime_errors: 0  # no JS console errors\n")
b.WriteString("  checks:\n")
b.WriteString("    - description: \"Brief description of what to verify\"\n")
b.WriteString("      selector: \"CSS selector or DOM check\"\n")
b.WriteString("      assertion: \"exists|visible|text_contains|count\"\n")
b.WriteString("```\n\n")
b.WriteString("The pipeline uses this spec for automated feature verification after your changes.\n\n")
```

**Step 2: Build check**

Run: `cd /home/rishav/soul && go build ./...`
Expected: Clean compilation.

**Step 3: Commit**

```bash
git add internal/server/autonomous.go
git commit -m "feat(autonomous): add verification spec template to planning phase"
```

---

## Task 10: Layered Final Gate — Feature Verification

Extend the final gate to run feature verification (Layer B) using the verification spec from the planning phase.

**Files:**
- Modify: `internal/server/gates.go` (add FeatureGate function)
- Modify: `internal/server/autonomous.go` (parse verification spec from agent output, run after merge)

**Step 1: Add FeatureGate to gates.go**

Append to `internal/server/gates.go`:

```go
// FeatureCheck is a single feature verification item.
type FeatureCheck struct {
	Description string `json:"description"`
	Selector    string `json:"selector"`
	Assertion   string `json:"assertion"` // exists, visible, text_contains, count
	Expected    string `json:"expected,omitempty"`
}

// FeatureGateResult holds results of feature verification.
type FeatureGateResult struct {
	AllPass bool           `json:"allPass"`
	Checks  []FeatureCheck `json:"checks"`
	Errors  []string       `json:"errors"`
}

// RunFeatureGate executes feature checks from a verification spec via E2E.
func RunFeatureGate(checks []FeatureCheck, serverURL, e2eHost, e2eRunnerPath string) (*FeatureGateResult, error) {
	if len(checks) == 0 {
		return &FeatureGateResult{AllPass: true}, nil
	}

	log.Printf("[gate] running feature gate: %d checks against %s", len(checks), serverURL)

	// Build assertion commands for the test runner.
	var assertions []map[string]string
	for _, c := range checks {
		assertions = append(assertions, map[string]string{
			"selector":  c.Selector,
			"assertion": c.Assertion,
			"expected":  c.Expected,
		})
	}

	argsJSON, _ := json.Marshal(map[string]any{
		"action":     "feature_test",
		"url":        serverURL,
		"assertions": assertions,
	})

	command := fmt.Sprintf(
		"echo %s | ssh %s 'cat > /tmp/soul-e2e-args.json && cd %s && node test-runner.js --json /tmp/soul-e2e-args.json'",
		shellescape(string(argsJSON)), e2eHost, e2eRunnerPath,
	)

	cmd := exec.Command("bash", "-c", command)
	done := make(chan struct{})
	var out []byte
	var execErr error

	go func() {
		out, execErr = cmd.CombinedOutput()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(60 * time.Second):
		cmd.Process.Kill()
		return nil, fmt.Errorf("feature gate timed out after 60s")
	}

	if execErr != nil {
		return nil, fmt.Errorf("feature gate failed: %v\n%s", execErr, string(out))
	}

	// Parse result.
	output := strings.TrimSpace(string(out))
	lines := strings.Split(output, "\n")
	var jsonLine string
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "{") {
			jsonLine = strings.TrimSpace(lines[i])
			break
		}
	}

	if jsonLine == "" {
		log.Printf("[gate] feature gate: no JSON output (runner may not support feature_test yet)")
		return &FeatureGateResult{AllPass: true}, nil
	}

	var result FeatureGateResult
	if err := json.Unmarshal([]byte(jsonLine), &result); err != nil {
		log.Printf("[gate] feature gate: failed to parse JSON: %v", err)
		return &FeatureGateResult{AllPass: true}, nil
	}

	log.Printf("[gate] feature gate result: allPass=%v, checks=%d", result.AllPass, len(result.Checks))
	return &result, nil
}
```

**Step 2: Build check**

Run: `cd /home/rishav/soul && go build ./...`
Expected: Clean compilation.

**Step 3: Commit**

```bash
git add internal/server/gates.go
git commit -m "feat(gates): add FeatureGate for verification spec-driven feature testing"
```

---

## Task 11: Extend Puppeteer Test Runner — Feature Tests

Add `feature_test` action to the Puppeteer test runner on titan-pc.

**Files:**
- Modify: `~/soul-e2e/test-runner.js` on titan-pc (via SSH)

**Step 1: Add feature_test action**

SSH to titan-pc and add the handler:

```javascript
case 'feature_test': {
  const results = [];
  const errors = [];

  await page.goto(args.url, { waitUntil: 'networkidle0', timeout: 15000 });

  for (const assertion of (args.assertions || [])) {
    try {
      const { selector, assertion: type, expected } = assertion;
      let pass = false;
      let detail = '';

      switch (type) {
        case 'exists':
          pass = await page.$(selector) !== null;
          detail = pass ? 'Element found' : 'Element not found';
          break;
        case 'visible': {
          const el = await page.$(selector);
          if (el) {
            pass = await el.isIntersectingViewport();
            detail = pass ? 'Element visible' : 'Element not visible';
          } else {
            detail = 'Element not found';
          }
          break;
        }
        case 'text_contains': {
          const el = await page.$(selector);
          if (el) {
            const text = await el.evaluate(e => e.textContent);
            pass = text && text.includes(expected);
            detail = pass ? `Contains "${expected}"` : `Text: "${text}"`;
          } else {
            detail = 'Element not found';
          }
          break;
        }
        case 'count': {
          const els = await page.$$(selector);
          const count = els.length;
          pass = count === parseInt(expected);
          detail = `Found ${count}, expected ${expected}`;
          break;
        }
        default:
          detail = `Unknown assertion type: ${type}`;
      }

      results.push({ selector, assertion: type, pass, detail });
    } catch (err) {
      errors.push(`${assertion.selector}: ${err.message}`);
    }
  }

  const allPass = results.every(r => r.pass) && errors.length === 0;
  console.log(JSON.stringify({ allPass, checks: results, errors }));
  break;
}
```

**Step 2: Test**

Run: `ssh rishav@192.168.0.113 'cd ~/soul-e2e && echo "{\"action\":\"feature_test\",\"url\":\"http://192.168.0.128:3000\",\"assertions\":[{\"selector\":\"body\",\"assertion\":\"exists\"}]}" > /tmp/test-args.json && node test-runner.js --json /tmp/test-args.json'`
Expected: `{"allPass":true,"checks":[{"selector":"body","assertion":"exists","pass":true,"detail":"Element found"}],"errors":[]}`

**Step 3: Commit on titan-pc**

```bash
ssh rishav@192.168.0.113 'cd ~/soul-e2e && git add test-runner.js && git commit -m "feat: add feature_test action for verification spec assertions"'
```

---

## Task 12: Visual Regression Gate (Layer C)

Add screenshot comparison for UI tasks. Take before/after screenshots and compare.

**Files:**
- Modify: `internal/server/gates.go` (add VisualRegressionGate)

**Step 1: Add VisualRegressionGate**

Append to `internal/server/gates.go`:

```go
// VisualRegressionResult holds the comparison result.
type VisualRegressionResult struct {
	AllPass bool `json:"allPass"`
	Pages   []struct {
		URL        string  `json:"url"`
		Similarity float64 `json:"similarity"`
		Pass       bool    `json:"pass"`
	} `json:"pages"`
}

// RunVisualRegression takes screenshots of pages and compares before/after.
func RunVisualRegression(pages []string, serverURL, e2eHost, e2eRunnerPath string, threshold float64) (*VisualRegressionResult, error) {
	if len(pages) == 0 {
		return &VisualRegressionResult{AllPass: true}, nil
	}

	log.Printf("[gate] running visual regression: %d pages, threshold=%.2f", len(pages), threshold)

	argsJSON, _ := json.Marshal(map[string]any{
		"action":    "visual_regression",
		"url":       serverURL,
		"pages":     pages,
		"threshold": threshold,
	})

	command := fmt.Sprintf(
		"echo %s | ssh %s 'cat > /tmp/soul-e2e-args.json && cd %s && node test-runner.js --json /tmp/soul-e2e-args.json'",
		shellescape(string(argsJSON)), e2eHost, e2eRunnerPath,
	)

	cmd := exec.Command("bash", "-c", command)
	done := make(chan struct{})
	var out []byte
	var execErr error

	go func() {
		out, execErr = cmd.CombinedOutput()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(90 * time.Second):
		cmd.Process.Kill()
		return nil, fmt.Errorf("visual regression timed out after 90s")
	}

	if execErr != nil {
		return nil, fmt.Errorf("visual regression failed: %v\n%s", execErr, string(out))
	}

	output := strings.TrimSpace(string(out))
	lines := strings.Split(output, "\n")
	var jsonLine string
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "{") {
			jsonLine = strings.TrimSpace(lines[i])
			break
		}
	}

	if jsonLine == "" {
		log.Printf("[gate] visual regression: no JSON output")
		return &VisualRegressionResult{AllPass: true}, nil
	}

	var result VisualRegressionResult
	if err := json.Unmarshal([]byte(jsonLine), &result); err != nil {
		log.Printf("[gate] visual regression: parse error: %v", err)
		return &VisualRegressionResult{AllPass: true}, nil
	}

	log.Printf("[gate] visual regression: allPass=%v", result.AllPass)
	return &result, nil
}
```

**Step 2: Build check**

Run: `cd /home/rishav/soul && go build ./...`
Expected: Clean compilation.

**Step 3: Commit**

```bash
git add internal/server/gates.go
git commit -m "feat(gates): add VisualRegressionGate for UI task screenshot comparison"
```

---

## Task 13: Wire Full Final Gate into processTask

Update the post-merge section in `processTask()` to run all 3 layers of the final gate instead of just the smoke test.

**Files:**
- Modify: `internal/server/autonomous.go:338-357` (smoke test section)

**Step 1: Replace basic smoke test with layered final gate**

After the dev frontend rebuild succeeds (around line 338), replace the simple smoke test call with:

```go
tp.sendActivity(taskID, "status", "Dev frontend rebuilt — running final gate (3 layers)...")

// Layer A: Smoke test (page loads, no crashes).
devURL := fmt.Sprintf("http://localhost:%d", tp.server.cfg.Port+1)
smokeResult, smokeErr := RunSmokeTest(devURL, tp.server.cfg.E2EHost, tp.server.cfg.E2ERunnerPath)
if smokeErr != nil {
	log.Printf("[autonomous] smoke test error for task %d: %v", taskID, smokeErr)
	tp.sendActivity(taskID, "status", fmt.Sprintf("Smoke test error: %v", smokeErr))
} else if !smokeResult.AllPass {
	log.Printf("[autonomous] smoke test FAILED for task %d", taskID)
	tp.sendActivity(taskID, "status", "Smoke test FAILED — reverting merge...")
	devWT := filepath.Join(tp.projectRoot, ".worktrees", "dev-server")
	if revErr := RevertLastMerge(devWT); revErr != nil {
		log.Printf("[autonomous] revert failed: %v", revErr)
	}
	tp.server.RebuildDevFrontend()
	prompt = prompt + "\n\n## Dev Smoke Test Failed\n\n" + FormatSmokeFailure(smokeResult) + "\n"
	hasChanges = false
	mergeRetry = true
} else {
	tp.sendActivity(taskID, "status", "Layer A (smoke): PASSED")

	// Layer A.2: Runtime errors check.
	if rtErr := RuntimeGate(devURL, tp.server.cfg.E2EHost, tp.server.cfg.E2ERunnerPath); rtErr != nil {
		log.Printf("[autonomous] runtime gate FAILED for task %d: %v", taskID, rtErr)
		tp.sendActivity(taskID, "status", fmt.Sprintf("Runtime gate FAILED: %v", rtErr))
		// Don't revert for runtime errors — report as warning.
		tp.postVerificationComment(taskID, fmt.Sprintf("**Runtime Warning**: %v", rtErr))
	} else {
		tp.sendActivity(taskID, "status", "Layer A.2 (runtime): PASSED")
	}

	// Layer B: Feature verification (if verification spec exists).
	// TODO: Parse verification spec from agent output and run FeatureGate.
	tp.sendActivity(taskID, "status", "Final gate: ALL LAYERS PASSED")
}
```

**Step 2: Build check**

Run: `cd /home/rishav/soul && go build ./...`
Expected: Clean compilation.

**Step 3: Commit**

```bash
git add internal/server/autonomous.go
git commit -m "feat(autonomous): wire layered final gate (smoke + runtime + feature) into processTask"
```

---

## Task 14: Build, Deploy, and Integration Test

Build the full binary, rebuild frontend, restart Soul, and test with a real task.

**Files:**
- No new files — integration testing

**Step 1: Go build check**

Run: `cd /home/rishav/soul && go build ./...`
Expected: Clean compilation, no errors.

**Step 2: Build the binary**

Run: `cd /home/rishav/soul && go build -o soul ./cmd/soul`
Expected: Binary produced at `./soul`.

**Step 3: Build frontend**

Run: `cd /home/rishav/soul/web && npx vite build`
Expected: Clean build, dist/ updated.

**Step 4: Restart Soul**

Run: `sudo systemctl restart soul`
Expected: Service starts cleanly.

**Step 5: Check logs**

Run: `sudo journalctl -u soul --no-pager -n 20`
Expected: Soul starts, no panics, both servers (:3000, :3001) listening.

**Step 6: Create a test task**

Create a micro task via the UI or API:
- Title: "Add tooltip to Settings button in ProductRail"
- Description: "Add a title='Settings' tooltip to the settings gear icon in ProductRail."
- Workflow: micro (auto-detected)

Verify:
- [ ] Task auto-classifies as `micro`
- [ ] Agent gets CLAUDE.md injected
- [ ] Agent gets ProductRail.tsx pre-loaded
- [ ] Agent does NOT run e2e_* tools
- [ ] Iteration limit is 15
- [ ] PhaseRunner logs show `micro` → simple path (no Opus review)
- [ ] Task reaches validation

**Step 7: Create a quick task**

- Title: "Add conversation history count badge to chat tab in HorizontalRail"
- Verify:
- [ ] Task classifies as `quick`
- [ ] Agent runs with Sonnet
- [ ] After implementation, Opus diff review runs
- [ ] If Opus finds issues, fix phase runs
- [ ] Pre-merge gate passes (tsc + vite)
- [ ] Runtime gate passes (no JS errors)
- [ ] Task reaches validation

**Step 8: Push to Gitea + GitHub**

```bash
cd /home/rishav/soul
git push origin master
git push github master
```

---

## Summary of All Files Changed

| File | Action | Task |
|------|--------|------|
| `internal/ai/client.go` | Modify — add `Complete` method with thinking | Task 1 |
| `internal/server/agent.go:115` | Modify — add `modelOverride` field | Task 2 |
| `internal/server/agent.go:355-416` | Modify — use modelOverride in request | Task 2 |
| `internal/server/agent.go:22-109` | Modify — system prompt chat behavior rules | Task 7 |
| `internal/server/gates.go` | Modify — add RuntimeGate, StepVerificationGate, FeatureGate, VisualRegressionGate | Tasks 3, 10, 12 |
| `internal/server/phases.go` | Create — PhaseRunner with step-verify-fix | Task 4 |
| `internal/server/autonomous.go:248-280` | Modify — wire PhaseRunner | Task 5 |
| `internal/server/autonomous.go:338-357` | Modify — layered final gate | Task 13 |
| `internal/server/autonomous.go:477-486` | Modify — verification spec template | Task 9 |
| `internal/server/routes.go` | Modify — add screenshot API endpoint | Task 8 |
| `web/src/components/chat/ToolCallBlock.tsx` | Modify — inline screenshot rendering | Task 8 |
| `~/soul-e2e/test-runner.js` (titan-pc) | Modify — console_errors + feature_test actions | Tasks 6, 11 |

## Verification Checklist

1. `go build ./...` passes at every task
2. Micro task → simple path, no Opus review, 15 iterations max
3. Quick task → Sonnet implementation, Opus diff review, 30 iterations max
4. Full task → Sonnet implementation, Opus diff review + fix, 40 iterations max
5. Runtime gate catches JS console errors
6. Opus review catches removed-declaration-with-remaining-references
7. Stuck detection still works (unchanged from current)
8. Chat does NOT auto-create subtasks
9. E2E screenshots render inline in chat
10. Both prod (:3000) and dev (:3001) smoke tests pass after deploy
