package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
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

	// Start products from config file + env var / CLI flag overrides.
	productConfigs := config.LoadProducts(cfg.DataDir)

	// Backwards compat: if no config file, check legacy env vars / flags.
	if len(productConfigs) == 0 {
		// Legacy compliance
		compBin := getFlagValue(args, "--compliance-bin")
		if compBin == "" {
			compBin = os.Getenv("SOUL_COMPLIANCE_BIN")
		}
		if compBin != "" {
			productConfigs = append(productConfigs, config.ProductConfig{
				Name: "compliance", Binary: compBin, Label: "Compliance", Color: "validation",
			})
		}
		// Legacy scout
		scoutBin := getFlagValue(args, "--scout-bin")
		if scoutBin == "" {
			scoutBin = os.Getenv("SOUL_SCOUT_BIN")
		}
		if scoutBin == "" {
			cwd, _ := os.Getwd()
			candidate := filepath.Join(cwd, "products", "scout", "scout")
			if _, err := os.Stat(candidate); err == nil {
				scoutBin = candidate
			}
		}
		if scoutBin != "" {
			productConfigs = append(productConfigs, config.ProductConfig{
				Name: "scout", Binary: scoutBin, Label: "Career Intelligence", Color: "brainstorm",
			})
		}
	} else {
		// Config file exists — apply env var / CLI flag overrides per product.
		for i := range productConfigs {
			pc := &productConfigs[i]
			envKey := "SOUL_" + strings.ToUpper(strings.ReplaceAll(pc.Name, "-", "_")) + "_BIN"
			if envBin := os.Getenv(envKey); envBin != "" {
				pc.Binary = envBin
			}
			if flagBin := getFlagValue(args, "--"+pc.Name+"-bin"); flagBin != "" {
				pc.Binary = flagBin
			}
		}
	}

	ctx := context.Background()
	for _, pc := range productConfigs {
		if pc.Binary == "" {
			continue
		}
		if _, err := os.Stat(pc.Binary); err != nil {
			log.Printf("WARNING: %s binary not found at %s", pc.Name, pc.Binary)
			continue
		}
		fmt.Printf("  Starting %s product: %s\n", pc.Name, pc.Binary)
		if err := manager.StartProduct(ctx, pc.Name, pc.Binary); err != nil {
			log.Printf("WARNING: failed to start %s product: %v", pc.Name, err)
		} else {
			fmt.Printf("  %s product started\n", pc.Name)
		}
	}

	// Create server with embedded SPA (falls back to placeholder if web/dist not built).
	srv := server.NewWithWebFS(cfg, manager, aiClient, plannerStore, soul.WebDist, productConfigs)

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
	fmt.Println("  soul serve [--port PORT] [--dev]   Start web UI")
	fmt.Println("  soul --version                     Show version")
	fmt.Println("  soul --help                        Show this help")
	fmt.Println()
	fmt.Println("Authentication (in priority order):")
	fmt.Println("  ANTHROPIC_API_KEY      Claude API key")
	fmt.Println("  ~/.claude/.credentials.json   Claude Max/Pro OAuth (auto-detected)")
	fmt.Println()
	fmt.Println("Products:")
	fmt.Println("  Configure in ~/.soul/products.yaml")
	fmt.Println("  Override binary: SOUL_<NAME>_BIN or --<name>-bin flag")
	fmt.Println()
	fmt.Println("Environment:")
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
