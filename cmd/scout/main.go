package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/rishav1305/soul-v2/internal/chat/stream"
	"github.com/rishav1305/soul-v2/internal/scout/ai"
	"github.com/rishav1305/soul-v2/internal/scout/profiledb"
	"github.com/rishav1305/soul-v2/internal/scout/server"
	"github.com/rishav1305/soul-v2/internal/scout/store"
	"github.com/rishav1305/soul-v2/internal/scout/sweep"
	"github.com/rishav1305/soul-v2/pkg/auth"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: soul-scout <command>")
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

	// Server host/port.
	host := os.Getenv("SOUL_SCOUT_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := 3020
	if p := os.Getenv("SOUL_SCOUT_PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}

	// 1. Open store.
	st, err := store.Open(filepath.Join(dataDir, "scout.db"))
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// 2. Connect profiledb (optional).
	var pdb *profiledb.Client
	if pgURL := os.Getenv("SOUL_SCOUT_PG_URL"); pgURL != "" {
		pdb, err = profiledb.New(pgURL)
		if err != nil {
			log.Printf("scout: profiledb unavailable: %v", err)
		} else {
			defer pdb.Close()
			log.Println("scout: profiledb connected")
		}
	}

	// 3. Create stream client for AI tools.
	credPath := filepath.Join(os.Getenv("HOME"), ".claude", ".credentials.json")
	authSource := auth.NewOAuthTokenSource(credPath, nil)
	streamClient := stream.NewClient(authSource)

	// 4. Create AI service (pdb may be nil).
	aiSvc := ai.New(st, pdb, streamClient, dataDir)

	// 5. Load sweep config.
	configPath := filepath.Join(dataDir, "scout", "sweep-config.json")
	sweepCfg, err := sweep.LoadConfig(configPath)
	if err != nil {
		log.Printf("scout: sweep config: %v — using defaults", err)
		sweepCfg = sweep.DefaultConfig()
	}

	// 6. Build server options.
	opts := []server.Option{
		server.WithHost(host),
		server.WithPort(port),
		server.WithDataDir(dataDir),
		server.WithStore(st),
		server.WithAIService(aiSvc),
		server.WithConfigPath(configPath),
	}
	if pdb != nil {
		opts = append(opts, server.WithProfileDB(pdb))
	} else if pgURL := os.Getenv("SOUL_SCOUT_PG_URL"); pgURL != "" {
		// Fallback: let server create its own profiledb connection
		opts = append(opts, server.WithPgURL(pgURL))
	}

	// 7. Create TheirStack client and scheduler (if API key present).
	if theirStackKey := os.Getenv("SOUL_SCOUT_THEIRSTACK_KEY"); theirStackKey != "" {
		tsClient := sweep.NewTheirStackClient(theirStackKey, http.DefaultClient)
		scheduler := sweep.NewScheduler(sweepCfg, st, aiSvc, tsClient)
		scheduler.Start()
		defer scheduler.Stop()
		opts = append(opts, server.WithScheduler(scheduler))
		log.Println("scout: TheirStack sweep scheduler started")
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
