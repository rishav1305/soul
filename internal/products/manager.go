package products

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	soulv1 "github.com/rishav1305/soul/proto/soul/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ProductProcess represents a running product subprocess and its gRPC connection.
type ProductProcess struct {
	Name       string
	BinaryPath string
	SocketPath string
	Cmd        *exec.Cmd
	Client     soulv1.ProductServiceClient
	Conn       *grpc.ClientConn
}

// Manager manages the lifecycle of product subprocesses and their gRPC connections.
type Manager struct {
	mu       sync.Mutex
	registry *Registry
	products map[string]*ProductProcess
	dataDir  string
}

// NewManager creates a new product manager.
func NewManager(registry *Registry, dataDir string) *Manager {
	return &Manager{
		registry: registry,
		products: make(map[string]*ProductProcess),
		dataDir:  dataDir,
	}
}

// Registry returns the underlying product registry.
func (m *Manager) Registry() *Registry {
	return m.registry
}

// StartProduct starts a product binary as a subprocess, connects to its gRPC
// server via unix domain socket, retrieves its manifest, and registers it.
func (m *Manager) StartProduct(ctx context.Context, name, binaryPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	socketDir := filepath.Join(m.dataDir, "sockets")
	if err := os.MkdirAll(socketDir, 0o755); err != nil {
		return fmt.Errorf("creating socket directory: %w", err)
	}

	socketPath := filepath.Join(socketDir, name+".sock")

	// Remove stale socket file if it exists.
	_ = os.Remove(socketPath)

	// Start the product binary with --socket argument.
	cmd := exec.CommandContext(ctx, binaryPath, "--socket", socketPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting product %s: %w", name, err)
	}

	// Wait for the socket file to appear (up to 10 seconds).
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}

	if _, err := os.Stat(socketPath); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("socket file %s did not appear: %w", socketPath, err)
	}

	// Connect via gRPC over unix domain socket.
	conn, err := grpc.NewClient(
		"unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("connecting to product %s: %w", name, err)
	}

	client := soulv1.NewProductServiceClient(conn)

	// Retrieve the manifest from the product.
	manifest, err := client.GetManifest(ctx, &soulv1.Empty{})
	if err != nil {
		_ = conn.Close()
		_ = cmd.Process.Kill()
		return fmt.Errorf("getting manifest from product %s: %w", name, err)
	}

	// Register in the registry.
	m.registry.Register(name, manifest)

	m.products[name] = &ProductProcess{
		Name:       name,
		BinaryPath: binaryPath,
		SocketPath: socketPath,
		Cmd:        cmd,
		Client:     client,
		Conn:       conn,
	}

	return nil
}

// GetClient returns the gRPC client for the named product.
func (m *Manager) GetClient(name string) (soulv1.ProductServiceClient, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.products[name]
	if !ok {
		return nil, false
	}
	return p.Client, true
}

// StopAll stops all running product processes, closes their gRPC connections,
// and removes their socket files.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, p := range m.products {
		if p.Conn != nil {
			_ = p.Conn.Close()
		}
		if p.Cmd != nil && p.Cmd.Process != nil {
			_ = p.Cmd.Process.Kill()
			_ = p.Cmd.Wait()
		}
		_ = os.Remove(p.SocketPath)
		delete(m.products, name)
	}
}
