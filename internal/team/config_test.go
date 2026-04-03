package team

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", cfg.Host)
	}
	if cfg.Port != 3028 {
		t.Errorf("expected port 3028, got %d", cfg.Port)
	}
	if cfg.TeamName != "soul-team" {
		t.Errorf("expected team soul-team, got %s", cfg.TeamName)
	}
	if cfg.PollIntervalMS != 1000 {
		t.Errorf("expected poll interval 1000, got %d", cfg.PollIntervalMS)
	}
	if cfg.PaneBufferSize != 500 {
		t.Errorf("expected pane buffer 500, got %d", cfg.PaneBufferSize)
	}
}

func TestDefaultConfigFromEnv(t *testing.T) {
	t.Setenv("SOUL_TEAM_HOST", "0.0.0.0")
	t.Setenv("SOUL_TEAM_PORT", "9999")
	t.Setenv("SOUL_TEAM_NAME", "test-team")

	cfg := DefaultConfig()

	if cfg.Host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0, got %s", cfg.Host)
	}
	if cfg.Port != 9999 {
		t.Errorf("expected port 9999, got %d", cfg.Port)
	}
	if cfg.TeamName != "test-team" {
		t.Errorf("expected team test-team, got %s", cfg.TeamName)
	}
}

func TestEnvInt(t *testing.T) {
	tests := []struct {
		value    string
		expected int
	}{
		{"42", 42},
		{"0", 0},
		{"", 99},
		{"abc", 99},
		{"-1", 99}, // negative not supported by our simple parser
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			if tt.value != "" {
				t.Setenv("TEST_INT", tt.value)
			}
			got := envInt("TEST_INT", 99)
			if got != tt.expected {
				t.Errorf("envInt(%q, 99) = %d, want %d", tt.value, got, tt.expected)
			}
		})
	}
}

func TestLoadPanesMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "panes.json")

	// Valid panes.json.
	err := os.WriteFile(path, []byte(`{"shuri": "%5", "happy": "%8"}`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	panes, err := LoadPanesMap(path)
	if err != nil {
		t.Fatalf("LoadPanesMap: %v", err)
	}
	if panes["shuri"] != "%5" {
		t.Errorf("expected shuri=%%5, got %s", panes["shuri"])
	}
	if panes["happy"] != "%8" {
		t.Errorf("expected happy=%%8, got %s", panes["happy"])
	}
}

func TestLoadPanesMapErrors(t *testing.T) {
	// Non-existent file.
	_, err := LoadPanesMap("/nonexistent/panes.json")
	if err == nil {
		t.Error("expected error for non-existent file")
	}

	// Invalid JSON.
	dir := t.TempDir()
	path := filepath.Join(dir, "panes.json")
	_ = os.WriteFile(path, []byte(`not json`), 0644)
	_, err = LoadPanesMap(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestAgentMachineMap(t *testing.T) {
	m := AgentMachineMap()

	if m["shuri"] != "titan-pc" {
		t.Errorf("shuri should be titan-pc, got %s", m["shuri"])
	}
	if m["happy"] != "titan-pi" {
		t.Errorf("happy should be titan-pi, got %s", m["happy"])
	}
	if m["banner"] != "titan-pc" {
		t.Errorf("banner should be titan-pc, got %s", m["banner"])
	}

	// Verify all agents present.
	expected := []string{"team-lead", "happy", "xavier", "hawkeye", "pepper", "fury", "loki", "shuri", "stark", "banner"}
	for _, name := range expected {
		if _, ok := m[name]; !ok {
			t.Errorf("agent %q missing from machine map", name)
		}
	}
}
