package executor

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree represents a git worktree created for an isolated task.
type Worktree struct {
	Dir    string
	Branch string
	repo   string
}

// CreateWorktree creates a new git worktree for the given task ID.
// Branch name: task/{taskID}, Dir: {repoDir}/.worktrees/task-{taskID}/
func CreateWorktree(repoDir string, taskID int64) (*Worktree, error) {
	branch := fmt.Sprintf("task/%d", taskID)
	dir := filepath.Join(repoDir, ".worktrees", fmt.Sprintf("task-%d", taskID))

	// Remove stale worktree if directory exists
	if _, err := os.Stat(dir); err == nil {
		// Try git worktree remove first, then fallback to os.RemoveAll
		_ = gitCmd(repoDir, "worktree", "remove", "--force", dir)
		if err := os.RemoveAll(dir); err != nil {
			return nil, fmt.Errorf("remove stale worktree dir %s: %w", dir, err)
		}
	}

	if err := gitCmd(repoDir, "worktree", "add", "-b", branch, dir, "HEAD"); err != nil {
		return nil, fmt.Errorf("git worktree add: %w", err)
	}

	return &Worktree{
		Dir:    dir,
		Branch: branch,
		repo:   repoDir,
	}, nil
}

// HasChanges returns true if the worktree has uncommitted changes.
func (wt *Worktree) HasChanges() bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = wt.Dir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// Commit stages all changes and creates a commit with the given message.
// Returns the commit hash, or ("", nil) if there are no changes.
func (wt *Worktree) Commit(message string) (string, error) {
	if !wt.HasChanges() {
		return "", nil
	}

	if err := gitCmd(wt.Dir, "add", "-A"); err != nil {
		return "", fmt.Errorf("git add: %w", err)
	}

	if err := gitCmd(wt.Dir, "commit", "-m", message); err != nil {
		return "", fmt.Errorf("git commit: %w", err)
	}

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = wt.Dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// Cleanup removes the worktree from both git and the filesystem.
func (wt *Worktree) Cleanup() error {
	err := gitCmd(wt.repo, "worktree", "remove", "--force", wt.Dir)
	// Fallback: remove directory regardless of git error
	if removeErr := os.RemoveAll(wt.Dir); removeErr != nil {
		return fmt.Errorf("remove worktree dir %s: %w", wt.Dir, removeErr)
	}
	return err
}

// gitCmd runs a git command in the given directory and returns an error
// that includes the combined output on failure.
func gitCmd(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w\noutput: %s", strings.Join(args, " "), err, buf.String())
	}
	return nil
}
