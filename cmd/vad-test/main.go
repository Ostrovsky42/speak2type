// cmd/vad-test demonstrates real-time Voice Activity Detection.
// Combines AudioService, VADService, and Gate logic.
//
// Usage:
//
//	export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:.
//	go run cmd/vad-test/main.go
package main

import (
	"context"
	"flag"
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

	deviceIndex := flag.Int("device-index", -1, "capture device index (see list above, -1 = system default)")
	debugRMS := flag.Bool("debug-rms", false, "print RMS/min/max for every VAD chunk")
	debugOut := flag.Bool("debug-out", false, "print raw VAD model outputs")
	sampleRate := flag.Int("sample-rate", 16000, "audio sample rate (8000 or 16000)")
	chunkSize := flag.Int("chunk-size", 512, "VAD chunk size (512, 1024, 1536)")
	inputGain := flag.Float64("gain", 1.0, "linear gain applied before VAD (e.g., 10, 20 for quiet mics)")
	singleLogit := flag.Bool("single-logit", false, "treat single-value VAD output as logit (apply sigmoid)")
	invertOut := flag.Bool("invert-out", false, "invert final VAD probability (prob = 1 - prob)")
	gateStart := flag.Float64("gate-start", 0.5, "gate start threshold (prob >= start opens speech)")
	gateEnd := flag.Float64("gate-end", 0.35, "gate end threshold (prob < end closes speech)")
	normRMS := flag.Float64("norm-rms", 0, "normalize chunk RMS to this target before VAD (0 = disabled)")
	listDevices := flag.Bool("list-devices", false, "list available audio input devices and exit")
	modelPath := flag.String("model", "models/silero_vad.onnx", "path to silero_vad.onnx")
	flag.Parse()

	// List devices logic
	if *listDevices {
		fmt.Println("\n📋 Available audio devices:")
		devices, err := audio.ListDevices(context.Background())
		if err != nil {
			panic(fmt.Sprintf("Failed to list devices: %v", err))
		}
		for i, dev := range devices {
			status := ""
			if dev.IsDefault {
				status = " (default)"
			}
			fmt.Printf("  [%d] %s%s\n", i, dev.Name, status)
		}
		return
	}

	// 1. Initialize Audio Service
	audioConfig := audio.DefaultConfig()
	audioConfig.SampleRate = uint32(*sampleRate)
	audioConfig.BufferMS = uint32((*chunkSize * 1000) / *sampleRate) // match chunk duration

	// Re-fetch devices for selection logic
	devices, err := audio.ListDevices(context.Background())
	if err != nil {
		fmt.Printf("⚠️  Failed to list devices: %v\n", err)
	}

	if len(devices) > 0 {
		selected := resolveDevice(devices, *deviceIndex)
		if selected != nil {
			audioConfig.DeviceID = &selected.ID
			fmt.Printf("🎚️  Using capture device: %s\n", selected.String())
		} else {
			fmt.Println("🎚️  Using capture device: system default (no default flag from driver)")
		}
	} else {
		fmt.Println("⚠️  No capture devices reported by malgo; falling back to default device selection")
	}
	fmt.Printf("🔧 Audio: %d Hz, %d channel(s), buffer %dms\n", audioConfig.SampleRate, audioConfig.Channels, audioConfig.BufferMS)

	audioService, err := audio.NewAudioService(audioConfig)
	if err != nil {
		panic(fmt.Sprintf("Failed to init audio: %v", err))
	}
	defer audioService.Close()

	// 2. Initialize VAD Service
	vadConfig := vad.DefaultConfig()
	vadConfig.ModelPath = *modelPath
	vadConfig.SampleRate = *sampleRate
	vadConfig.ChunkSize = *chunkSize
	vadConfig.DebugRMS = *debugRMS
	vadConfig.DebugOut = *debugOut
	vadConfig.InputGain = float32(*inputGain)
	vadConfig.SingleLogit = *singleLogit
	vadConfig.InvertOut = *invertOut
	if *normRMS > 0 {
		vadConfig.NormalizeRMS = true
		vadConfig.TargetRMS = float32(*normRMS)
	}

	vadService, err := vad.NewVADService(vadConfig)
	if err != nil {
		panic(fmt.Sprintf("Failed to init VAD: %v", err))
	}
	defer vadService.Close()

	// 3. Initialize Gate
	gateConfig := vad.DefaultGateConfig()
	gateConfig.ThresholdStart = float32(*gateStart)
	gateConfig.ThresholdEnd = float32(*gateEnd)
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

	ticker := time.NewTicker(10 * time.Millisecond) // Poll faster than audio rate
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
			// Check if we have enough data for a full chunk
			stats := audioService.GetStats()
			if stats.BufferStats.Available < vadConfig.ChunkSize {
				continue
			}

			// Read full chunk (consumes data)
			n := audioService.Read(chunk)
			if n < vadConfig.ChunkSize {
				continue
			}

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

func resolveDevice(devices []audio.DeviceInfo, idx int) *audio.DeviceInfo {
	if len(devices) == 0 {
		return nil
	}

	// Explicit index takes priority
	if idx >= 0 {
		if idx >= len(devices) {
			panic(fmt.Sprintf("device-index %d out of range (0-%d)", idx, len(devices)-1))
		}
		return &devices[idx]
	}

	// Otherwise pick the driver-reported default
	for i := range devices {
		if devices[i].IsDefault {
			return &devices[i]
		}
	}

	// Fallback to the first device if nothing is marked default
	return &devices[0]
}
