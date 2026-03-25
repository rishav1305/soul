package agent

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/rishav1305/soul/internal/scout/store"
)

// LaunchConfig configures a Claude subprocess launch.
type LaunchConfig struct {
	Mode    string // "referral" or "pitch"
	LeadID  int64
	Prompt  string // assembled by the calling ai/ function
	DataDir string // directory for agent run artifacts
}

// LaunchResult holds the outcome of a subprocess launch.
type LaunchResult struct {
	RunID      int64
	Output     string
	TokensUsed int
	Duration   time.Duration
	Error      string
}

// maxConcurrentLaunches limits concurrent subprocess spawns to prevent resource exhaustion.
var launchSem = make(chan struct{}, 3)

// LaunchAsync creates an agent_runs record and spawns the Claude subprocess
// in a background goroutine. Returns the run_id immediately — callers poll
// GET /api/agent/status for completion. At most 3 subprocesses run concurrently.
func LaunchAsync(st *store.Store, cfg LaunchConfig) (int64, error) {
	// Non-blocking check: reject if at capacity
	select {
	case launchSem <- struct{}{}:
		// acquired slot
	default:
		return 0, fmt.Errorf("agent launch queue full (max 3 concurrent) — try again later")
	}

	run := store.AgentRun{
		Platform:  "claude",
		Mode:      cfg.Mode,
		LeadID:    cfg.LeadID,
		Status:    "running",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	runID, err := st.AddAgentRun(run)
	if err != nil {
		<-launchSem // release slot on failure
		return 0, fmt.Errorf("create agent run: %w", err)
	}

	// Run subprocess in background goroutine — releases semaphore on completion
	go func() {
		defer func() { <-launchSem }()
		runSubprocess(st, runID, cfg.Prompt)
	}()

	return runID, nil
}

// runSubprocess executes the Claude CLI and updates agent_runs on completion.
func runSubprocess(st *store.Store, runID int64, prompt string) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// SAFETY: exec.Command directly, never shell
	cmd := exec.CommandContext(ctx, "claude", "--print", "--model", "claude-sonnet-4-6", "--max-turns", "5")
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		st.UpdateAgentRun(runID, "timeout", fmt.Sprintf(`{"error":"timeout after 120s"}`))
		return
	}
	if err != nil {
		errMsg := fmt.Sprintf("exec: %v — stderr: %s", err, stderr.String())
		st.UpdateAgentRun(runID, "failed", fmt.Sprintf(`{"error":%q}`, errMsg))
		return
	}

	st.UpdateAgentRun(runID, "completed", stdout.String())
}
