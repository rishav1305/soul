package agent

import (
	"github.com/rishav1305/soul-v2/internal/scout/store"
)

// OptimizationRun represents a single optimizer agent execution.
type OptimizationRun struct {
	ID        int64  `json:"id"`
	Platform  string `json:"platform"`
	Mode      string `json:"mode"`
	Status    string `json:"status"`
	Result    string `json:"result"`
	CreatedAt string `json:"createdAt"`
}

// LaunchOptimizer spawns an optimizer agent for the given platform.
// In production this would start a Claude subprocess with Playwright MCP.
// For now it creates an agent_run record with status "pending".
func LaunchOptimizer(platform string, st *store.Store) (*OptimizationRun, error) {
	run := store.AgentRun{
		Platform: platform,
		Mode:     "optimize",
		Status:   "pending",
		Result:   "agent launch deferred — Claude subprocess spawning not yet implemented",
	}

	id, err := st.AddAgentRun(run)
	if err != nil {
		return nil, err
	}

	saved, err := st.GetAgentRun(id)
	if err != nil {
		return nil, err
	}

	return &OptimizationRun{
		ID:        saved.ID,
		Platform:  saved.Platform,
		Mode:      saved.Mode,
		Status:    saved.Status,
		Result:    saved.Result,
		CreatedAt: saved.CreatedAt,
	}, nil
}
