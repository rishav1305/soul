package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/rishav1305/soul-v2/internal/auth"
	"github.com/rishav1305/soul-v2/internal/metrics"
	"github.com/rishav1305/soul-v2/internal/server"
	"github.com/rishav1305/soul-v2/internal/session"
	"github.com/rishav1305/soul-v2/internal/stream"
	"github.com/rishav1305/soul-v2/internal/ws"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: soul <command>")
		fmt.Println("commands: serve, metrics")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		runServe()
	case "metrics":
		if len(os.Args) < 3 {
			fmt.Println("usage: soul metrics <subcommand>")
			fmt.Println("subcommands: tail, log")
			os.Exit(1)
		}
		runMetrics(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runServe() {
	dataDir := os.Getenv("SOUL_V2_DATA_DIR")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("get home directory: %v", err)
		}
		dataDir = filepath.Join(home, ".soul-v2")
	}

	logger, err := metrics.NewEventLogger(dataDir)
	if err != nil {
		log.Fatalf("create event logger: %v", err)
	}
	defer logger.Close()

	// Load OAuth credentials (best effort — server works without auth).
	credPath := filepath.Join(os.Getenv("HOME"), ".claude", ".credentials.json")
	authSource := auth.NewOAuthTokenSource(credPath, logger)
	if _, err := authSource.Load(); err != nil {
		log.Printf("auth: %v (server will report auth as missing)", err)
	}

	// Open session store.
	dbPath := filepath.Join(dataDir, "sessions.db")
	store, err := session.Open(dbPath)
	if err != nil {
		log.Fatalf("open session store: %v", err)
	}
	defer store.Close()

	// Start system health sampler (30s interval).
	sampler := metrics.NewSampler(logger, 30*time.Second)
	sampler.Start()
	defer sampler.Stop()

	// Create WebSocket hub, stream client, and message handler.
	hub := ws.NewHub(
		ws.WithMetricsLogger(logger),
		ws.WithSessionStore(store),
	)
	streamClient := stream.NewClient(authSource)
	handler := ws.NewMessageHandler(hub, store, logger, ws.WithStreamClient(streamClient))
	hub.SetHandler(handler)

	hubCtx, hubCancel := context.WithCancel(context.Background())
	go hub.Run(hubCtx)
	defer hubCancel()

	srv := server.New(
		server.WithMetrics(logger),
		server.WithAuth(authSource),
		server.WithSessionStore(store),
		server.WithHub(hub),
		server.WithStaticDir("web/dist"),
	)

	// Handle SIGINT/SIGTERM for graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("received %s, shutting down...", sig)
		if err := srv.Shutdown(context.Background()); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	if err := srv.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
