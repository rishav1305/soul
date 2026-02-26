package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/rishav1305/soul/internal/config"
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

	srv := server.New(cfg)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Printf("◆ Soul v%s\n\n", version)
	fmt.Println("Usage:")
	fmt.Println("  soul serve [--port PORT] [--dev]   Start web UI")
	fmt.Println("  soul --version                     Show version")
	fmt.Println("  soul --help                        Show this help")
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
