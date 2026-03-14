package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateDBPath_MigratesOldFile(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "sessions.db")
	newPath := filepath.Join(dir, "chat.db")

	if err := os.WriteFile(oldPath, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}

	migrated, err := MigrateDBPath(dir)
	if err != nil {
		t.Fatalf("MigrateDBPath() error: %v", err)
	}
	if !migrated {
		t.Error("MigrateDBPath() = false, want true")
	}

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("old file should not exist after migration")
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Error("new file should exist after migration")
	}
}

func TestMigrateDBPath_MigratesWALAndSHM(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "sessions.db")

	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.WriteFile(oldPath+suffix, []byte("data"+suffix), 0600); err != nil {
			t.Fatal(err)
		}
	}

	migrated, err := MigrateDBPath(dir)
	if err != nil {
		t.Fatalf("MigrateDBPath() error: %v", err)
	}
	if !migrated {
		t.Error("MigrateDBPath() = false, want true")
	}

	newPath := filepath.Join(dir, "chat.db")
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if _, err := os.Stat(newPath + suffix); err != nil {
			t.Errorf("chat.db%s should exist", suffix)
		}
		if _, err := os.Stat(oldPath + suffix); !os.IsNotExist(err) {
			t.Errorf("sessions.db%s should not exist", suffix)
		}
	}
}

func TestMigrateDBPath_SkipsIfNewExists(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "sessions.db")
	newPath := filepath.Join(dir, "chat.db")

	if err := os.WriteFile(oldPath, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}

	migrated, err := MigrateDBPath(dir)
	if err != nil {
		t.Fatalf("MigrateDBPath() error: %v", err)
	}
	if migrated {
		t.Error("MigrateDBPath() = true, want false when chat.db exists")
	}

	data, _ := os.ReadFile(newPath)
	if string(data) != "new" {
		t.Error("chat.db should be unchanged")
	}
	oldData, _ := os.ReadFile(oldPath)
	if string(oldData) != "old" {
		t.Error("sessions.db should be preserved")
	}
}

func TestMigrateDBPath_FreshInstall(t *testing.T) {
	dir := t.TempDir()

	migrated, err := MigrateDBPath(dir)
	if err != nil {
		t.Fatalf("MigrateDBPath() error: %v", err)
	}
	if migrated {
		t.Error("MigrateDBPath() = true, want false on fresh install")
	}
}
