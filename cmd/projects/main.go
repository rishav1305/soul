package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/rishav1305/soul/internal/chat/metrics"
	"github.com/rishav1305/soul/internal/projects/content"
	"github.com/rishav1305/soul/internal/projects/server"
	"github.com/rishav1305/soul/internal/projects/store"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: soul-projects <command>")
		fmt.Println("commands: serve")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		runServe()
	default:
		fmt.Printf("unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runServe() {
	// Data directory.
	dataDir := os.Getenv("SOUL_V2_DATA_DIR")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".soul-v2")
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	// Content directory for project guide markdown files.
	contentDir := filepath.Join(dataDir, "projects", "content")
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		log.Fatalf("create content dir: %v", err)
	}

	// Open projects store.
	dbPath := filepath.Join(dataDir, "projects.db")
	projectsStore, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open projects store: %v", err)
	}
	defer projectsStore.Close()

	// Metrics logger.
	metricsLogger, err := metrics.NewEventLogger(dataDir, "projects")
	if err != nil {
		log.Fatalf("create metrics logger: %v", err)
	}
	defer metricsLogger.Close()

	// Seed database (idempotent).
	if err := projectsStore.Seed(); err != nil {
		log.Printf("projects: seed error: %v", err)
	}

	// Copy embedded guides to content dir (preserve user edits).
	copyGuides(contentDir)

	// Server options.
	host := os.Getenv("SOUL_PROJECTS_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := os.Getenv("SOUL_PROJECTS_PORT")
	if port == "" {
		port = "3008"
	}

	opts := []server.Option{
		server.WithStore(projectsStore),
		server.WithHost(host),
		server.WithPort(port),
		server.WithContentDir(contentDir),
		server.WithMetrics(metricsLogger),
	}

	srv := server.New(opts...)

	// Graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("received %s, shutting down...", sig)
		srv.Shutdown(context.Background())
	}()

	if err := srv.Start(); err != nil && err.Error() != "http: Server closed" {
		log.Fatalf("server error: %v", err)
	}
}

// copyGuides copies embedded guide files to the content directory.
// Skips files that already exist on disk to preserve user edits.
func copyGuides(contentDir string) {
	entries, err := fs.ReadDir(content.Guides, ".")
	if err != nil {
		log.Printf("projects: read embedded guides: %v", err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		guidePath := filepath.Join(name, "guide.md")
		diskPath := filepath.Join(contentDir, guidePath)

		// Skip if file already exists on disk.
		if _, err := os.Stat(diskPath); err == nil {
			continue
		}

		// Read from embedded FS.
		data, err := content.Guides.ReadFile(guidePath)
		if err != nil {
			continue
		}

		// Create directory and write file.
		if err := os.MkdirAll(filepath.Join(contentDir, name), 0755); err != nil {
			log.Printf("projects: create guide dir %s: %v", name, err)
			continue
		}
		if err := os.WriteFile(diskPath, data, 0644); err != nil {
			log.Printf("projects: write guide %s: %v", guidePath, err)
			continue
		}
		log.Printf("projects: copied guide %s", guidePath)
	}
}
