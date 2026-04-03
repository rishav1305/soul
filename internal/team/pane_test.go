package team

import (
	"testing"
)

func TestComputeDelta_FirstCapture(t *testing.T) {
	delta := computeDelta("shuri", nil, []string{"line1", "line2", "line3"})

	if delta.Agent != "shuri" {
		t.Errorf("expected agent shuri, got %s", delta.Agent)
	}
	if delta.Offset != 0 {
		t.Errorf("expected offset 0, got %d", delta.Offset)
	}
	if len(delta.DiffLines) != 3 {
		t.Errorf("expected 3 diff lines, got %d", len(delta.DiffLines))
	}
	if len(delta.Lines) != 3 {
		t.Errorf("expected 3 full lines, got %d", len(delta.Lines))
	}
	if delta.Total != 3 {
		t.Errorf("expected total 3, got %d", delta.Total)
	}
	if delta.Timestamp == 0 {
		t.Error("expected non-zero timestamp")
	}
}

func TestComputeDelta_NoChange(t *testing.T) {
	old := []string{"line1", "line2"}
	new := []string{"line1", "line2"}

	delta := computeDelta("test", old, new)

	if delta.Offset != -1 {
		t.Errorf("expected offset -1 (no change), got %d", delta.Offset)
	}
}

func TestComputeDelta_AppendedLines(t *testing.T) {
	old := []string{"line1", "line2"}
	new := []string{"line1", "line2", "line3", "line4"}

	delta := computeDelta("test", old, new)

	if delta.Offset != 2 {
		t.Errorf("expected offset 2, got %d", delta.Offset)
	}
	if len(delta.DiffLines) != 2 {
		t.Errorf("expected 2 diff lines, got %d", len(delta.DiffLines))
	}
	if delta.DiffLines[0] != "line3" {
		t.Errorf("expected diff[0]=line3, got %s", delta.DiffLines[0])
	}
}

func TestComputeDelta_ModifiedMiddle(t *testing.T) {
	old := []string{"line1", "line2", "line3"}
	new := []string{"line1", "CHANGED", "line3"}

	delta := computeDelta("test", old, new)

	if delta.Offset != 1 {
		t.Errorf("expected offset 1, got %d", delta.Offset)
	}
	if len(delta.DiffLines) != 2 {
		t.Errorf("expected 2 diff lines (from change onward), got %d", len(delta.DiffLines))
	}
	if delta.DiffLines[0] != "CHANGED" {
		t.Errorf("expected CHANGED, got %s", delta.DiffLines[0])
	}
}

func TestComputeDelta_ShrunkContent(t *testing.T) {
	old := []string{"line1", "line2", "line3"}
	new := []string{"line1"}

	delta := computeDelta("test", old, new)

	// Shrunk content sends full state.
	if delta.Offset != 0 {
		t.Errorf("expected offset 0 for shrunk content, got %d", delta.Offset)
	}
	if delta.Total != 1 {
		t.Errorf("expected total 1, got %d", delta.Total)
	}
}

func TestComputeDelta_EmptyToEmpty(t *testing.T) {
	delta := computeDelta("test", nil, nil)
	if delta.Offset != -1 {
		t.Errorf("expected offset -1 for empty->empty, got %d", delta.Offset)
	}
}

func TestComputeDelta_EmptyOldEmptyNew(t *testing.T) {
	delta := computeDelta("test", []string{}, []string{})
	if delta.Offset != -1 {
		t.Errorf("expected offset -1 for []->[]], got %d", delta.Offset)
	}
}

func TestNewPanePoller(t *testing.T) {
	panes := map[string]string{"shuri": "%5", "happy": "%8"}
	p := NewPanePoller(panes, 500, 1000, "soul-team")

	if p.bufferSize != 500 {
		t.Errorf("expected bufferSize 500, got %d", p.bufferSize)
	}
	if p.pollMS != 1000 {
		t.Errorf("expected pollMS 1000, got %d", p.pollMS)
	}

	agents := p.Agents()
	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(agents))
	}
}

func TestPanePoller_GetBuffer_Empty(t *testing.T) {
	panes := map[string]string{"test": "%0"}
	p := NewPanePoller(panes, 100, 1000, "test")

	_, ok := p.GetBuffer("test")
	if ok {
		t.Error("expected no buffer for fresh poller")
	}
}

func TestPanePoller_ComputeAndStore(t *testing.T) {
	panes := map[string]string{"test": "%0"}
	p := NewPanePoller(panes, 100, 1000, "test")

	var received PaneDelta
	p.OnDelta(func(d PaneDelta) {
		received = d
	})

	// First capture.
	p.computeAndStore("test", []string{"hello", "world"})

	if received.Agent != "test" {
		t.Errorf("expected agent test, got %s", received.Agent)
	}
	if received.Offset != 0 {
		t.Errorf("expected offset 0, got %d", received.Offset)
	}

	// Second capture with change.
	p.computeAndStore("test", []string{"hello", "CHANGED"})
	if received.Offset != 1 {
		t.Errorf("expected offset 1, got %d", received.Offset)
	}
	if received.DiffLines[0] != "CHANGED" {
		t.Errorf("expected CHANGED, got %s", received.DiffLines[0])
	}

	// Verify buffer updated.
	buf, ok := p.GetBuffer("test")
	if !ok {
		t.Fatal("expected buffer for test")
	}
	if len(buf) != 2 {
		t.Errorf("expected 2 lines in buffer, got %d", len(buf))
	}
}
