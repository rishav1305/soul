package server

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rishav1305/soul/internal/ai"
)

// builtinCodeTools returns Claude tool definitions for code manipulation.
func builtinCodeTools() []ai.ClaudeTool {
	return []ai.ClaudeTool{
		{
			Name:        "code_read",
			Description: "Read the contents of a file. Returns the file content with line numbers.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "File path relative to project root (e.g., web/src/App.tsx)"},
					"start_line": {"type": "integer", "description": "Start reading from this line (1-based, optional)"},
					"end_line": {"type": "integer", "description": "Stop reading at this line (inclusive, optional)"}
				},
				"required": ["path"]
			}`),
		},
		{
			Name:        "code_write",
			Description: "Write or overwrite a file with the given content. Creates parent directories if needed.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path":    {"type": "string", "description": "File path relative to project root"},
					"content": {"type": "string", "description": "Full file content to write"}
				},
				"required": ["path", "content"]
			}`),
		},
		{
			Name:        "code_edit",
			Description: "Replace a specific string in a file. The old_string must match exactly (including whitespace/indentation).",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path":       {"type": "string", "description": "File path relative to project root"},
					"old_string": {"type": "string", "description": "Exact text to find and replace"},
					"new_string": {"type": "string", "description": "Replacement text"}
				},
				"required": ["path", "old_string", "new_string"]
			}`),
		},
		{
			Name:        "code_search",
			Description: "Search for files matching a glob pattern. Returns matching file paths.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"pattern": {"type": "string", "description": "Glob pattern (e.g., web/src/**/*.tsx, internal/**/*.go)"}
				},
				"required": ["pattern"]
			}`),
		},
		{
			Name:        "code_grep",
			Description: "Search file contents for a regex pattern. Returns matching lines with file paths and line numbers.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"pattern": {"type": "string", "description": "Search pattern (literal string or regex)"},
					"path":    {"type": "string", "description": "Directory or file to search in (relative to project root, default: .)"},
					"include": {"type": "string", "description": "File glob to include (e.g., *.tsx, *.go)"}
				},
				"required": ["pattern"]
			}`),
		},
		{
			Name:        "code_exec",
			Description: "Execute a shell command in the project root. Use for building, testing, or verifying changes. Timeout: 60 seconds.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"command": {"type": "string", "description": "Shell command to execute"}
				},
				"required": ["command"]
			}`),
		},
	}
}

// executeCodeTool handles code_* tools using the local filesystem.
func executeCodeTool(projectRoot string, tc toolCall) string {
	var input map[string]any
	if tc.Input != "" {
		if err := json.Unmarshal([]byte(tc.Input), &input); err != nil {
			return fmt.Sprintf("Error parsing input: %v", err)
		}
	}

	switch tc.Name {
	case "code_read":
		return toolCodeRead(projectRoot, input)
	case "code_write":
		return toolCodeWrite(projectRoot, input)
	case "code_edit":
		return toolCodeEdit(projectRoot, input)
	case "code_search":
		return toolCodeSearch(projectRoot, input)
	case "code_grep":
		return toolCodeGrep(projectRoot, input)
	case "code_exec":
		return toolCodeExec(projectRoot, input)
	default:
		return fmt.Sprintf("Error: unknown code tool %q", tc.Name)
	}
}

func resolveCodePath(projectRoot, relPath string) (string, error) {
	// Prevent path traversal.
	cleaned := filepath.Clean(relPath)
	if strings.HasPrefix(cleaned, "..") || filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("invalid path: must be relative to project root")
	}
	return filepath.Join(projectRoot, cleaned), nil
}

func toolCodeRead(root string, input map[string]any) string {
	path, _ := input["path"].(string)
	if path == "" {
		return "Error: path is required"
	}

	fullPath, err := resolveCodePath(root, path)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Sprintf("Error reading file: %v", err)
	}

	lines := strings.Split(string(data), "\n")

	startLine := 1
	endLine := len(lines)
	if s, ok := input["start_line"].(float64); ok && int(s) > 0 {
		startLine = int(s)
	}
	if e, ok := input["end_line"].(float64); ok && int(e) > 0 {
		endLine = int(e)
	}
	if startLine > len(lines) {
		return fmt.Sprintf("Error: start_line %d exceeds file length (%d lines)", startLine, len(lines))
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "File: %s (%d lines)\n", path, len(lines))
	for i := startLine - 1; i < endLine; i++ {
		fmt.Fprintf(&b, "%4d | %s\n", i+1, lines[i])
	}
	return b.String()
}

func toolCodeWrite(root string, input map[string]any) string {
	path, _ := input["path"].(string)
	content, _ := input["content"].(string)
	if path == "" {
		return "Error: path is required"
	}

	fullPath, err := resolveCodePath(root, path)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	// Create parent directories.
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Sprintf("Error creating directory: %v", err)
	}

	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		return fmt.Sprintf("Error writing file: %v", err)
	}

	lines := strings.Count(content, "\n") + 1
	return fmt.Sprintf("Written %d lines to %s", lines, path)
}

func toolCodeEdit(root string, input map[string]any) string {
	path, _ := input["path"].(string)
	oldStr, _ := input["old_string"].(string)
	newStr, _ := input["new_string"].(string)
	if path == "" || oldStr == "" {
		return "Error: path and old_string are required"
	}

	fullPath, err := resolveCodePath(root, path)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Sprintf("Error reading file: %v", err)
	}

	content := string(data)
	count := strings.Count(content, oldStr)
	if count == 0 {
		return "Error: old_string not found in file"
	}
	if count > 1 {
		return fmt.Sprintf("Error: old_string found %d times — must be unique. Provide more context.", count)
	}

	newContent := strings.Replace(content, oldStr, newStr, 1)
	if err := os.WriteFile(fullPath, []byte(newContent), 0o644); err != nil {
		return fmt.Sprintf("Error writing file: %v", err)
	}

	return fmt.Sprintf("Edited %s: replaced 1 occurrence", path)
}

func toolCodeSearch(root string, input map[string]any) string {
	pattern, _ := input["pattern"].(string)
	if pattern == "" {
		return "Error: pattern is required"
	}

	fullPattern := filepath.Join(root, pattern)
	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	if len(matches) == 0 {
		return "No files found matching pattern: " + pattern
	}

	// Return relative paths.
	var b strings.Builder
	fmt.Fprintf(&b, "Found %d files:\n", len(matches))
	for _, m := range matches {
		rel, _ := filepath.Rel(root, m)
		b.WriteString(rel + "\n")
	}
	return b.String()
}

func toolCodeGrep(root string, input map[string]any) string {
	pattern, _ := input["pattern"].(string)
	if pattern == "" {
		return "Error: pattern is required"
	}

	searchPath := "."
	if p, ok := input["path"].(string); ok && p != "" {
		searchPath = p
	}

	args := []string{"-rn", "--color=never"}
	if include, ok := input["include"].(string); ok && include != "" {
		args = append(args, "--include="+include)
	}
	args = append(args, pattern, searchPath)

	cmd := exec.Command("grep", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		// grep returns exit 1 when no matches.
		if len(out) == 0 {
			return "No matches found for: " + pattern
		}
	}

	result := string(out)
	// Limit output to avoid overwhelming Claude.
	lines := strings.Split(result, "\n")
	if len(lines) > 50 {
		result = strings.Join(lines[:50], "\n") + fmt.Sprintf("\n... (%d more lines truncated)", len(lines)-50)
	}

	return result
}

func toolCodeExec(root string, input map[string]any) string {
	command, _ := input["command"].(string)
	if command == "" {
		return "Error: command is required"
	}

	log.Printf("[code_exec] running: %s", command)

	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = root

	// 60 second timeout.
	done := make(chan error, 1)
	var out []byte
	go func() {
		var err error
		out, err = cmd.CombinedOutput()
		done <- err
	}()

	select {
	case err := <-done:
		result := string(out)
		// Limit output.
		if len(result) > 5000 {
			result = result[:5000] + "\n... (output truncated)"
		}
		if err != nil {
			return fmt.Sprintf("Exit error: %v\n%s", err, result)
		}
		return result
	case <-time.After(60 * time.Second):
		cmd.Process.Kill()
		return "Error: command timed out after 60 seconds"
	}
}
