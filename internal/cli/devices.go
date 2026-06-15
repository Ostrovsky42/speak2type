package cli

import (
	"context"
	"fmt"

	"github.com/Ostrovsky42/speak2type/internal/audio"
)

// RunDevices lists all available audio capture devices.
func RunDevices() int {
	devices, err := audio.ListDevices(context.Background())
	if err != nil {
		fmt.Printf("❌ Failed to list audio devices: %v\n", err)
		return 1
	}

	if len(devices) == 0 {
		fmt.Println("⚠️  No audio devices found.")
		return 0
	}

	fmt.Println("🎤 Available Audio Input Devices:")
	for _, dev := range devices {
		if dev.IsDefault {
			fmt.Printf("  👉 %s\n", dev.String())
		} else {
			fmt.Printf("     %s\n", dev.String())
		}
	}
	return 0
}
