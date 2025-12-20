package main

import (
	"fmt"
	"os"

	"github.com/Ostrovsky42/speak2type/internal/cli"
	"github.com/Ostrovsky42/speak2type/internal/version"
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
	case "stop":
		os.Exit(cli.RunStop())
	case "status":
		os.Exit(cli.RunStatus())
	case "install-service":
		os.Exit(cli.RunInstallService())
	case "tray", "ui":
		handleTrayCommand()
	case "version":
		fmt.Printf("Speak2Type Voice Input v%s\n", version.Version)
	default:
		fmt.Println("Usage: speak2type [run|doctor|inject-test|stop|status|install-service|version]")
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
