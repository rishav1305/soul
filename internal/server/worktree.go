package server

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrNothingToCommit is returned when CommitInWorktree finds no changes to commit.
var ErrNothingToCommit = errors.New("nothing to commit")

// WorktreeManager manages git worktrees for isolated task execution.
type WorktreeManager struct {
	repoRoot string // main repo root (e.g., /home/rishav/soul)
}

// NewWorktreeManager creates a new WorktreeManager.
func NewWorktreeManager(repoRoot string) *WorktreeManager {
	return &WorktreeManager{repoRoot: repoRoot}
}

// slugify converts a task title to a URL-safe slug.
func slugify(title string) string {
	re := regexp.MustCompile(`[^a-z0-9]+`)
	slug := re.ReplaceAllString(strings.ToLower(title), "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 40 {
		slug = slug[:40]
	}
	return slug
}

// branchName returns the git branch name for a task.
func (wm *WorktreeManager) branchName(taskID int64, title string) string {
	return fmt.Sprintf("task/%d-%s", taskID, slugify(title))
}

// worktreePath returns the filesystem path for a task's worktree.
func (wm *WorktreeManager) worktreePath(taskID int64) string {
	return filepath.Join(wm.repoRoot, ".worktrees", fmt.Sprintf("task-%d", taskID))
}

// EnsureSetup creates .worktrees/ dir and ensures dev branch exists.
func (wm *WorktreeManager) EnsureSetup() error {
	// Create .worktrees directory.
	wtDir := filepath.Join(wm.repoRoot, ".worktrees")
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		return fmt.Errorf("create .worktrees: %w", err)
	}

	// Add to .gitignore if not already there.
	gitignorePath := filepath.Join(wm.repoRoot, ".gitignore")
	data, _ := os.ReadFile(gitignorePath)
	if !strings.Contains(string(data), ".worktrees") {
		f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("open .gitignore: %w", err)
		}
		defer f.Close()
		if _, err := f.WriteString("\n.worktrees/\n"); err != nil {
			return fmt.Errorf("write .gitignore: %w", err)
		}
		log.Printf("[worktree] added .worktrees/ to .gitignore")
	}

	// Ensure dev branch exists (create from master if not).
	cmd := exec.Command("git", "rev-parse", "--verify", "dev")
	cmd.Dir = wm.repoRoot
	if err := cmd.Run(); err != nil {
		// dev branch doesn't exist — create it from master.
		cmd = exec.Command("git", "branch", "dev", "master")
		cmd.Dir = wm.repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("create dev branch: %s — %w", out, err)
		}
		log.Printf("[worktree] created dev branch from master")
	}

	return nil
}

// Create creates a worktree + branch for a task. Returns the worktree path.
func (wm *WorktreeManager) Create(taskID int64, title string) (string, error) {
	branch := wm.branchName(taskID, title)
	wtPath := wm.worktreePath(taskID)

	// Remove stale worktree if it exists.
	if _, err := os.Stat(wtPath); err == nil {
		log.Printf("[worktree] removing stale worktree at %s", wtPath)
		cmd := exec.Command("git", "worktree", "remove", "--force", wtPath)
		cmd.Dir = wm.repoRoot
		cmd.CombinedOutput()
	}

	// Delete stale branch if it exists.
	cmd := exec.Command("git", "branch", "-D", branch)
	cmd.Dir = wm.repoRoot
	cmd.CombinedOutput() // ignore error if branch doesn't exist

	// Create worktree from dev branch.
	cmd = exec.Command("git", "worktree", "add", wtPath, "-b", branch, "dev")
	cmd.Dir = wm.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git worktree add: %s — %w", out, err)
	}

	// Symlink web/node_modules so vite/npm tools work in the worktree.
	wtNodeModules := filepath.Join(wtPath, "web", "node_modules")
	mainNodeModules := filepath.Join(wm.repoRoot, "web", "node_modules")
	if _, err := os.Lstat(wtNodeModules); os.IsNotExist(err) {
		if err := os.Symlink(mainNodeModules, wtNodeModules); err != nil {
			log.Printf("[worktree] failed to symlink node_modules: %v", err)
		}
	}

	log.Printf("[worktree] created %s (branch: %s)", wtPath, branch)
	return wtPath, nil
}

// ProjectRoot returns the worktree path for a task (may not exist yet).
func (wm *WorktreeManager) ProjectRoot(taskID int64) string {
	return wm.worktreePath(taskID)
}

// Cleanup removes a task's worktree and deletes its branch.
func (wm *WorktreeManager) Cleanup(taskID int64, title string) error {
	wtPath := wm.worktreePath(taskID)
	branch := wm.branchName(taskID, title)

	// Remove worktree.
	cmd := exec.Command("git", "worktree", "remove", "--force", wtPath)
	cmd.Dir = wm.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[worktree] remove warning: %s — %v", out, err)
	}

	// Delete branch.
	cmd = exec.Command("git", "branch", "-D", branch)
	cmd.Dir = wm.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[worktree] branch delete warning: %s — %v", out, err)
	}

	// Prune stale worktree entries.
	cmd = exec.Command("git", "worktree", "prune")
	cmd.Dir = wm.repoRoot
	cmd.CombinedOutput()

	log.Printf("[worktree] cleaned up task %d", taskID)
	return nil
}

// CommitInWorktree stages all changes in a worktree and commits them.
func (wm *WorktreeManager) CommitInWorktree(taskID int64, title string) error {
	wtPath := wm.worktreePath(taskID)

	// Stage all changes.
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = wtPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %s — %w", out, err)
	}

	// Check if there's anything to commit.
	cmd = exec.Command("git", "diff", "--cached", "--quiet")
	cmd.Dir = wtPath
	if err := cmd.Run(); err == nil {
		log.Printf("[worktree] task %d: nothing to commit", taskID)
		return ErrNothingToCommit
	}

	// Commit.
	msg := fmt.Sprintf("task #%d: %s", taskID, title)
	cmd = exec.Command("git", "commit", "-m", msg)
	cmd.Dir = wtPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %s — %w", out, err)
	}

	log.Printf("[worktree] committed in task %d worktree", taskID)
	return nil
}

// MergeToDev merges a task branch into the dev branch.
// Uses the dev-server worktree (which has dev checked out) to avoid
// conflicts with the main repo's checked-out branch.
func (wm *WorktreeManager) MergeToDev(taskID int64, title string) error {
	branch := wm.branchName(taskID, title)
	devWT := filepath.Join(wm.repoRoot, ".worktrees", "dev-server")

	// Check if dev-server worktree exists; if not, merge from main repo.
	if _, err := os.Stat(devWT); os.IsNotExist(err) {
		// No dev-server worktree — fall back to checkout in main repo.
		cmd := exec.Command("git", "checkout", "dev")
		cmd.Dir = wm.repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("checkout dev: %s — %w", out, err)
		}
		cmd = exec.Command("git", "merge", branch, "--no-ff",
			"-m", fmt.Sprintf("merge: task #%d — %s", taskID, title))
		cmd.Dir = wm.repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			back := exec.Command("git", "checkout", "master")
			back.Dir = wm.repoRoot
			back.Run()
			return fmt.Errorf("merge to dev: %s — %w", out, err)
		}
		cmd = exec.Command("git", "checkout", "master")
		cmd.Dir = wm.repoRoot
		cmd.CombinedOutput()
	} else {
		// Merge inside the dev-server worktree (which already has dev checked out).
		cmd := exec.Command("git", "merge", branch, "--no-ff",
			"-m", fmt.Sprintf("merge: task #%d — %s", taskID, title))
		cmd.Dir = devWT
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("merge to dev: %s — %w", out, err)
		}
	}

	log.Printf("[worktree] merged task %d to dev", taskID)
	return nil
}

// MergeToMaster merges a task branch into the master branch.
func (wm *WorktreeManager) MergeToMaster(taskID int64, title string) error {
	branch := wm.branchName(taskID, title)

	cmd := exec.Command("git", "merge", branch, "--no-ff",
		"-m", fmt.Sprintf("merge: task #%d — %s", taskID, title))
	cmd.Dir = wm.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("merge to master: %s — %w", out, err)
	}

	log.Printf("[worktree] merged task %d to master", taskID)
	return nil
}

// RebuildFrontend runs vite build in the given directory.
// It ensures node_modules is symlinked from the main project root before building.
func (wm *WorktreeManager) RebuildFrontend(dir string) error {
	webDir := filepath.Join(dir, "web")

	// Ensure node_modules symlink exists (worktrees don't have their own).
	nodeModules := filepath.Join(webDir, "node_modules")
	mainNodeModules := filepath.Join(wm.repoRoot, "web", "node_modules")
	if _, err := os.Lstat(nodeModules); os.IsNotExist(err) {
		if err := os.Symlink(mainNodeModules, nodeModules); err != nil {
			return fmt.Errorf("symlink node_modules: %w", err)
		}
	}

	cmd := exec.Command("npx", "vite", "build")
	cmd.Dir = webDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("vite build: %s — %w", out, err)
	}
	log.Printf("[worktree] frontend rebuilt in %s", dir)
	return nil
}
