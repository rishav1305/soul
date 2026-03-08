package server

import (
	"context"
	"sync"
)

// AgentEntry tracks a single running agent goroutine.
type AgentEntry struct {
	Cancel    context.CancelFunc
	Done      <-chan struct{}
	SessionID int64
}

// AgentRegistry is a thread-safe map of session ID → running agent.
// Agents are keyed by their DB session ID so they can survive across
// WebSocket reconnections.
type AgentRegistry struct {
	mu     sync.RWMutex
	agents map[int64]*AgentEntry
}

// NewAgentRegistry returns an initialised registry.
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		agents: make(map[int64]*AgentEntry),
	}
}

// Register adds an agent entry for the given session.
// If an entry already exists it is silently replaced — callers must
// cancel+wait the old entry before calling Register again.
func (r *AgentRegistry) Register(sessionID int64, cancel context.CancelFunc, done <-chan struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[sessionID] = &AgentEntry{
		Cancel:    cancel,
		Done:      done,
		SessionID: sessionID,
	}
}

// Cancel cancels the agent for the given session, if one exists.
// It does NOT wait for the goroutine to finish — call WaitDone for that.
func (r *AgentRegistry) Cancel(sessionID int64) {
	r.mu.RLock()
	entry := r.agents[sessionID]
	r.mu.RUnlock()
	if entry != nil {
		entry.Cancel()
	}
}

// WaitDone returns the done channel for the given session, or nil if none.
// The caller should read from the channel to wait for termination.
func (r *AgentRegistry) WaitDone(sessionID int64) <-chan struct{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if entry, ok := r.agents[sessionID]; ok {
		return entry.Done
	}
	return nil
}

// IsRunning reports whether an agent is currently registered (and not yet
// unregistered) for the given session.
func (r *AgentRegistry) IsRunning(sessionID int64) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.agents[sessionID]
	return ok
}

// Unregister removes the agent entry for the given session.
// Called by the agent goroutine when it finishes.
func (r *AgentRegistry) Unregister(sessionID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.agents, sessionID)
}

// RunningIDs returns the session IDs of all currently registered agents.
func (r *AgentRegistry) RunningIDs() []int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]int64, 0, len(r.agents))
	for id := range r.agents {
		ids = append(ids, id)
	}
	return ids
}
