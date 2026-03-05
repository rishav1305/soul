package server

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// HookConfig represents the hooks configuration file.
type HookConfig struct {
	Hooks         []ToolHook     `json:"hooks"`
	WorkflowHooks []WorkflowHook `json:"workflow_hooks"`
}

// ToolHook defines a pre/post tool execution hook.
type ToolHook struct {
	Event       string `json:"event"`        // "before:code_edit", "after:code_exec", etc.
	Match       string `json:"match"`        // file glob pattern (e.g., "*.tsx", "*.go")
	Command     string `json:"command"`      // shell command with template variables
	Timeout     int    `json:"timeout"`      // seconds (default 10)
	DenyPattern string `json:"deny_pattern"` // regex pattern to block (for before: hooks)
	Action      string `json:"action"`       // "block" to prevent tool execution
	Message     string `json:"message"`      // message when blocking
}

// WorkflowHook defines a hook at workflow milestones.
type WorkflowHook struct {
	Event   string `json:"event"`   // "after:merge_to_dev", "after:task_done", etc.
	Command string `json:"command"` // shell command with template variables
	Timeout int    `json:"timeout"` // seconds (default 30)
}

// HookRunner manages and executes hooks.
type HookRunner struct {
	config     *HookConfig
	configPath string
}

// NewHookRunner creates a hook runner, loading config from ~/.soul/hooks.json.
func NewHookRunner() *HookRunner {
	home, err := os.UserHomeDir()
	if err != nil {
		return &HookRunner{}
	}
	configPath := filepath.Join(home, ".soul", "hooks.json")

	hr := &HookRunner{configPath: configPath}
	hr.reload()
	return hr
}

// reload loads or reloads the hook configuration from disk.
func (hr *HookRunner) reload() {
	data, err := os.ReadFile(hr.configPath)
	if err != nil {
		hr.config = &HookConfig{}
		return
	}
	var cfg HookConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("[hooks] failed to parse %s: %v", hr.configPath, err)
		hr.config = &HookConfig{}
		return
	}
	hr.config = &cfg
	log.Printf("[hooks] loaded %d tool hooks, %d workflow hooks", len(cfg.Hooks), len(cfg.WorkflowHooks))
}

// RunToolHook executes matching hooks for a tool event.
// phase is "before" or "after".
// toolName is the tool being called (e.g., "code_edit").
// vars contains template variables: file, worktree, task_id, tool_name.
// Returns (blocked bool, message string, hookOutput string).
func (hr *HookRunner) RunToolHook(phase, toolName string, vars map[string]string) (bool, string, string) {
	if hr.config == nil || len(hr.config.Hooks) == 0 {
		return false, "", ""
	}

	event := phase + ":" + toolName
	var outputs []string

	for _, hook := range hr.config.Hooks {
		if hook.Event != event {
			continue
		}

		// Check file match pattern.
		if hook.Match != "" {
			file := vars["file"]
			if file == "" {
				continue
			}
			matched, _ := filepath.Match(hook.Match, filepath.Base(file))
			if !matched {
				continue
			}
		}

		// Check deny pattern (for before: hooks).
		if phase == "before" && hook.DenyPattern != "" {
			re, err := regexp.Compile(hook.DenyPattern)
			if err == nil {
				// Check against the command/input being passed.
				if input, ok := vars["input"]; ok && re.MatchString(input) {
					msg := hook.Message
					if msg == "" {
						msg = fmt.Sprintf("Blocked by hook: pattern %q matched", hook.DenyPattern)
					}
					return true, msg, ""
				}
			}
		}

		// Block action.
		if phase == "before" && hook.Action == "block" {
			msg := hook.Message
			if msg == "" {
				msg = "Blocked by hook"
			}
			return true, msg, ""
		}

		// Execute command.
		if hook.Command != "" {
			cmd := expandVars(hook.Command, vars)
			timeout := hook.Timeout
			if timeout <= 0 {
				timeout = 10
			}
			output, err := runHookCommand(cmd, timeout)
			if err != nil {
				if phase == "before" {
					return true, fmt.Sprintf("Hook failed: %v", err), ""
				}
				log.Printf("[hooks] after hook failed: %v", err)
			}
			if output != "" {
				outputs = append(outputs, fmt.Sprintf("[Hook: %s]", strings.TrimSpace(output)))
			}
		}
	}

	return false, "", strings.Join(outputs, " ")
}

// RunWorkflowHook executes matching workflow hooks.
func (hr *HookRunner) RunWorkflowHook(event string, vars map[string]string) {
	if hr.config == nil || len(hr.config.WorkflowHooks) == 0 {
		return
	}

	for _, hook := range hr.config.WorkflowHooks {
		if hook.Event != event {
			continue
		}
		cmd := expandVars(hook.Command, vars)
		timeout := hook.Timeout
		if timeout <= 0 {
			timeout = 30
		}
		output, err := runHookCommand(cmd, timeout)
		if err != nil {
			log.Printf("[hooks] workflow hook %s failed: %v", event, err)
		} else if output != "" {
			log.Printf("[hooks] workflow hook %s: %s", event, strings.TrimSpace(output))
		}
	}
}

// expandVars replaces {key} placeholders in a string with values from vars.
func expandVars(template string, vars map[string]string) string {
	result := template
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{"+k+"}", v)
	}
	return result
}

// runHookCommand executes a shell command with a timeout.
func runHookCommand(command string, timeoutSec int) (string, error) {
	cmd := exec.Command("bash", "-c", command)
	done := make(chan error, 1)
	var out []byte
	go func() {
		var err error
		out, err = cmd.CombinedOutput()
		done <- err
	}()

	select {
	case err := <-done:
		return string(out), err
	case <-time.After(time.Duration(timeoutSec) * time.Second):
		cmd.Process.Kill()
		return "", fmt.Errorf("hook timed out after %ds", timeoutSec)
	}
}
