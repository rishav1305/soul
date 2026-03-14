package session

import (
	"os"
	"path/filepath"
)

// MigrateDBPath renames sessions.db to chat.db if chat.db does not exist
// and sessions.db does. It also handles WAL and SHM companion files.
// Returns true if migration occurred.
func MigrateDBPath(dataDir string) (bool, error) {
	newPath := filepath.Join(dataDir, "chat.db")
	oldPath := filepath.Join(dataDir, "sessions.db")

	if _, err := os.Stat(newPath); !os.IsNotExist(err) {
		return false, nil // chat.db already exists (or stat error)
	}
	if _, err := os.Stat(oldPath); err != nil {
		return false, nil // sessions.db doesn't exist
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		return false, err
	}

	// Best-effort rename of WAL and SHM companion files.
	os.Rename(oldPath+"-wal", newPath+"-wal")
	os.Rename(oldPath+"-shm", newPath+"-shm")

	return true, nil
}
