// cmd/mic-test demonstrates basic audio capture functionality.
// This is a development tool to verify AudioService works correctly.
//
// Usage:
//
//	go run cmd/mic-test/main.go
//
// Press Ctrl+C to stop recording.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Ostrovsky42/speak2type/internal/audio"
)

func main() {
	fmt.Println("🎤 Speak2Type Audio Test")
	fmt.Println("=====================")
	fmt.Println()

	// List available devices
	fmt.Println("📋 Available audio devices:")
	devices, err := audio.ListDevices(nil)
	if err != nil {
		fmt.Printf("❌ Error listing devices: %v\n", err)
		os.Exit(1)
	}

	for _, dev := range devices {
		fmt.Printf("  %s\n", dev.String())
	}
	fmt.Println()

	// Create audio service with default config
	config := audio.DefaultConfig()
	fmt.Printf("🔧 Configuration:\n")
	fmt.Printf("  Sample Rate: %d Hz\n", config.SampleRate)
	fmt.Printf("  Channels: %d (mono)\n", config.Channels)
	fmt.Printf("  Buffer: %d ms\n", config.BufferMS)
	fmt.Printf("  Ring Buffer: %.1f seconds\n", config.RingBufferDuration)
	fmt.Println()

	service, err := audio.NewAudioService(config)
	if err != nil {
		fmt.Printf("❌ Error creating audio service: %v\n", err)
		os.Exit(1)
	}
	defer service.Close()

	// Start recording
	fmt.Println("▶️  Starting audio capture...")
	err = service.Start()
	if err != nil {
		fmt.Printf("❌ Error starting audio: %v\n", err)
		os.Exit(1)
	}
	defer service.Stop()

	fmt.Println("✅ Recording started!")
	fmt.Println()
	fmt.Println("🎙️  Speak into your microphone...")
	fmt.Println("   Press Ctrl+C to stop")
	fmt.Println()

	// Set up signal handler
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Statistics reporting ticker
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Main loop
	for {
		select {
		case <-sigChan:
			fmt.Println("\n\n⏹️  Stopping...")
			return

		case <-ticker.C:
			stats := service.GetStats()
			printStats(stats)
		}
	}
}

func printStats(stats audio.AudioServiceStats) {
	fmt.Printf("\r📊 Stats: Uptime=%s | Callbacks=%d | Frames=%d | Dropped=%.3f%% | Buffer=%.1f%%  ",
		stats.Uptime.Round(time.Second),
		stats.CallbackHits,
		stats.TotalFrames,
		stats.DropRate,
		stats.BufferStats.Utilization*100,
	)
}
