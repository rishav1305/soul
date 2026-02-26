package main

import (
	"fmt"
	"os"
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
		fmt.Println("◆ Soul serve: coming soon")
		return
	}

	printHelp()
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
