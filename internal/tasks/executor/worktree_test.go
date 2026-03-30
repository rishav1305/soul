package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestRepo creates a temporary git repository for testing.
// It runs git init -b main, configures user, writes README.md, and commits it.
func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	if err := gitCmd(dir, "init", "-b", "main"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := gitCmd(dir, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("git config email: %v", err)
	}
	if err := gitCmd(dir, "config", "user.name", "Test User"); err != nil {
		t.Fatalf("git config name: %v", err)
	}

	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}

	if err := gitCmd(dir, "add", "README.md"); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := gitCmd(dir, "commit", "-m", "init"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	return dir
}

func TestWorktreeCreateAndCleanup(t *testing.T) {
	repo := setupTestRepo(t)

	wt, err := CreateWorktree(repo, 42)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(wt.Dir); os.IsNotExist(err) {
		t.Errorf("worktree dir %s does not exist", wt.Dir)
	}

	// Verify branch name
	if wt.Branch != "task/42" {
		t.Errorf("Branch = %q, want %q", wt.Branch, "task/42")
	}

	// Verify README.md is present in the worktree
	readme := filepath.Join(wt.Dir, "README.md")
	if _, err := os.Stat(readme); os.IsNotExist(err) {
		t.Errorf("README.md not present in worktree at %s", readme)
	}

	// Cleanup
	if err := wt.Cleanup(); err != nil {
		t.Errorf("Cleanup: %v", err)
	}

	// Verify directory was removed
	if _, err := os.Stat(wt.Dir); !os.IsNotExist(err) {
		t.Errorf("worktree dir %s still exists after cleanup", wt.Dir)
	}
}

func TestWorktreeCommit(t *testing.T) {
	repo := setupTestRepo(t)

	wt, err := CreateWorktree(repo, 1)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	t.Cleanup(func() { _ = wt.Cleanup() })

	// Write a new file to the worktree
	newFile := filepath.Join(wt.Dir, "newfile.txt")
	if err := os.WriteFile(newFile, []byte("hello\n"), 0644); err != nil {
		t.Fatalf("write newfile.txt: %v", err)
	}

	hash, err := wt.Commit("add newfile.txt")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if hash == "" {
		t.Error("Commit returned empty hash, expected non-empty")
	}
}

func TestWorktreeCommitNoChanges(t *testing.T) {
	repo := setupTestRepo(t)

	wt, err := CreateWorktree(repo, 2)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	t.Cleanup(func() { _ = wt.Cleanup() })

	hash, err := wt.Commit("should be a no-op")
	if err != nil {
		t.Errorf("Commit with no changes returned error: %v", err)
	}
	if hash != "" {
		t.Errorf("Commit with no changes returned hash %q, want empty string", hash)
	}
}

func TestWorktreeHasChanges(t *testing.T) {
	repo := setupTestRepo(t)

	wt, err := CreateWorktree(repo, 3)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	t.Cleanup(func() { _ = wt.Cleanup() })

	// Initially no changes
	if wt.HasChanges() {
		t.Error("HasChanges() = true on fresh worktree, want false")
	}

	// Write a file to create changes
	newFile := filepath.Join(wt.Dir, "change.txt")
	if err := os.WriteFile(newFile, []byte("change\n"), 0644); err != nil {
		t.Fatalf("write change.txt: %v", err)
	}

	if !wt.HasChanges() {
		t.Error("HasChanges() = false after writing file, want true")
	}
}

// --- Additional worktree coverage tests ---

func TestWorktreeCreateStale(t *testing.T) {
	repo := setupTestRepo(t)

	// Create a stale directory at the expected worktree path.
	staleDir := filepath.Join(repo, ".worktrees", "task-99")
	if err := os.MkdirAll(staleDir, 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(staleDir, "stale.txt"), []byte("old"), 0644)

	wt, err := CreateWorktree(repo, 99)
	if err != nil {
		t.Fatalf("CreateWorktree with stale dir: %v", err)
	}
	t.Cleanup(func() { _ = wt.Cleanup() })

	// Verify the stale file is gone and fresh worktree has HEAD content.
	if _, err := os.Stat(filepath.Join(wt.Dir, "stale.txt")); !os.IsNotExist(err) {
		t.Error("stale.txt should not exist in fresh worktree")
	}
	if _, err := os.Stat(filepath.Join(wt.Dir, "README.md")); os.IsNotExist(err) {
		t.Error("README.md should exist in worktree from HEAD")
	}
}

func TestGitCmd_Success(t *testing.T) {
	repo := setupTestRepo(t)

	err := gitCmd(repo, "status")
	if err != nil {
		t.Errorf("gitCmd status: %v", err)
	}
}

func TestGitCmd_Failure(t *testing.T) {
	dir := t.TempDir() // not a git repo

	err := gitCmd(dir, "status")
	if err == nil {
		t.Fatal("expected error for non-repo dir")
	}
	if !strings.Contains(err.Error(), "git") {
		t.Errorf("error = %q, should mention git", err.Error())
	}
}

func TestWorktreeCleanup_NonexistentDir(t *testing.T) {
	wt := &Worktree{
		Dir:    filepath.Join(t.TempDir(), "nonexistent"),
		Branch: "task/404",
		repo:   t.TempDir(),
	}

	// Cleanup on a non-existent dir should not panic.
	err := wt.Cleanup()
	_ = err
}

func TestWorktreeMultipleCommits(t *testing.T) {
	repo := setupTestRepo(t)

	wt, err := CreateWorktree(repo, 10)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = wt.Cleanup() })

	// First commit.
	os.WriteFile(filepath.Join(wt.Dir, "file1.txt"), []byte("one"), 0644)
	hash1, err := wt.Commit("add file1")
	if err != nil {
		t.Fatalf("first commit: %v", err)
	}
	if hash1 == "" {
		t.Error("first commit returned empty hash")
	}

	// Second commit.
	os.WriteFile(filepath.Join(wt.Dir, "file2.txt"), []byte("two"), 0644)
	hash2, err := wt.Commit("add file2")
	if err != nil {
		t.Fatalf("second commit: %v", err)
	}
	if hash2 == "" {
		t.Error("second commit returned empty hash")
	}
	if hash1 == hash2 {
		t.Error("expected different hashes for two commits")
	}
}
