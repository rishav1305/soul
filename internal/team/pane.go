package team

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// PanePoller captures tmux pane output at regular intervals and computes delta diffs.
type PanePoller struct {
	mu         sync.RWMutex
	panes      map[string]string // agent -> pane_id
	buffers    map[string][]string // agent -> last captured lines
	bufferSize int
	pollMS     int
	session    string
	onDelta    func(PaneDelta) // callback when delta detected

	cancel context.CancelFunc
}

// NewPanePoller creates a PanePoller with the given pane mappings.
func NewPanePoller(panes map[string]string, bufferSize, pollMS int, session string) *PanePoller {
	return &PanePoller{
		panes:      panes,
		buffers:    make(map[string][]string),
		bufferSize: bufferSize,
		pollMS:     pollMS,
		session:    session,
	}
}

// OnDelta registers a callback for pane deltas. Must be called before Start.
func (p *PanePoller) OnDelta(fn func(PaneDelta)) {
	p.onDelta = fn
}

// Start begins polling all panes at the configured interval.
// It blocks until ctx is cancelled.
func (p *PanePoller) Start(ctx context.Context) {
	ctx, p.cancel = context.WithCancel(ctx)
	ticker := time.NewTicker(time.Duration(p.pollMS) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.pollAll(ctx)
		}
	}
}

// Stop cancels the polling loop.
func (p *PanePoller) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}

// GetBuffer returns the current pane buffer for an agent.
func (p *PanePoller) GetBuffer(agent string) ([]string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	buf, ok := p.buffers[agent]
	if !ok {
		return nil, false
	}
	// Return a copy to avoid races.
	cp := make([]string, len(buf))
	copy(cp, buf)
	return cp, true
}

// Agents returns the list of known agent names.
func (p *PanePoller) Agents() []string {
	agents := make([]string, 0, len(p.panes))
	for name := range p.panes {
		agents = append(agents, name)
	}
	return agents
}

func (p *PanePoller) pollAll(ctx context.Context) {
	for agent, paneID := range p.panes {
		if ctx.Err() != nil {
			return
		}
		lines, err := capturePaneOutput(ctx, p.session, paneID, p.bufferSize)
		if err != nil {
			// Pane might not exist (agent not running). Skip silently.
			continue
		}
		p.computeAndStore(agent, lines)
	}
}

func (p *PanePoller) computeAndStore(agent string, newLines []string) {
	p.mu.Lock()
	oldLines := p.buffers[agent]
	p.buffers[agent] = newLines
	p.mu.Unlock()

	if p.onDelta == nil {
		return
	}

	delta := computeDelta(agent, oldLines, newLines)
	if delta.Offset >= 0 {
		p.onDelta(delta)
	}
}

// capturePaneOutput runs tmux capture-pane and returns the output lines.
func capturePaneOutput(ctx context.Context, session, paneID string, maxLines int) ([]string, error) {
	// Use -S to limit scroll history. Negative means lines from bottom.
	// Pane IDs (%N) are globally unique in tmux — no session prefix needed.
	cmd := exec.CommandContext(ctx, "tmux", "capture-pane", "-p", "-S", fmt.Sprintf("-%d", maxLines), "-t", paneID)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("capture pane %s: %w", paneID, err)
	}

	lines := strings.Split(string(out), "\n")
	// Trim trailing empty lines.
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines, nil
}

// computeDelta finds the first line where old and new differ and returns the delta.
// Returns delta with Offset=-1 if no change detected.
func computeDelta(agent string, oldLines, newLines []string) PaneDelta {
	if len(oldLines) == 0 {
		// First capture — send everything.
		if len(newLines) == 0 {
			return PaneDelta{Agent: agent, Offset: -1}
		}
		return PaneDelta{
			Agent:     agent,
			Lines:     newLines,
			DiffLines: newLines,
			Offset:    0,
			Total:     len(newLines),
			Timestamp: time.Now().UnixMilli(),
		}
	}

	// Find first differing line.
	offset := -1
	minLen := len(oldLines)
	if len(newLines) < minLen {
		minLen = len(newLines)
	}

	for i := 0; i < minLen; i++ {
		if oldLines[i] != newLines[i] {
			offset = i
			break
		}
	}

	// Check if new has more lines.
	if offset == -1 && len(newLines) > len(oldLines) {
		offset = len(oldLines)
	}

	// No change.
	if offset == -1 && len(newLines) <= len(oldLines) {
		// Could be fewer lines — detect shrinkage.
		if len(newLines) < len(oldLines) {
			// Content shrank — send full state.
			return PaneDelta{
				Agent:     agent,
				Lines:     newLines,
				DiffLines: newLines,
				Offset:    0,
				Total:     len(newLines),
				Timestamp: time.Now().UnixMilli(),
			}
		}
		return PaneDelta{Agent: agent, Offset: -1}
	}

	diffLines := newLines[offset:]
	return PaneDelta{
		Agent:     agent,
		DiffLines: diffLines,
		Offset:    offset,
		Total:     len(newLines),
		Timestamp: time.Now().UnixMilli(),
	}
}

// CapturePaneOutputFunc is an alias for testing — allows injecting mock capture.
var CapturePaneOutputFunc = capturePaneOutput
