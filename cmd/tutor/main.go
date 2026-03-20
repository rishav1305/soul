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

	"github.com/rishav1305/soul-v2/internal/chat/metrics"
	"github.com/rishav1305/soul-v2/internal/chat/stream"
	"github.com/rishav1305/soul-v2/internal/tutor/eval"
	"github.com/rishav1305/soul-v2/internal/tutor/questions"
	"github.com/rishav1305/soul-v2/internal/tutor/server"
	"github.com/rishav1305/soul-v2/internal/tutor/store"
	"github.com/rishav1305/soul-v2/pkg/auth"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: soul-tutor <command>")
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

	// Content directory for tutor markdown files.
	contentDir := filepath.Join(dataDir, "tutor", "content")
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		log.Fatalf("create content dir: %v", err)
	}

	// Open tutor store.
	dbPath := filepath.Join(dataDir, "tutor.db")
	tutorStore, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open tutor store: %v", err)
	}
	defer tutorStore.Close()

	// Claude API client for semantic evaluation.
	credPath := filepath.Join(os.Getenv("HOME"), ".claude", ".credentials.json")
	authSource := auth.NewOAuthTokenSource(credPath, nil)
	streamClient := stream.NewClient(authSource)
	evaluator := eval.New(streamClient)
	log.Println("tutor: semantic evaluation initialized")

	// Metrics logger.
	metricsLogger, err := metrics.NewEventLogger(dataDir, "tutor")
	if err != nil {
		log.Fatalf("create metrics logger: %v", err)
	}
	defer metricsLogger.Close()

	// Seed embedded questions on boot (idempotent).
	loadStats, err := questions.Load(tutorStore)
	if err != nil {
		log.Printf("tutor: question loading error: %v", err)
	} else {
		log.Printf("tutor: questions loaded — %d topics, %d questions", loadStats.TopicsCreated, loadStats.QuestionsCreated)
	}

	// Server options.
	host := os.Getenv("SOUL_TUTOR_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := 3006
	if p := os.Getenv("SOUL_TUTOR_PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}

	opts := []server.Option{
		server.WithStore(tutorStore),
		server.WithHost(host),
		server.WithPort(port),
		server.WithContentDir(contentDir),
		server.WithMetrics(metricsLogger),
		server.WithEvaluator(evaluator),
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
