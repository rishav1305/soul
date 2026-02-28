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

	soul "github.com/rishav1305/soul"
	"github.com/rishav1305/soul/internal/ai"
	"github.com/rishav1305/soul/internal/config"
	"github.com/rishav1305/soul/internal/planner"
	"github.com/rishav1305/soul/internal/products"
	"github.com/rishav1305/soul/internal/server"
)

var version = "0.2.0-alpha"

func main() {
	args := os.Args[1:]

	if hasFlag(args, "--version", "-v") {
		fmt.Printf("Soul v%s\n", version)
		return
	}

	if hasFlag(args, "--help", "-h") {
		printHelp()
		return
	}

	if len(args) > 0 && args[0] == "serve" {
		runServe(args[1:])
		return
	}

	printHelp()
}

func runServe(args []string) {
	cfg := config.FromEnv()

	// Parse --port flag from args.
	if v := getFlagValue(args, "--port"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 && p <= 65535 {
			cfg.Port = p
		}
	}

	// Parse --dev flag.
	if hasFlag(args, "--dev") {
		cfg.DevMode = true
	}

	// Create the product registry and manager.
	registry := products.NewRegistry()
	manager := products.NewManager(registry, cfg.DataDir)

	// Create AI client: try API key first, then Claude Max OAuth credentials.
	var aiClient *ai.Client
	if cfg.APIKey != "" {
		aiClient = ai.NewClient(cfg.APIKey, cfg.Model)
		fmt.Println("  AI client configured (API key)")
	} else if oauthCreds := ai.LoadOAuthCredentials(); oauthCreds != nil {
		tokenSource := ai.NewOAuthTokenSource(oauthCreds)
		aiClient = ai.NewOAuthClient(tokenSource, cfg.Model)
		fmt.Println("  AI client configured (Claude Max OAuth)")
	} else {
		fmt.Println("  AI client not configured (set ANTHROPIC_API_KEY or log in with Claude CLI)")
	}

	// Open planner store.
	var plannerStore *planner.Store
	plannerDB := filepath.Join(cfg.DataDir, "planner.db")
	plannerStore, err := planner.OpenStore(plannerDB)
	if err != nil {
		log.Printf("WARNING: failed to open planner store: %v", err)
	} else {
		fmt.Println("  Planner store opened")
	}

	// Determine compliance binary path.
	complianceBin := getFlagValue(args, "--compliance-bin")
	if complianceBin == "" {
		complianceBin = os.Getenv("SOUL_COMPLIANCE_BIN")
	}

	// Start compliance product if binary is available.
	if complianceBin != "" {
		if _, err := os.Stat(complianceBin); err == nil {
			ctx := context.Background()
			fmt.Printf("  Starting compliance product: %s\n", complianceBin)
			if err := manager.StartProduct(ctx, "compliance", complianceBin); err != nil {
				log.Printf("WARNING: failed to start compliance product: %v", err)
			} else {
				fmt.Println("  Compliance product started")
			}
		} else {
			log.Printf("WARNING: compliance binary not found at %s", complianceBin)
		}
	}

	// Create server with embedded SPA (falls back to placeholder if web/dist not built).
	srv := server.NewWithWebFS(cfg, manager, aiClient, plannerStore, soul.WebDist)

	// Start dev server on port+1.
	go srv.StartDevServer(cfg.Port + 1)

	// Handle graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		manager.StopAll()
		os.Exit(0)
	}()

	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		manager.StopAll()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Printf("◆ Soul v%s\n\n", version)
	fmt.Println("Usage:")
	fmt.Println("  soul serve [--port PORT] [--dev] [--compliance-bin PATH]   Start web UI")
	fmt.Println("  soul --version                                              Show version")
	fmt.Println("  soul --help                                                 Show this help")
	fmt.Println()
	fmt.Println("Authentication (in priority order):")
	fmt.Println("  ANTHROPIC_API_KEY      Claude API key")
	fmt.Println("  ~/.claude/.credentials.json   Claude Max/Pro OAuth (auto-detected)")
	fmt.Println()
	fmt.Println("Environment:")
	fmt.Println("  SOUL_COMPLIANCE_BIN    Path to compliance product binary")
	fmt.Println("  SOUL_PORT              Server port (default: 3000)")
	fmt.Println("  SOUL_HOST              Server host (default: 127.0.0.1)")
	fmt.Println("  SOUL_DATA_DIR          Data directory (default: ~/.soul)")
	fmt.Println("  SOUL_MODEL             Claude model (default: claude-sonnet-4-6)")
}

func hasFlag(args []string, flags ...string) bool {
	for _, arg := range args {
		for _, flag := range flags {
			if arg == flag {
				return true
			}
		}
	}
	return false
}

func getFlagValue(args []string, flag string) string {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
