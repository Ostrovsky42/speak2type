//go:build !tray

package main

import (
	"fmt"
	"os"

	"github.com/Ostrovsky42/speak2type/internal/cli"
)

func handleDefaultCommand() {
	printUsage()
	os.Exit(0)
}

func handleRunCommand(args []string) {
	os.Exit(cli.RunSession(args))
}

func handleTrayCommand() {
	fmt.Println("❌ Tray support is not compiled into this binary.")
	fmt.Println("💡 To enable tray, install dependencies and build with '-tags tray':")
	fmt.Println("   sudo apt install libayatana-appindicator3-dev")
	fmt.Println("   go build -tags tray ...")
	os.Exit(1)
}
