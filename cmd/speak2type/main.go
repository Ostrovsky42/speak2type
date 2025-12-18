package main

import (
	"fmt"
	"os"

	"github.com/Ostrovsky42/speak2type/internal/cli"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	// Subcommand arguments (skipping program name and subcommand name)
	args := os.Args[2:]

	switch command {
	case "doctor":
		// Doctor doesn't take args currently, but good to be consistent
		os.Exit(cli.RunDoctor())
	case "run":
		os.Exit(cli.RunSession(args))
	case "inject-test":
		os.Exit(cli.RunInjectTest(args))
	case "version":
		fmt.Println("Speak2Type v0.9.0 (Phase 9)")
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: speak2type <command> [args]")
	fmt.Println("Commands:")
	fmt.Println("  doctor       Check system environment")
	fmt.Println("  run          Start voice session")
	fmt.Println("  inject-test  Test text injection")
	fmt.Println("  version      Show version")
}
