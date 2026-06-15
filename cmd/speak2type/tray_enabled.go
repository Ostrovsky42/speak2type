//go:build tray

package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/Ostrovsky42/speak2type/internal/cli"
	"github.com/Ostrovsky42/speak2type/internal/cli/tray"
)

func handleDefaultCommand() {
	printUsage()
	os.Exit(0)
}

func handleRunCommand(args []string) {
	if hasArg(args, "--daemon") {
		os.Exit(cli.RunSession(args))
	}

	if cli.IsTrayRunning() {
		fmt.Println("⚠️  Speak2Type tray application is already running.")
		os.Exit(0)
	}

	fmt.Println("🚀 Starting Speak2Type...")
	if err := cli.StartDaemonIfNeededWithArgs(args); err != nil {
		fmt.Printf("❌ Failed to start background daemon: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(os.Args[0], "tray")
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		fmt.Printf("❌ Failed to start tray in background: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Speak2Type is running in the background.")
	fmt.Println("💡 Look for the microphone icon in your system tray!")
	os.Exit(0)
}

func handleTrayCommand() {
	if cli.IsTrayRunning() {
		fmt.Println("⚠️  Speak2Type tray application is already running.")
		os.Exit(0)
	}

	if err := cli.StartDaemonIfNeeded(); err != nil {
		fmt.Printf("❌ Failed to start background daemon: %v\n", err)
		os.Exit(1)
	}
	os.Exit(tray.RunTray())
}

func hasArg(args []string, target string) bool {
	for _, arg := range args {
		if arg == target {
			return true
		}
	}
	return false
}
