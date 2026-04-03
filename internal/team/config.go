package team

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds all team server configuration, resolved from env vars with defaults.
type Config struct {
	Host        string // bind address
	Port        int    // listen port
	TeamName    string // clawteam team name
	ClawteamDir string // ~/.clawteam/teams/{team}
	HeartbeatDir string // ~/.local/share/assistant/heartbeat
	BacklogCLI  string // path to backlog_cli.py
	TmuxSession string // tmux session name (for pane capture)
	PanesFile   string // path to panes.json
	PollIntervalMS int // pane poll interval in milliseconds
	PaneBufferSize int // max lines per agent pane buffer
}

// DefaultConfig returns a Config populated from environment variables with sensible defaults.
func DefaultConfig() Config {
	home, _ := os.UserHomeDir()
	teamName := envOr("SOUL_TEAM_NAME", "soul-team")

	return Config{
		Host:           envOr("SOUL_TEAM_HOST", "127.0.0.1"),
		Port:           envInt("SOUL_TEAM_PORT", 3028),
		TeamName:       teamName,
		ClawteamDir:    envOr("SOUL_TEAM_CLAWTEAM_DIR", filepath.Join(home, ".clawteam", "teams", teamName)),
		HeartbeatDir:   envOr("SOUL_TEAM_HEARTBEAT_DIR", filepath.Join(home, ".local", "share", "assistant", "heartbeat")),
		BacklogCLI:     envOr("SOUL_TEAM_BACKLOG_CLI", filepath.Join(home, ".claude", "skills", "pa-backlog", "backlog_cli.py")),
		TmuxSession:    envOr("SOUL_TEAM_TMUX_SESSION", teamName),
		PanesFile:      envOr("SOUL_TEAM_PANES_FILE", filepath.Join(home, ".clawteam", "teams", teamName, "panes.json")),
		PollIntervalMS: envInt("SOUL_TEAM_POLL_MS", 1000),
		PaneBufferSize: envInt("SOUL_TEAM_PANE_BUFFER", 500),
	}
}

// AgentMachineMap returns the known agent-to-machine mapping.
// This is static configuration based on the soul-team layout.
func AgentMachineMap() map[string]string {
	return map[string]string{
		"team-lead": "titan-pi",
		"happy":     "titan-pi",
		"xavier":    "titan-pi",
		"hawkeye":   "titan-pi",
		"pepper":    "titan-pi",
		"fury":      "titan-pi",
		"loki":      "titan-pi",
		"shuri":     "titan-pc",
		"stark":     "titan-pc",
		"banner":    "titan-pc",
	}
}

// LoadPanesMap reads panes.json and returns agent -> pane_id mapping.
func LoadPanesMap(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read panes file: %w", err)
	}
	var panes map[string]string
	if err := json.Unmarshal(data, &panes); err != nil {
		return nil, fmt.Errorf("parse panes file: %w", err)
	}
	return panes, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n := 0
	for _, c := range v {
		if c < '0' || c > '9' {
			return fallback
		}
		n = n*10 + int(c-'0')
	}
	return n
}
