package team

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseTimeOr(t *testing.T) {
	tests := []struct {
		input    string
		wantZero bool
	}{
		{"2026-03-31T10:00:00Z", false},
		{"2026-03-31T10:00:00+05:30", false},
		{"2026-03-31T10:00:00-07:00", false},
		{"", true},
		{"garbage", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseTimeOr(tt.input, time.Time{})
			if tt.wantZero && !result.IsZero() {
				t.Errorf("expected zero time for %q", tt.input)
			}
			if !tt.wantZero && result.IsZero() {
				t.Errorf("expected non-zero time for %q", tt.input)
			}
		})
	}
}

func TestAgentStateAggregator_ReadHeartbeats(t *testing.T) {
	dir := t.TempDir()
	hbDir := filepath.Join(dir, "heartbeat")
	if err := os.MkdirAll(hbDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write test heartbeat.
	hbJSON := `{
		"agent": "shuri",
		"status": "working",
		"current_task": "Building Soul Control",
		"since": "2026-03-31T10:00:00+05:30",
		"updated": "` + time.Now().Format(time.RFC3339) + `",
		"blocked_on": null,
		"next_task": "Frontend phase"
	}`
	if err := os.WriteFile(filepath.Join(hbDir, "shuri.json"), []byte(hbJSON), 0644); err != nil {
		t.Fatal(err)
	}

	panes := map[string]string{"shuri": "%5"}
	cfg := Config{
		HeartbeatDir: hbDir,
		ClawteamDir:  dir, // empty, no inboxes
	}

	agg := NewAgentStateAggregator(cfg, panes)
	if err := agg.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	state, ok := agg.Get("shuri")
	if !ok {
		t.Fatal("expected shuri state")
	}
	if state.Status != "working" {
		t.Errorf("expected status working, got %s", state.Status)
	}
	if state.CurrentTask != "Building Soul Control" {
		t.Errorf("expected task 'Building Soul Control', got %s", state.CurrentTask)
	}
	if state.Machine != "titan-pc" {
		t.Errorf("expected machine titan-pc, got %s", state.Machine)
	}
}

func TestAgentStateAggregator_StaleHeartbeat(t *testing.T) {
	dir := t.TempDir()
	hbDir := filepath.Join(dir, "heartbeat")
	if err := os.MkdirAll(hbDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write stale heartbeat (10 minutes ago).
	staleTime := time.Now().Add(-10 * time.Minute).Format(time.RFC3339)
	hbJSON := `{
		"agent": "happy",
		"status": "working",
		"current_task": "old task",
		"since": "` + staleTime + `",
		"updated": "` + staleTime + `",
		"blocked_on": null,
		"next_task": ""
	}`
	if err := os.WriteFile(filepath.Join(hbDir, "happy.json"), []byte(hbJSON), 0644); err != nil {
		t.Fatal(err)
	}

	panes := map[string]string{"happy": "%8"}
	cfg := Config{
		HeartbeatDir: hbDir,
		ClawteamDir:  dir,
	}

	agg := NewAgentStateAggregator(cfg, panes)
	if err := agg.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	state, ok := agg.Get("happy")
	if !ok {
		t.Fatal("expected happy state")
	}
	if state.Status != "offline" {
		t.Errorf("expected offline for stale heartbeat, got %s", state.Status)
	}
}

func TestAgentStateAggregator_NoHeartbeatDir(t *testing.T) {
	panes := map[string]string{"test": "%0"}
	cfg := Config{
		HeartbeatDir: "/nonexistent/heartbeat",
		ClawteamDir:  "/nonexistent/clawteam",
	}

	agg := NewAgentStateAggregator(cfg, panes)
	if err := agg.Refresh(); err != nil {
		t.Fatalf("Refresh should not error on missing dir: %v", err)
	}

	state, ok := agg.Get("test")
	if !ok {
		t.Fatal("expected test state")
	}
	if state.Status != "offline" {
		t.Errorf("expected offline, got %s", state.Status)
	}
}

func TestAgentStateAggregator_GetAll(t *testing.T) {
	dir := t.TempDir()
	hbDir := filepath.Join(dir, "heartbeat")
	_ = os.MkdirAll(hbDir, 0755)

	panes := map[string]string{"a": "%0", "b": "%1"}
	cfg := Config{
		HeartbeatDir: hbDir,
		ClawteamDir:  dir,
	}

	agg := NewAgentStateAggregator(cfg, panes)
	_ = agg.Refresh()

	all := agg.GetAll()
	if len(all) != 2 {
		t.Errorf("expected 2 agents, got %d", len(all))
	}
}

func TestAgentStateAggregator_CountInboxMessages(t *testing.T) {
	dir := t.TempDir()
	inboxDir := filepath.Join(dir, "inboxes", "shuri_shuri", "new")
	if err := os.MkdirAll(inboxDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write 2 message files.
	for _, name := range []string{"msg1.json", "msg2.json"} {
		if err := os.WriteFile(filepath.Join(inboxDir, name), []byte(`{}`), 0644); err != nil {
			t.Fatal(err)
		}
	}

	panes := map[string]string{"shuri": "%5"}
	cfg := Config{
		HeartbeatDir: filepath.Join(dir, "heartbeat"),
		ClawteamDir:  dir,
	}

	agg := NewAgentStateAggregator(cfg, panes)

	counts := agg.countInboxMessages()
	if counts["shuri"] != 2 {
		t.Errorf("expected 2 messages for shuri, got %d", counts["shuri"])
	}
}
