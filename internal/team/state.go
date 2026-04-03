package team

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// heartbeatData mirrors the JSON structure of heartbeat files.
type heartbeatData struct {
	Agent       string  `json:"agent"`
	Status      string  `json:"status"`
	CurrentTask string  `json:"current_task"`
	Since       string  `json:"since"`
	Updated     string  `json:"updated"`
	BlockedOn   *string `json:"blocked_on"`
	NextTask    string  `json:"next_task"`
}

// AgentStateAggregator reads heartbeats, inbox counts, task counts, and
// guardian data to produce per-agent state snapshots.
type AgentStateAggregator struct {
	mu           sync.RWMutex
	states       map[string]*AgentState
	cfg          Config
	machineMap   map[string]string
	panes        map[string]string
}

// NewAgentStateAggregator creates an aggregator with the given config.
func NewAgentStateAggregator(cfg Config, panes map[string]string) *AgentStateAggregator {
	return &AgentStateAggregator{
		states:     make(map[string]*AgentState),
		cfg:        cfg,
		machineMap: AgentMachineMap(),
		panes:      panes,
	}
}

// Refresh re-reads all data sources and rebuilds agent states.
func (a *AgentStateAggregator) Refresh() error {
	heartbeats, err := a.readHeartbeats()
	if err != nil {
		return fmt.Errorf("read heartbeats: %w", err)
	}

	inboxCounts := a.countInboxMessages()
	taskCounts := a.countTasksByAgent()

	// Check which tmux panes are alive as a fallback for stale heartbeats.
	alivePanes := ListTmuxPanes(a.cfg.TmuxSession)

	a.mu.Lock()
	defer a.mu.Unlock()

	// Build state for every known agent (from panes map), excluding team-lead.
	for agent, paneID := range a.panes {
		// Filter out the CEO/team-lead -- not an agent.
		if agent == "team-lead" {
			continue
		}

		state := &AgentState{
			Name:       agent,
			Status:     "offline",
			Machine:    a.machineMap[agent],
			PaneID:     paneID,
			InboxCount: inboxCounts[agent],
			TaskCount:  taskCounts[agent],
		}

		if hb, ok := heartbeats[agent]; ok {
			state.Status = hb.Status
			state.CurrentTask = hb.CurrentTask
			state.NextTask = hb.NextTask
			if hb.BlockedOn != nil {
				state.BlockedOn = *hb.BlockedOn
			}
			state.Since = parseTimeOr(hb.Since, time.Time{})
			state.Updated = parseTimeOr(hb.Updated, time.Time{})

			// If heartbeat is stale (>5 min), check tmux pane as fallback.
			if !state.Updated.IsZero() && time.Since(state.Updated) > 5*time.Minute {
				if alivePanes[paneID] {
					// Pane is alive but heartbeat stale -- agent is running but
					// not writing heartbeats. Show as idle rather than offline.
					state.Status = "idle"
				} else {
					state.Status = "offline"
				}
			}
		} else if alivePanes[paneID] {
			// No heartbeat at all but tmux pane exists -- agent is alive.
			state.Status = "idle"
		}

		a.states[agent] = state
	}

	return nil
}

// GetAll returns all current agent states.
func (a *AgentStateAggregator) GetAll() []AgentState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]AgentState, 0, len(a.states))
	for _, s := range a.states {
		result = append(result, *s)
	}
	return result
}

// Get returns a single agent's state.
func (a *AgentStateAggregator) Get(name string) (AgentState, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	s, ok := a.states[name]
	if !ok {
		return AgentState{}, false
	}
	return *s, true
}

func (a *AgentStateAggregator) readHeartbeats() (map[string]heartbeatData, error) {
	result := make(map[string]heartbeatData)
	dir := a.cfg.HeartbeatDir

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		var hb heartbeatData
		if err := json.Unmarshal(data, &hb); err != nil {
			continue
		}

		// Use filename (minus .json) as key if agent field is empty.
		name := hb.Agent
		if name == "" {
			name = strings.TrimSuffix(entry.Name(), ".json")
		}
		result[name] = hb
	}

	return result, nil
}

func (a *AgentStateAggregator) countInboxMessages() map[string]int {
	counts := make(map[string]int)
	inboxBase := filepath.Join(a.cfg.ClawteamDir, "inboxes")

	entries, err := os.ReadDir(inboxBase)
	if err != nil {
		return counts
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Inbox dirs are named like "shuri_shuri" or "team-lead_team-lead"
		parts := strings.SplitN(entry.Name(), "_", 2)
		if len(parts) == 0 {
			continue
		}
		agent := parts[0]

		// Count files in new/ subdirectory.
		newDir := filepath.Join(inboxBase, entry.Name(), "new")
		newEntries, err := os.ReadDir(newDir)
		if err != nil {
			// Also count top-level JSON files (some inboxes have flat structure).
			topEntries, err2 := os.ReadDir(filepath.Join(inboxBase, entry.Name()))
			if err2 != nil {
				continue
			}
			for _, te := range topEntries {
				if !te.IsDir() && strings.HasSuffix(te.Name(), ".json") {
					counts[agent]++
				}
			}
			continue
		}
		for _, ne := range newEntries {
			if !ne.IsDir() && strings.HasSuffix(ne.Name(), ".json") {
				counts[agent]++
			}
		}
	}

	return counts
}

func (a *AgentStateAggregator) countTasksByAgent() map[string]int {
	counts := make(map[string]int)

	// Walk backlog directories matching agent names.
	backlogDir := filepath.Join(os.Getenv("HOME"), ".claude", "pa-backlog")
	if _, err := os.Stat(backlogDir); os.IsNotExist(err) {
		return counts
	}

	for agent := range a.panes {
		agentDir := filepath.Join(backlogDir, agent)
		if _, err := os.Stat(agentDir); os.IsNotExist(err) {
			continue
		}

		_ = filepath.WalkDir(agentDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
				counts[agent]++
			}
			return nil
		})
	}

	return counts
}

func parseTimeOr(s string, fallback time.Time) time.Time {
	if s == "" {
		return fallback
	}
	// Try RFC3339 first, then with timezone offset.
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05+05:30",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return fallback
}
