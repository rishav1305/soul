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

	"github.com/rishav1305/soul-v2/internal/chat/stream"
	"github.com/rishav1305/soul-v2/internal/sentinel/engine"
	"github.com/rishav1305/soul-v2/internal/sentinel/server"
	"github.com/rishav1305/soul-v2/internal/sentinel/store"
	"github.com/rishav1305/soul-v2/pkg/auth"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: soul-sentinel <command>")
		fmt.Println("commands: serve")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		runServe()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runServe() {
	// Data directory.
	dataDir := os.Getenv("SOUL_V2_DATA_DIR")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("get home directory: %v", err)
		}
		dataDir = filepath.Join(home, ".soul-v2")
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	// Open sentinel store.
	dbPath := filepath.Join(dataDir, "sentinel.db")
	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open sentinel store: %v", err)
	}
	defer st.Close()

	// Seed challenges from embedded data.
	challengesPath := findChallengesJSON()
	if challengesPath != "" {
		data, err := os.ReadFile(challengesPath)
		if err != nil {
			log.Printf("read challenges.json: %v (skipping seed)", err)
		} else {
			if err := st.SeedChallenges(data); err != nil {
				log.Fatalf("seed challenges: %v", err)
			}
			log.Printf("seeded challenges from %s", challengesPath)
		}
	} else {
		log.Printf("challenges.json not found — skipping seed")
	}

	// Load OAuth credentials for Claude API.
	credPath := filepath.Join(os.Getenv("HOME"), ".claude", ".credentials.json")
	authSource := auth.NewOAuthTokenSource(credPath, nil)
	if _, err := authSource.Load(); err != nil {
		log.Printf("auth: %v (Claude API calls will fail)", err)
	}

	// Create stream client.
	streamClient := stream.NewClient(authSource,
		stream.WithBetaHeader("prompt-caching-2024-07-31,"+auth.OAuthBetaHeader),
	)

	// Create engine.
	eng := engine.New(st, streamClient)

	// Server config.
	host := os.Getenv("SOUL_SENTINEL_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := 3022
	if p := os.Getenv("SOUL_SENTINEL_PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}

	srv := server.New(
		server.WithHost(host),
		server.WithPort(port),
		server.WithEngine(eng),
		server.WithStore(st),
	)

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

// findChallengesJSON searches common locations for the challenges data file.
func findChallengesJSON() string {
	candidates := []string{
		"internal/sentinel/challenges/challenges.json",
	}

	// Try relative to working directory.
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	// Try relative to executable.
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		for _, c := range candidates {
			p := filepath.Join(dir, c)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}

	return ""
}
