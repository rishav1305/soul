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
	"time"

	"github.com/rishav1305/soul-v2/internal/mesh/agent"
	"github.com/rishav1305/soul-v2/internal/mesh/hub"
	"github.com/rishav1305/soul-v2/internal/mesh/node"
	"github.com/rishav1305/soul-v2/internal/mesh/server"
	"github.com/rishav1305/soul-v2/internal/mesh/store"
)

func main() {
	host := envOr("SOUL_MESH_HOST", "127.0.0.1")
	port := envInt("SOUL_MESH_PORT", 3024)
	name := envOr("SOUL_MESH_NAME", "")
	role := envOr("SOUL_MESH_ROLE", "hub")
	secret := envOr("SOUL_MESH_SECRET", "soul-mesh-dev-secret")
	hubURL := envOr("SOUL_MESH_HUB", "")
	dataDir := envOr("SOUL_V2_DATA_DIR", defaultDataDir())

	if err := os.MkdirAll(dataDir, 0700); err != nil {
		log.Fatalf("mesh: create data dir: %v", err)
	}

	// Load or create node identity.
	nodeID, err := node.LoadOrCreateID(filepath.Join(dataDir, "mesh-node-id"))
	if err != nil {
		log.Fatalf("mesh: load node id: %v", err)
	}

	// Gather system info.
	snap, err := node.SystemSnapshot()
	if err != nil {
		log.Fatalf("mesh: system snapshot: %v", err)
	}
	snap.ID = nodeID
	snap.Host = host
	snap.Port = port
	snap.IsHub = role == "hub"
	if name != "" {
		snap.Name = name
	}

	// Open store.
	st, err := store.Open(filepath.Join(dataDir, "mesh.db"))
	if err != nil {
		log.Fatalf("mesh: open store: %v", err)
	}
	defer st.Close()

	// Register self.
	self := store.Node{
		ID:             snap.ID,
		Name:           snap.Name,
		Host:           snap.Host,
		Port:           snap.Port,
		Role:           role,
		Platform:       snap.Platform,
		Arch:           snap.Arch,
		CPUCores:       snap.CPUCores,
		RAMTotalMB:     snap.RAMTotalMB,
		StorageTotalGB: snap.StorageTotalGB,
		Status:         "online",
	}
	if err := st.RegisterNode(self); err != nil {
		log.Fatalf("mesh: register self: %v", err)
	}

	h := hub.New(st)
	srv := server.New(*snap, st, h, secret)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// If agent role, start heartbeat loop to hub.
	if role == "agent" && hubURL != "" {
		wsURL := fmt.Sprintf("ws://%s/ws/mesh", hubURL)
		ag := agent.New(*snap, wsURL, secret)
		go func() {
			if err := ag.HeartbeatLoop(ctx); err != nil && ctx.Err() == nil {
				log.Printf("mesh agent: heartbeat loop stopped: %v", err)
			}
		}()
	}

	// Graceful shutdown.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("mesh: shutting down...")
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("mesh: shutdown error: %v", err)
		}
	}()

	log.Printf("soul-mesh (%s) id=%s role=%s", snap.Name, snap.ID, role)
	if err := srv.ListenAndServe(host, port); err != nil && ctx.Err() != nil {
		// Expected shutdown.
	} else if err != nil {
		log.Fatalf("mesh: server error: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".soul-v2"
	}
	return filepath.Join(home, ".soul-v2")
}
