package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: soul <command>")
		fmt.Println("commands: serve, metrics")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		fmt.Println("soul-v2 server starting...")
		// Server implementation will be added in Phase 1.
	case "metrics":
		if len(os.Args) < 3 {
			fmt.Println("usage: soul metrics <subcommand>")
			fmt.Println("subcommands: tail, log")
			os.Exit(1)
		}
		runMetrics(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
