package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/rishav1305/soul-v2/pkg/auth"
	"github.com/rishav1305/soul-v2/internal/chat/metrics"
	"github.com/rishav1305/soul-v2/internal/chat/server"
	"github.com/rishav1305/soul-v2/internal/chat/session"
	"github.com/rishav1305/soul-v2/internal/chat/stream"
	"github.com/rishav1305/soul-v2/internal/chat/ws"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: soul-chat <command>")
		fmt.Println("commands: serve, metrics")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		runServe()
	case "metrics":
		if len(os.Args) < 3 {
			fmt.Println("usage: soul-chat metrics <subcommand>")
			fmt.Println("subcommands: status, quality, layers, cost, latency, tail, log")
			os.Exit(1)
		}
		runMetrics(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runServe() {
	// Soft memory limit — triggers more aggressive GC before hitting 256MB.
	debug.SetMemoryLimit(256 * 1024 * 1024)

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

	alertChecker := metrics.NewAlertCheckerWithDefaults(logger)
	logger.SetAlertChecker(alertChecker)

	// Load OAuth credentials (best effort — server works without auth).
	credPath := filepath.Join(os.Getenv("HOME"), ".claude", ".credentials.json")
	authSource := auth.NewOAuthTokenSource(credPath, logger)
	if _, err := authSource.Load(); err != nil {
		log.Printf("auth: %v (server will report auth as missing)", err)
	}

	// Auto-migrate sessions.db → chat.db
	chatDBPath := filepath.Join(dataDir, "chat.db")
	oldDBPath := filepath.Join(dataDir, "sessions.db")
	if _, err := os.Stat(chatDBPath); os.IsNotExist(err) {
		if _, err := os.Stat(oldDBPath); err == nil {
			log.Printf("Migrating %s → %s", oldDBPath, chatDBPath)
			if err := os.Rename(oldDBPath, chatDBPath); err != nil {
				log.Fatalf("Failed to rename database: %v", err)
			}
			// Also rename WAL and SHM files if they exist
			os.Rename(oldDBPath+"-wal", chatDBPath+"-wal")
			os.Rename(oldDBPath+"-shm", chatDBPath+"-shm")
		}
	}

	// Open session store.
	rawStore, err := session.Open(chatDBPath)
	if err != nil {
		log.Fatalf("open session store: %v", err)
	}
	defer rawStore.Close()

	// Wrap with timing instrumentation (100ms slow query threshold).
	store := session.NewTimedStore(rawStore, logger, 100)

	// Start system health sampler (30s interval).
	sampler := metrics.NewSampler(logger, 30*time.Second)
	sampler.Start()
	defer sampler.Stop()

	// Create WebSocket hub, stream client, and message handler.
	hub := ws.NewHub(
		ws.WithMetricsLogger(logger),
		ws.WithSessionStore(store),
	)
	streamClient := stream.NewClient(authSource,
		stream.WithBetaHeader("prompt-caching-2024-07-31,"+auth.OAuthBetaHeader),
	)
	handler := ws.NewMessageHandler(hub, store, logger, ws.WithStreamClient(streamClient))
	hub.SetHandler(handler)

	hubCtx, hubCancel := context.WithCancel(context.Background())
	go hub.Run(hubCtx)
	defer hubCancel()

	serverOpts := []server.Option{
		server.WithMetrics(logger),
		server.WithAuth(authSource),
		server.WithSessionStore(store),
		server.WithHub(hub),
		server.WithStaticDir("web/dist"),
	}

	// Enable TLS if cert and key are configured.
	tlsCert := os.Getenv("SOUL_V2_TLS_CERT")
	tlsKey := os.Getenv("SOUL_V2_TLS_KEY")
	if tlsCert == "" {
		// Default: check data dir for tls/server.crt.
		defaultCert := filepath.Join(dataDir, "tls", "server.crt")
		defaultKey := filepath.Join(dataDir, "tls", "server.key")
		if _, err := os.Stat(defaultCert); err == nil {
			tlsCert = defaultCert
			tlsKey = defaultKey
		}
	}
	if tlsCert != "" && tlsKey != "" {
		serverOpts = append(serverOpts, server.WithTLS(tlsCert, tlsKey))
		log.Printf("TLS enabled: cert=%s", tlsCert)
	}

	srv := server.New(serverOpts...)

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
