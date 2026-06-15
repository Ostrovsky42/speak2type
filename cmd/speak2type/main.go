package main

import (
	"fmt"
	"os"

	"github.com/Ostrovsky42/speak2type/internal/cli"
	"github.com/Ostrovsky42/speak2type/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		handleDefaultCommand()
	}

	command := os.Args[1]
	if command == "--version" || command == "-v" {
		fmt.Printf("Speak2Type Voice Input v%s\n", version.Version)
		return
	}
	// Subcommand arguments (skipping program name and subcommand name)
	args := os.Args[2:]

	switch command {
	case "doctor":
		// Doctor doesn't take args currently, but good to be consistent
		os.Exit(cli.RunDoctor())
	case "run":
		handleRunCommand(args)
	case "inject-test":
		os.Exit(cli.RunInjectTest(args))
	case "stop":
		os.Exit(cli.RunStop() + cli.RunStopTray())
	case "restart":
		os.Exit(cli.RunRestart())
	case "status":
		os.Exit(cli.RunStatus())
	case "install-service":
		os.Exit(cli.RunInstallService())
	case "enable":
		os.Exit(cli.RunEnableService())
	case "disable":
		os.Exit(cli.RunDisableService())
	case "tray", "ui":
		handleTrayCommand()
	case "version":
		fmt.Printf("Speak2Type Voice Input v%s\n", version.Version)
	default:
		fmt.Println("Usage: speak2type [run|doctor|inject-test|stop|restart|status|install-service|enable|disable|version]")
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: speak2type <command> [args]")
	fmt.Println("       speak2type --version")
	fmt.Println("Commands:")
	fmt.Println("  doctor       Check system environment")
	fmt.Println("  run          Start daemon + tray in tray builds; start voice session otherwise")
	fmt.Println("  inject-test  Test text injection")
	fmt.Println("  stop         Stop running daemon and tray")
	fmt.Println("  restart      Restart daemon and tray")
	fmt.Println("  status       Show daemon status")
	fmt.Println("  tray         Start tray UI (requires tray build tag)")
	fmt.Println("  install-service  Write systemd unit file")
	fmt.Println("  enable       Enable systemd user service")
	fmt.Println("  disable      Disable systemd user service")
	fmt.Println("  version      Show version")
}
