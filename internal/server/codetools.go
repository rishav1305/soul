package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rishav1305/soul/internal/ai"
)

// builtinCodeTools returns Claude tool definitions for code manipulation.
func builtinCodeTools() []ai.ClaudeTool {
	return []ai.ClaudeTool{
		{
			Name:        "code_read",
			Description: "Read the contents of a file. Returns the file content with line numbers. For large files (>200 lines) without line range, returns a summary of exports/signatures instead — use start_line/end_line to read specific sections.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "File path relative to project root (e.g., web/src/App.tsx)"},
					"start_line": {"type": "integer", "description": "Start reading from this line (1-based, optional)"},
					"end_line": {"type": "integer", "description": "Stop reading at this line (inclusive, optional)"},
					"summary": {"type": "boolean", "description": "Return only function/type signatures and exports (optional, default false)"}
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
			Description: "Search file contents for a regex pattern. Returns matching lines grouped by file with line numbers.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"pattern": {"type": "string", "description": "Search pattern (literal string or regex)"},
					"path":    {"type": "string", "description": "Directory or file to search in (relative to project root, default: .)"},
					"include": {"type": "string", "description": "File glob to include (e.g., *.tsx, *.go)"},
					"context_lines": {"type": "integer", "description": "Number of context lines before and after each match (0-3, default 0)"},
					"max_results": {"type": "integer", "description": "Maximum matching lines to return (default 50, max 200)"}
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
		{
			Name:        "code_glob",
			Description: "Find files matching a glob pattern with ** support for recursive matching. Faster than code_search for discovering files by extension or name pattern.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"pattern": {"type": "string", "description": "Glob pattern with ** support (e.g., **/*.tsx, web/src/**/*.ts, internal/**/*_test.go)"},
					"path":    {"type": "string", "description": "Base directory to search from (relative to project root, default: .)"}
				},
				"required": ["pattern"]
			}`),
		},
		{
			Name:        "task_memory",
			Description: "Store or recall facts discovered during task execution. Use 'store' to save a fact, 'recall' to retrieve. Avoids re-searching for the same information. Check memory before searching.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"action": {"type": "string", "enum": ["store", "recall", "list"], "description": "store: save a key-value fact. recall: get a fact by key. list: show all stored facts."},
					"key": {"type": "string", "description": "Fact key (e.g., 'inputbar_location', 'design_tokens')"},
					"value": {"type": "string", "description": "Fact value (for store action)"}
				},
				"required": ["action"]
			}`),
		},
		{
			Name:        "subagent",
			Description: "Spawn a focused sub-agent for codebase exploration. Gets fresh context (no pollution from current conversation). Limited to 5 iterations and read-only tools. Use for 'find all X' or 'understand how Y works' queries.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"task": {"type": "string", "description": "Clear exploration task (e.g., 'Find all components that import useChat hook', 'List all API endpoints in routes.go')"},
					"max_iterations": {"type": "integer", "description": "Max iterations for sub-agent (default 5, max 10)"}
				},
				"required": ["task"]
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
	case "code_glob":
		return toolCodeGlob(projectRoot, input)
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

	// Summary mode: return only function/type signatures and exports.
	wantSummary, _ := input["summary"].(bool)
	if wantSummary {
		return fileSummary(path, lines)
	}

	_, hasStart := input["start_line"].(float64)
	_, hasEnd := input["end_line"].(float64)

	// Auto-summary for large files without line range.
	if len(lines) > 200 && !hasStart && !hasEnd {
		summary := fileSummary(path, lines)
		return summary + fmt.Sprintf("\n\nFile has %d lines. Use start_line/end_line to read specific sections.", len(lines))
	}

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

// fileSummary extracts function/type signatures, exports, and key structural lines.
func fileSummary(path string, lines []string) string {
	ext := filepath.Ext(path)
	var b strings.Builder
	fmt.Fprintf(&b, "File: %s (%d lines) — SUMMARY\n\n", path, len(lines))

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		show := false
		switch ext {
		case ".go":
			show = strings.HasPrefix(trimmed, "func ") ||
				strings.HasPrefix(trimmed, "type ") ||
				strings.HasPrefix(trimmed, "package ") ||
				strings.HasPrefix(trimmed, "import ")
		case ".ts", ".tsx", ".js", ".jsx":
			show = strings.HasPrefix(trimmed, "export ") ||
				strings.HasPrefix(trimmed, "import ") ||
				strings.HasPrefix(trimmed, "interface ") ||
				strings.HasPrefix(trimmed, "type ") ||
				strings.HasPrefix(trimmed, "function ") ||
				strings.HasPrefix(trimmed, "const ") ||
				strings.HasPrefix(trimmed, "class ")
		case ".css":
			show = strings.HasPrefix(trimmed, "@") ||
				strings.HasPrefix(trimmed, "--") ||
				(strings.Contains(trimmed, "{") && !strings.HasPrefix(trimmed, "/*"))
		default:
			show = strings.HasPrefix(trimmed, "func ") ||
				strings.HasPrefix(trimmed, "export ") ||
				strings.HasPrefix(trimmed, "class ") ||
				strings.HasPrefix(trimmed, "def ")
		}
		if show {
			fmt.Fprintf(&b, "%4d | %s\n", i+1, line)
		}
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

	// Context lines (0-3).
	contextLines := 0
	if c, ok := input["context_lines"].(float64); ok && int(c) >= 0 && int(c) <= 3 {
		contextLines = int(c)
	}

	// Max results (default 50, max 200).
	maxResults := 50
	if m, ok := input["max_results"].(float64); ok && int(m) > 0 {
		maxResults = int(m)
		if maxResults > 200 {
			maxResults = 200
		}
	}

	args := []string{"-rn", "--color=never"}
	if contextLines > 0 {
		args = append(args, fmt.Sprintf("-C%d", contextLines))
	}
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
	lines := strings.Split(result, "\n")
	if len(lines) > maxResults {
		result = strings.Join(lines[:maxResults], "\n") + fmt.Sprintf("\n... (%d more lines truncated)", len(lines)-maxResults)
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

// toolCodeGlob finds files matching a glob pattern with ** (recursive) support.
func toolCodeGlob(root string, input map[string]any) string {
	pattern, _ := input["pattern"].(string)
	if pattern == "" {
		return "Error: pattern is required"
	}

	basePath := "."
	if p, ok := input["path"].(string); ok && p != "" {
		basePath = p
	}

	searchRoot, err := resolveCodePath(root, basePath)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	// Skip directories.
	skipDirs := map[string]bool{
		"node_modules": true, ".git": true, "dist": true,
		".worktrees": true, "vendor": true, "__pycache__": true,
	}

	// Split pattern into directory parts and filename pattern.
	// For "**/*.tsx", we walk everything and match the filename.
	// For "web/src/**/*.ts", we only walk under web/src/.
	var matches []string
	err = filepath.WalkDir(searchRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		// Match against the pattern using filepath.Match on the filename for simple patterns,
		// or match the full relative path for complex patterns.
		if globMatch(pattern, rel) {
			matches = append(matches, rel)
		}
		return nil
	})
	if err != nil {
		return fmt.Sprintf("Error walking directory: %v", err)
	}

	if len(matches) == 0 {
		return "No files found matching: " + pattern
	}

	sort.Strings(matches)
	if len(matches) > 50 {
		matches = matches[:50]
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d files:\n", len(matches))
	for _, m := range matches {
		b.WriteString(m + "\n")
	}
	return b.String()
}

// globMatch matches a path against a glob pattern with ** support.
// ** matches any number of path segments.
func globMatch(pattern, path string) bool {
	// Split into segments.
	patParts := strings.Split(filepath.ToSlash(pattern), "/")
	pathParts := strings.Split(filepath.ToSlash(path), "/")
	return globMatchParts(patParts, pathParts)
}

func globMatchParts(pattern, path []string) bool {
	for len(pattern) > 0 && len(path) > 0 {
		if pattern[0] == "**" {
			// ** matches zero or more path segments.
			pattern = pattern[1:]
			if len(pattern) == 0 {
				return true // trailing ** matches everything
			}
			// Try matching the rest of the pattern against each suffix of path.
			for i := 0; i <= len(path); i++ {
				if globMatchParts(pattern, path[i:]) {
					return true
				}
			}
			return false
		}
		matched, _ := filepath.Match(pattern[0], path[0])
		if !matched {
			return false
		}
		pattern = pattern[1:]
		path = path[1:]
	}
	// Handle trailing ** in pattern.
	for len(pattern) > 0 && pattern[0] == "**" {
		pattern = pattern[1:]
	}
	return len(pattern) == 0 && len(path) == 0
}
