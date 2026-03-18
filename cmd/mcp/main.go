package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	prodctx "github.com/rishav1305/soul-v2/internal/chat/context"
	"github.com/rishav1305/soul-v2/internal/mcp/auth"
	"github.com/rishav1305/soul-v2/internal/mcp/server"
	"github.com/rishav1305/soul-v2/internal/mcp/tools"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: soul-mcp <command>")
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
	// Required environment variables.
	secret := os.Getenv("SOUL_MCP_SECRET")
	if secret == "" {
		log.Fatal("SOUL_MCP_SECRET is required")
	}

	adminPass := os.Getenv("SOUL_MCP_ADMIN_PASSWORD")
	if adminPass == "" {
		log.Fatal("SOUL_MCP_ADMIN_PASSWORD is required")
	}

	// Optional environment variables.
	host := os.Getenv("SOUL_MCP_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := 3028
	if p := os.Getenv("SOUL_MCP_PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}
	baseURL := os.Getenv("SOUL_MCP_BASE_URL")
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://%s:%d", host, port)
	}

	// Create dispatcher and registry.
	dispatcher := prodctx.NewDispatcher()
	registry := tools.NewRegistry(dispatcher)

	// Create OAuth handler.
	oauth := auth.NewOAuthHandler(secret, adminPass)
	oauth.SetBaseURL(baseURL)

	// Create server.
	opts := []server.Option{
		server.WithHost(host),
		server.WithPort(port),
		server.WithSecret(secret),
		server.WithRegistry(registry),
		server.WithOAuth(oauth),
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
