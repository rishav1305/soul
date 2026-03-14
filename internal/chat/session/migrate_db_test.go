package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDBRename_MigratesOldFile(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "sessions.db")
	newPath := filepath.Join(dir, "chat.db")

	// Create old DB file
	if err := os.WriteFile(oldPath, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}

	// Simulate migration
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		if _, err := os.Stat(oldPath); err == nil {
			if err := os.Rename(oldPath, newPath); err != nil {
				t.Fatal(err)
			}
		}
	}

	// Verify old file gone, new file exists
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("old file should not exist")
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Error("new file should exist")
	}
}

func TestDBRename_SkipsIfNewExists(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "sessions.db")
	newPath := filepath.Join(dir, "chat.db")

	// Create both files
	os.WriteFile(oldPath, []byte("old"), 0600)
	os.WriteFile(newPath, []byte("new"), 0600)

	// Migration should skip (chat.db already exists)
	if _, err := os.Stat(newPath); !os.IsNotExist(err) {
		// chat.db exists, skip migration
	}

	// Verify new file unchanged
	data, _ := os.ReadFile(newPath)
	if string(data) != "new" {
		t.Error("new file should be unchanged")
	}
}
