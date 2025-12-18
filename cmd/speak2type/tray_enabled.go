//go:build tray

package main

import (
	"os"

	"github.com/Ostrovsky42/speak2type/internal/cli/tray"
)

func handleTrayCommand() {
	os.Exit(tray.RunTray())
}
