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
	if os.Getenv("SPEAK2TYPE_TRAY_BACKGROUND") != "1" {
		fmt.Println("🚀 Starting Speak2Type...")
		if err := cli.StartDaemonIfNeeded(); err != nil {
			fmt.Printf("❌ Failed to start background daemon: %v\n", err)
			os.Exit(1)
		}

		cmd := exec.Command(os.Args[0])
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, "SPEAK2TYPE_TRAY_BACKGROUND=1")
		if err := cmd.Start(); err != nil {
			fmt.Printf("❌ Failed to start tray in background: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Speak2Type is running in the background.")
		fmt.Println("💡 Look for the microphone icon in your system tray!")
		os.Exit(0)
	}

	os.Exit(tray.RunTray())
}

func handleTrayCommand() {
	if err := cli.StartDaemonIfNeeded(); err != nil {
		fmt.Printf("❌ Failed to start background daemon: %v\n", err)
		os.Exit(1)
	}
	os.Exit(tray.RunTray())
}
