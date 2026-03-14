package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rishav1305/soul-v2/internal/chat/stream"
	"github.com/rishav1305/soul-v2/internal/tasks/store"
)

const bashTimeout = 30 * time.Second

const (
	maxFileReadBytes  = 100 * 1024 // 100KB
	maxBashOutputBytes = 50 * 1024  // 50KB
	maxListEntries    = 500
)

// skipDirs contains directory names that should be skipped during file listing.
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	".worktrees":   true,
	"dist":         true,
}

// ToolSet holds the root directory and store for executing agent tools.
type ToolSet struct {
	rootDir string
	store   *store.Store
}

// NewToolSet creates a new ToolSet rooted at rootDir.
func NewToolSet(rootDir string, s *store.Store) *ToolSet {
	return &ToolSet{rootDir: rootDir, store: s}
}

// Definitions returns the list of tool definitions with JSON schemas.
func (ts *ToolSet) Definitions() []stream.Tool {
	tools := []stream.Tool{
		{
			Name:        "file_read",
			Description: "Read the contents of a file. Returns the file content, truncated at 100KB.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Path to the file to read, relative to the project root."
					}
				},
				"required": ["path"]
			}`),
		},
		{
			Name:        "file_write",
			Description: "Write content to a file, creating parent directories as needed.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Path to the file to write, relative to the project root."
					},
					"content": {
						"type": "string",
						"description": "Content to write to the file."
					}
				},
				"required": ["path", "content"]
			}`),
		},
		{
			Name:        "bash",
			Description: "Execute a bash command with a 30-second timeout. Returns combined stdout and stderr.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"command": {
						"type": "string",
						"description": "The bash command to execute."
					}
				},
				"required": ["command"]
			}`),
		},
		{
			Name:        "list_files",
			Description: "List files in a directory. Skips .git, node_modules, .worktrees, and dist directories.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Directory path to list, relative to project root. Defaults to root if omitted."
					},
					"recursive": {
						"type": "boolean",
						"description": "Whether to list files recursively. Defaults to false."
					}
				}
			}`),
		},
		{
			Name:        "task_update",
			Description: "Update the current task stage or add a note. Stage must be 'validation' or 'blocked'.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"stage": {
						"type": "string",
						"enum": ["validation", "blocked"],
						"description": "New stage for the task."
					},
					"note": {
						"type": "string",
						"description": "Optional note to record with the update."
					}
				}
			}`),
		},
	}
	return tools
}

// Execute dispatches a tool call by name and returns its output.
func (ts *ToolSet) Execute(name, input string) (string, error) {
	switch name {
	case "file_read":
		return ts.execFileRead(input)
	case "file_write":
		return ts.execFileWrite(input)
	case "bash":
		return ts.execBash(input)
	case "list_files":
		return ts.execListFiles(input)
	case "task_update":
		return ts.execTaskUpdate(input)
	default:
		return "", fmt.Errorf("unknown tool: %q", name)
	}
}

// resolvePath resolves a relative path against rootDir and prevents path traversal.
func (ts *ToolSet) resolvePath(relPath string) (string, error) {
	if relPath == "" {
		return ts.rootDir, nil
	}
	// Join and clean the path.
	abs := filepath.Join(ts.rootDir, relPath)
	abs = filepath.Clean(abs)

	// Ensure the resolved path is within rootDir.
	root := filepath.Clean(ts.rootDir)
	if !strings.HasPrefix(abs, root+string(filepath.Separator)) && abs != root {
		return "", fmt.Errorf("path traversal not allowed: %q resolves outside root", relPath)
	}
	return abs, nil
}

// execFileRead reads a file and returns its content, truncated at 100KB.
func (ts *ToolSet) execFileRead(input string) (string, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("file_read: invalid input: %w", err)
	}
	if params.Path == "" {
		return "", fmt.Errorf("file_read: path is required")
	}

	abs, err := ts.resolvePath(params.Path)
	if err != nil {
		return "", fmt.Errorf("file_read: %w", err)
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("file_read: %w", err)
	}

	if len(data) > maxFileReadBytes {
		data = data[:maxFileReadBytes]
		return string(data) + "\n[truncated at 100KB]", nil
	}
	return string(data), nil
}

// execFileWrite writes content to a file, creating parent directories as needed.
func (ts *ToolSet) execFileWrite(input string) (string, error) {
	var params struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("file_write: invalid input: %w", err)
	}
	if params.Path == "" {
		return "", fmt.Errorf("file_write: path is required")
	}

	abs, err := ts.resolvePath(params.Path)
	if err != nil {
		return "", fmt.Errorf("file_write: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", fmt.Errorf("file_write: create directories: %w", err)
	}

	if err := os.WriteFile(abs, []byte(params.Content), 0o644); err != nil {
		return "", fmt.Errorf("file_write: %w", err)
	}

	return fmt.Sprintf("wrote %d bytes to %s", len(params.Content), params.Path), nil
}

// execBash runs a bash command with a 30-second timeout.
func (ts *ToolSet) execBash(input string) (string, error) {
	var params struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("bash: invalid input: %w", err)
	}
	if params.Command == "" {
		return "", fmt.Errorf("bash: command is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), bashTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", params.Command)
	cmd.Dir = ts.rootDir

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	runErr := cmd.Run()

	output := buf.String()
	if len(output) > maxBashOutputBytes {
		output = output[:maxBashOutputBytes] + "\n[truncated at 50KB]"
	}

	if runErr != nil {
		// Return output + error status, not a Go error — the agent needs to see it.
		exitMsg := fmt.Sprintf("\n[exit status: %v]", runErr)
		return output + exitMsg, nil
	}
	return output, nil
}

// execListFiles lists files in a directory, with optional recursion.
func (ts *ToolSet) execListFiles(input string) (string, error) {
	var params struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
	}
	if input != "" && input != "{}" {
		if err := json.Unmarshal([]byte(input), &params); err != nil {
			return "", fmt.Errorf("list_files: invalid input: %w", err)
		}
	}

	abs, err := ts.resolvePath(params.Path)
	if err != nil {
		return "", fmt.Errorf("list_files: %w", err)
	}

	var entries []string

	if params.Recursive {
		err = filepath.WalkDir(abs, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil // skip unreadable entries
			}
			if d.IsDir() && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			if !d.IsDir() {
				rel, _ := filepath.Rel(abs, path)
				entries = append(entries, rel)
				if len(entries) >= maxListEntries {
					return filepath.SkipAll
				}
			}
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("list_files: walk: %w", err)
		}
	} else {
		dirEntries, err := os.ReadDir(abs)
		if err != nil {
			return "", fmt.Errorf("list_files: read dir: %w", err)
		}
		for _, de := range dirEntries {
			name := de.Name()
			if de.IsDir() {
				name += "/"
			}
			entries = append(entries, name)
		}
	}

	if len(entries) == 0 {
		return "(empty directory)", nil
	}
	return strings.Join(entries, "\n"), nil
}

// execTaskUpdate is a placeholder that returns a formatted update string.
func (ts *ToolSet) execTaskUpdate(input string) (string, error) {
	var params struct {
		Stage string `json:"stage"`
		Note  string `json:"note"`
	}
	if input != "" && input != "{}" {
		if err := json.Unmarshal([]byte(input), &params); err != nil {
			return "", fmt.Errorf("task_update: invalid input: %w", err)
		}
	}

	var parts []string
	if params.Stage != "" {
		parts = append(parts, fmt.Sprintf("stage=%s", params.Stage))
	}
	if params.Note != "" {
		parts = append(parts, fmt.Sprintf("note=%q", params.Note))
	}

	if len(parts) == 0 {
		return "task_update: no changes requested", nil
	}
	return fmt.Sprintf("task_update: %s", strings.Join(parts, " ")), nil
}
