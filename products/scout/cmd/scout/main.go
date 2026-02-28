package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rishav1305/soul/products/scout/internal"
	"github.com/rishav1305/soul/products/scout/internal/browser"
	soulv1 "github.com/rishav1305/soul/proto/soul/v1"
	"google.golang.org/grpc"
)

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "setup" {
		runSetup(os.Args[2:])
		return
	}

	// Default: gRPC server mode (launched by Soul).
	var socketPath string
	for i, arg := range os.Args[1:] {
		if arg == "--socket" && i+2 < len(os.Args) {
			socketPath = os.Args[i+2]
		}
		if strings.HasPrefix(arg, "--socket=") {
			socketPath = strings.TrimPrefix(arg, "--socket=")
		}
	}
	if socketPath == "" {
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  scout --socket PATH     Run as gRPC product server\n")
		fmt.Fprintf(os.Stderr, "  scout setup PLATFORM    Launch visible browser for platform login\n")
		fmt.Fprintf(os.Stderr, "\nPlatforms: %s\n", strings.Join(browser.AllPlatforms(), ", "))
		os.Exit(1)
	}

	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("failed to remove existing socket: %v", err)
	}

	lis, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", socketPath, err)
	}
	defer lis.Close()

	srv := grpc.NewServer()
	soulv1.RegisterProductServiceServer(srv, internal.NewScoutService())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		fmt.Printf("received signal %v, shutting down...\n", sig)
		srv.GracefulStop()
		os.Remove(socketPath)
	}()

	log.Printf("scout gRPC server listening on %s", socketPath)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("gRPC server error: %v", err)
	}
}

// runSetup launches a visible browser for the given platform and waits for
// the user to complete login. Designed for use over X11 forwarding.
func runSetup(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: scout setup PLATFORM\n")
		fmt.Fprintf(os.Stderr, "Platforms: %s\n", strings.Join(browser.AllPlatforms(), ", "))
		os.Exit(1)
	}
	platform := strings.ToLower(args[0])

	if os.Getenv("DISPLAY") == "" {
		fmt.Fprintln(os.Stderr, "Error: DISPLAY is not set. Use X11 forwarding:")
		fmt.Fprintln(os.Stderr, "  ssh -X rishav@192.168.0.128")
		fmt.Fprintln(os.Stderr, "  cd ~/soul && ./products/scout/scout setup linkedin")
		os.Exit(1)
	}

	urls, ok := browser.PlatformURLs[platform]
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown platform: %s\nSupported: %s\n",
			platform, strings.Join(browser.AllPlatforms(), ", "))
		os.Exit(1)
	}

	fmt.Printf("Launching %s login page... (DISPLAY=%s)\n", platform, os.Getenv("DISPLAY"))
	fmt.Printf("Login URL: %s\n", urls.Login)
	fmt.Println("Please log in. The browser will close automatically once login is detected.")
	fmt.Println("Press Ctrl+C to cancel.\n")

	b, page, err := browser.LaunchVisible(platform)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to launch browser: %v\n", err)
		os.Exit(1)
	}
	defer b.MustClose()

	// Handle Ctrl+C.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nCancelled.")
		b.MustClose()
		os.Exit(0)
	}()

	loginURL := urls.Login
	const (
		pollInterval = 2 * time.Second
		timeout      = 5 * time.Minute
	)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		time.Sleep(pollInterval)
		info := page.MustInfo()
		currentURL := info.URL
		if currentURL != loginURL && !strings.HasPrefix(currentURL, loginURL) {
			profileDir, _ := browser.ProfileDir(platform)
			fmt.Printf("\nLogin successful for %s!\n", platform)
			fmt.Printf("Session saved to: %s\n", profileDir)
			return
		}
	}

	fmt.Fprintf(os.Stderr, "\nLogin timed out after %s.\n", timeout)
	os.Exit(1)
}
