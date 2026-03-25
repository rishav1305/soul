package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/rishav1305/soul/internal/chat/metrics"
	"github.com/rishav1305/soul/internal/chat/stream"
	"github.com/rishav1305/soul/internal/tasks/executor"
	"github.com/rishav1305/soul/internal/tasks/server"
	"github.com/rishav1305/soul/internal/tasks/store"
	"github.com/rishav1305/soul/pkg/auth"
	"github.com/rishav1305/soul/pkg/events"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: soul-tasks <command>")
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

	// Open task store.
	dbPath := filepath.Join(dataDir, "tasks.db")
	taskStore, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open task store: %v", err)
	}
	defer taskStore.Close()

	// Metrics logger.
	metricsLogger, err := metrics.NewEventLogger(dataDir, "tasks")
	if err != nil {
		log.Fatalf("create metrics logger: %v", err)
	}
	defer metricsLogger.Close()

	// Recover interrupted tasks on startup.
	recoverInterruptedTasks(taskStore)

	// Load OAuth credentials for Claude API access.
	credPath := filepath.Join(os.Getenv("HOME"), ".claude", ".credentials.json")
	authSource := auth.NewOAuthTokenSource(credPath, events.NopLogger{})
	if _, err := authSource.Load(); err != nil {
		log.Printf("auth: %v (autonomous execution will be unavailable)", err)
	}

	claudeClient := stream.NewClient(authSource,
		stream.WithBetaHeader("prompt-caching-2024-07-31,"+auth.OAuthBetaHeader),
	)

	// Project root for worktree creation.
	repoDir := os.Getenv("SOUL_V2_REPO_DIR")
	if repoDir == "" {
		repoDir, _ = os.Getwd()
	}

	// Create executor.
	exec := executor.New(executor.Config{
		Store:       taskStore,
		MaxParallel: 3,
		RepoDir:     repoDir,
		Client:      claudeClient,
	})

	// Server options.
	host := os.Getenv("SOUL_TASKS_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := 3004
	if p := os.Getenv("SOUL_TASKS_PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}

	opts := []server.Option{
		server.WithStore(taskStore),
		server.WithLogger(events.NopLogger{}),
		server.WithHost(host),
		server.WithPort(port),
		server.WithExecutor(exec),
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

// recoverInterruptedTasks marks any active tasks as blocked on startup.
func recoverInterruptedTasks(s *store.Store) {
	tasks, err := s.List("active", "")
	if err != nil {
		log.Printf("warn: could not scan for interrupted tasks: %v", err)
		return
	}
	for _, t := range tasks {
		log.Printf("recovering interrupted task %d: %s", t.ID, t.Title)
		s.Update(t.ID, map[string]interface{}{
			"stage": "blocked",
		})
		s.AddActivity(t.ID, "task.blocked", map[string]interface{}{
			"reason": "server restart — execution interrupted",
		})
	}
}
