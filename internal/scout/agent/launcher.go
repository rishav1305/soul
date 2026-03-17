package agent

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/rishav1305/soul-v2/internal/scout/store"
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

// Launch spawns a Claude CLI subprocess and tracks the run in agent_runs.
// SAFETY: Uses exec.Command directly — never shell invocation.
func Launch(ctx context.Context, st *store.Store, cfg LaunchConfig) (*LaunchResult, error) {
	// Create agent_runs record
	run := store.AgentRun{
		Platform:  "claude",
		Mode:      cfg.Mode,
		LeadID:    cfg.LeadID,
		Status:    "running",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	runID, err := st.AddAgentRun(run)
	if err != nil {
		return nil, fmt.Errorf("create agent run: %w", err)
	}

	// 120s timeout
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	start := time.Now()

	// SAFETY: exec.Command directly, never shell
	cmd := exec.CommandContext(ctx, "claude", "--print", "--model", "claude-sonnet-4-6", "--max-turns", "5")
	cmd.Stdin = strings.NewReader(cfg.Prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	duration := time.Since(start)

	result := &LaunchResult{
		RunID:    runID,
		Duration: duration,
	}

	if ctx.Err() == context.DeadlineExceeded {
		result.Error = "timeout after 120s"
		st.UpdateAgentRun(runID, "timeout", fmt.Sprintf(`{"error":%q}`, result.Error))
		return result, nil
	}
	if err != nil {
		result.Error = fmt.Sprintf("exec: %v — stderr: %s", err, stderr.String())
		st.UpdateAgentRun(runID, "failed", fmt.Sprintf(`{"error":%q}`, result.Error))
		return result, nil
	}

	result.Output = stdout.String()
	st.UpdateAgentRun(runID, "completed", result.Output)

	return result, nil
}
