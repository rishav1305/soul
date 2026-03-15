package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const maxFileReadBytes = 100 * 1024 // 100KB
const maxGrepOutputBytes = 50 * 1024 // 50KB
const maxEntries = 500
const toolTimeout = 10 * time.Second

// skipDirs contains directory names excluded from search operations.
var skipDirs = []string{".git", "node_modules", ".worktrees", "dist"}

// executeReadOnlyTool dispatches a read-only tool call by name.
func executeReadOnlyTool(projectRoot, name, input string) string {
	switch name {
	case "file_read":
		return execROFileRead(projectRoot, input)
	case "file_search":
		return execROFileSearch(projectRoot, input)
	case "file_grep":
		return execROFileGrep(projectRoot, input)
	case "file_glob":
		return execROFileGlob(projectRoot, input)
	default:
		return fmt.Sprintf("error: tool %q is not available in read-only mode", name)
	}
}

// resolvePath resolves a path against rootDir and prevents path traversal.
func resolvePath(rootDir, path string) (string, error) {
	if path == "" {
		return rootDir, nil
	}
	abs := filepath.Clean(filepath.Join(rootDir, path))
	root := filepath.Clean(rootDir)
	if abs != root && !strings.HasPrefix(abs, root+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal not allowed: %q resolves outside root", path)
	}
	return abs, nil
}

// execROFileRead reads a file, truncating at 100KB.
func execROFileRead(rootDir, input string) string {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return fmt.Sprintf("error: invalid input: %v", err)
	}
	if params.Path == "" {
		return "error: path is required"
	}

	abs, err := resolvePath(rootDir, params.Path)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	if len(data) > maxFileReadBytes {
		return string(data[:maxFileReadBytes]) + "\n[truncated at 100KB]"
	}
	return string(data)
}

// execROFileSearch searches for files matching a query string using find.
func execROFileSearch(rootDir, input string) string {
	var params struct {
		Query     string `json:"query"`
		Directory string `json:"directory"`
	}
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return fmt.Sprintf("error: invalid input: %v", err)
	}
	if params.Query == "" {
		return "error: query is required"
	}

	searchDir := rootDir
	if params.Directory != "" {
		resolved, err := resolvePath(rootDir, params.Directory)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		searchDir = resolved
	}

	// Build find command with exclusions.
	var excludes []string
	for _, d := range skipDirs {
		excludes = append(excludes, "-name", d, "-prune", "-o")
	}
	args := append([]string{searchDir}, excludes...)
	args = append(args, "-name", fmt.Sprintf("*%s*", params.Query), "-print")

	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "find", args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil && ctx.Err() != nil {
		return "error: search timed out"
	}

	output := buf.String()
	if output == "" {
		return "no files found"
	}

	// Make paths relative and limit entries.
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var results []string
	root := filepath.Clean(rootDir)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		rel, err := filepath.Rel(root, line)
		if err != nil {
			rel = line
		}
		results = append(results, rel)
		if len(results) >= maxEntries {
			break
		}
	}

	if len(results) == 0 {
		return "no files found"
	}
	return strings.Join(results, "\n")
}

// execROFileGrep searches file contents using grep.
func execROFileGrep(rootDir, input string) string {
	var params struct {
		Pattern   string `json:"pattern"`
		Directory string `json:"directory"`
		Include   string `json:"include"`
	}
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return fmt.Sprintf("error: invalid input: %v", err)
	}
	if params.Pattern == "" {
		return "error: pattern is required"
	}

	searchDir := rootDir
	if params.Directory != "" {
		resolved, err := resolvePath(rootDir, params.Directory)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		searchDir = resolved
	}

	args := []string{"-rn"}

	// Add exclusions.
	for _, d := range skipDirs {
		args = append(args, fmt.Sprintf("--exclude-dir=%s", d))
	}

	// Add include filter if specified.
	if params.Include != "" {
		args = append(args, fmt.Sprintf("--include=%s", params.Include))
	}

	args = append(args, params.Pattern, searchDir)

	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "grep", args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	runErr := cmd.Run()

	output := buf.String()
	if len(output) > maxGrepOutputBytes {
		output = output[:maxGrepOutputBytes] + "\n[truncated at 50KB]"
	}

	if output == "" {
		if runErr != nil && ctx.Err() != nil {
			return "error: grep timed out"
		}
		return "no matches found"
	}

	// Make paths relative.
	root := filepath.Clean(rootDir)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var results []string
	for _, line := range lines {
		if strings.HasPrefix(line, root+string(filepath.Separator)) {
			line = line[len(root)+1:]
		}
		results = append(results, line)
	}

	return strings.Join(results, "\n")
}

// execROFileGlob finds files matching a glob pattern.
func execROFileGlob(rootDir, input string) string {
	var params struct {
		Pattern   string `json:"pattern"`
		Directory string `json:"directory"`
	}
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return fmt.Sprintf("error: invalid input: %v", err)
	}
	if params.Pattern == "" {
		return "error: pattern is required"
	}

	searchDir := rootDir
	if params.Directory != "" {
		resolved, err := resolvePath(rootDir, params.Directory)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		searchDir = resolved
	}

	absPattern := filepath.Join(searchDir, params.Pattern)
	matches, err := filepath.Glob(absPattern)
	if err != nil {
		return fmt.Sprintf("error: invalid glob pattern: %v", err)
	}

	if len(matches) == 0 {
		return "no files found"
	}

	root := filepath.Clean(rootDir)
	var results []string
	for _, m := range matches {
		rel, err := filepath.Rel(root, m)
		if err != nil {
			rel = m
		}
		results = append(results, rel)
		if len(results) >= maxEntries {
			break
		}
	}

	return strings.Join(results, "\n")
}
