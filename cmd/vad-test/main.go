// cmd/vad-test demonstrates real-time Voice Activity Detection.
// Combines AudioService, VADService, and Gate logic.
//
// Usage:
//
//	export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:.
//	go run cmd/vad-test/main.go
package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Ostrovsky42/speak2type/internal/audio"
	"github.com/Ostrovsky42/speak2type/internal/vad"
)

func main() {
	fmt.Println("🎤 Speak2Type VAD Test")
	fmt.Println("===================")

	// 1. Initialize Audio Service
	audioConfig := audio.DefaultConfig()
	audioConfig.BufferMS = 32 // Match VAD chunk size (512 samples @ 16kHz = 32ms)

	audioService, err := audio.NewAudioService(audioConfig)
	if err != nil {
		panic(fmt.Sprintf("Failed to init audio: %v", err))
	}
	defer audioService.Close()

	// 2. Initialize VAD Service
	vadConfig := vad.DefaultConfig()
	vadConfig.ModelPath = "models/silero_vad.onnx"

	vadService, err := vad.NewVADService(vadConfig)
	if err != nil {
		panic(fmt.Sprintf("Failed to init VAD: %v", err))
	}
	defer vadService.Close()

	// 3. Initialize Gate
	gateConfig := vad.DefaultGateConfig()
	gate := vad.NewGate(gateConfig)

	// Start Audio
	if err := audioService.Start(); err != nil {
		panic(fmt.Sprintf("Failed to start audio: %v", err))
	}
	defer audioService.Stop()

	fmt.Println("✅ Services started. Speak into microphone...")
	fmt.Println("   [Probability] | [Gate State] | [Visual]")
	fmt.Println("------------------------------------------")

	// Main Loop
	// In a real app, this would be in a worker goroutine reading from a channel.
	// Here we poll the ring buffer for simplicity, but we need to be careful to sync with audio rate.
	// Better: use a ticker matching chunk duration (32ms).

	ticker := time.NewTicker(32 * time.Millisecond)
	defer ticker.Stop()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Buffer for VAD processing
	chunk := make([]float32, vadConfig.ChunkSize)

	for {
		select {
		case <-sigChan:
			fmt.Println("\n⏹️  Stopping...")
			return

		case <-ticker.C:
			// Get latest samples from RingBuffer
			// Note: This is a simplified approach. In production, we should track read position
			// to ensure we process contiguous chunks without gaps or overlaps.
			// For visualization, SnapshotLatest is acceptable.
			samples := audioService.SnapshotLatest(vadConfig.ChunkSize)

			if len(samples) < vadConfig.ChunkSize {
				continue // Not enough data yet
			}

			// Copy to fixed-size chunk
			copy(chunk, samples)

			// Run VAD
			prob, err := vadService.Process(chunk)
			if err != nil {
				fmt.Printf("VAD Error: %v\n", err)
				continue
			}

			// Run Gate
			_, active := gate.Process(prob)

			// Visualize
			visualize(prob, active)
		}
	}
}

func visualize(prob float32, active bool) {
	// Bar graph
	bars := int(prob * 20)
	graph := strings.Repeat("█", bars) + strings.Repeat("░", 20-bars)

	state := "SILENCE"
	color := "\033[37m" // White

	if active {
		state = "SPEECH "
		color = "\033[32m" // Green
	} else if prob > 0.3 {
		color = "\033[33m" // Yellow (uncertain)
	}

	// Clear line and print
	fmt.Printf("\r%s[%.4f] | %s | %s\033[0m", color, prob, state, graph)
}
