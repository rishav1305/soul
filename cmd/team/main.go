package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/rishav1305/soul/internal/team"
)

func main() {
	// Soft memory limit — keep tight on RPi.
	debug.SetMemoryLimit(128 * 1024 * 1024)

	cfg := team.DefaultConfig()

	log.Printf("soul-team starting on %s:%d (team=%s)", cfg.Host, cfg.Port, cfg.TeamName)

	srv, err := team.NewServer(cfg)
	if err != nil {
		log.Fatalf("soul-team: create server: %v", err)
	}

	// Graceful shutdown.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("soul-team: shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("soul-team: shutdown error: %v", err)
		}
	}()

	if err := srv.ListenAndServe(); err != nil {
		// http.ErrServerClosed is expected on graceful shutdown.
		if err.Error() != "http: Server closed" {
			log.Fatalf("soul-team: server error: %v", err)
		}
	}

	log.Println("soul-team: stopped")
}
