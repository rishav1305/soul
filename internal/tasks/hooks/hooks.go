package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// HookConfig holds tool and workflow hook definitions loaded from a JSON file.
type HookConfig struct {
	Hooks         []ToolHook     `json:"hooks"`
	WorkflowHooks []WorkflowHook `json:"workflow_hooks"`
}

// ToolHook defines a hook that runs before or after a tool invocation.
type ToolHook struct {
	Event       string `json:"event"`        // "before:code_edit", "after:code_exec"
	Match       string `json:"match"`        // file glob pattern "*.go"
	Command     string `json:"command"`      // shell command with {key} templates
	Timeout     int    `json:"timeout"`      // seconds, default 10
	DenyPattern string `json:"deny_pattern"` // regex to block
	Action      string `json:"action"`       // "block" to prevent execution
	Message     string `json:"message"`      // message when blocking
}

// WorkflowHook defines a hook that runs after workflow events.
type WorkflowHook struct {
	Event   string `json:"event"`   // "after:merge_to_dev", "after:task_done"
	Command string `json:"command"`
	Timeout int    `json:"timeout"` // default 30
}

// HookRunner loads and executes hook definitions from a config file.
type HookRunner struct {
	config     *HookConfig
	configPath string
}

// NewHookRunner creates a HookRunner that loads config from the given JSON file.
// If the file does not exist, an empty config is used.
func NewHookRunner(configPath string) *HookRunner {
	hr := &HookRunner{
		configPath: configPath,
		config:     &HookConfig{},
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return hr
	}

	var cfg HookConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return hr
	}
	hr.config = &cfg
	return hr
}

// RunToolHook executes hooks matching the given phase and tool name.
// phase is "before" or "after". Returns whether the tool call should be blocked,
// a blocking message, and any command output.
func (hr *HookRunner) RunToolHook(phase, toolName string, vars map[string]string) (blocked bool, message string, output string) {
	event := phase + ":" + toolName

	for _, hook := range hr.config.Hooks {
		if hook.Event != event {
			continue
		}

		// If hook has a Match pattern, check against vars["file"].
		if hook.Match != "" {
			file := vars["file"]
			if file == "" {
				continue
			}
			matched, err := filepath.Match(hook.Match, filepath.Base(file))
			if err != nil || !matched {
				continue
			}
		}

		// Blocking hook.
		if hook.Action == "block" {
			return true, hook.Message, ""
		}

		// Command hook.
		if hook.Command != "" {
			timeout := hook.Timeout
			if timeout <= 0 {
				timeout = 10
			}
			cmd := expandVars(hook.Command, vars)
			out, _ := runHookCommand(cmd, timeout)
			output += out
		}
	}

	return false, "", output
}

// RunWorkflowHook executes workflow hooks matching the given event.
func (hr *HookRunner) RunWorkflowHook(event string, vars map[string]string) {
	for _, hook := range hr.config.WorkflowHooks {
		if hook.Event != event {
			continue
		}
		timeout := hook.Timeout
		if timeout <= 0 {
			timeout = 30
		}
		cmd := expandVars(hook.Command, vars)
		runHookCommand(cmd, timeout)
	}
}

// expandVars replaces {key} placeholders in template with values from vars.
func expandVars(template string, vars map[string]string) string {
	result := template
	for key, value := range vars {
		result = strings.ReplaceAll(result, "{"+key+"}", value)
	}
	return result
}

// runHookCommand runs a shell command with a context timeout.
func runHookCommand(command string, timeoutSec int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout

	err := cmd.Run()
	return strings.TrimSpace(stdout.String()), err
}
