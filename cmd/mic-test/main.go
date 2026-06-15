package main

import (
	"context"
	"flag"
	"fmt"
	"math"
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

	// Parse flags
	deviceIndex := flag.Int("device-index", -1, "capture device index to test (-1 = system default)")
	flag.Parse()

	// List available devices
	fmt.Println("📋 Available audio devices:")
	devices, err := audio.ListDevices(context.Background())
	if err != nil {
		fmt.Printf("❌ Error listing devices: %v\n", err)
		os.Exit(1)
	}

	for _, dev := range devices {
		marker := "   "
		if dev.IsDefault {
			marker = " 👉"
		}
		fmt.Printf("%s [%d] %s\n", marker, dev.Index, dev.Name)
	}
	fmt.Println()

	// Create audio service config
	audioConfig := audio.DefaultConfig()
	if *deviceIndex >= 0 && *deviceIndex < len(devices) {
		audioConfig.DeviceID = &devices[*deviceIndex].ID
		fmt.Printf("🔧 Using selected device: %s\n", devices[*deviceIndex].String())
	} else {
		// Resolve system default device explicitly
		var defaultDev *audio.DeviceInfo
		for i := range devices {
			if devices[i].IsDefault {
				defaultDev = &devices[i]
				break
			}
		}
		if defaultDev != nil {
			audioConfig.DeviceID = &defaultDev.ID
			fmt.Printf("🔧 Using system default device: %s\n", defaultDev.String())
		} else {
			fmt.Println("🔧 Using miniaudio default device")
		}
	}

	fmt.Printf("  Sample Rate: %d Hz | Channels: %d (mono)\n\n", audioConfig.SampleRate, audioConfig.Channels)

	service, err := audio.NewAudioService(audioConfig)
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
	fmt.Println("🎙️  Speak into your microphone to test volume level...")
	fmt.Println("   Press Ctrl+C to stop")
	fmt.Println()

	// Set up signal handler
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Volume rendering ticker (runs 10 times a second for smooth live feedback)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Main loop
	for {
		select {
		case <-sigChan:
			fmt.Println("\n\n⏹️  Stopping...")
			return

		case <-ticker.C:
			stats := service.GetStats()

			// Get recent 100ms of audio samples to calculate peak
			samples := service.Snapshot(0.1)

			peak := 0.0
			for _, s := range samples {
				abs := math.Abs(float64(s))
				if abs > peak {
					peak = abs
				}
			}

			// Generate simple text VU meter bar (20 chars max)
			barLength := int(peak * 20.0)
			if barLength > 20 {
				barLength = 20
			}
			bar := ""
			for i := 0; i < barLength; i++ {
				bar += "="
			}
			for len(bar) < 20 {
				bar += " "
			}

			fmt.Printf("\r📊 Stats: Uptime=%s | Peak Volume: [%s] %.4f | Buffer=%.1f%%  ",
				stats.Uptime.Round(time.Second),
				bar,
				peak,
				stats.BufferStats.Utilization*100,
			)
		}
	}
}
