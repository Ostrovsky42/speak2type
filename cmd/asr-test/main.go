// cmd/asr-test demonstrates the full audio pipeline: Audio -> VAD -> ASR.
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

	"github.com/Ostrovsky42/speak2type/internal/asr"
	"github.com/Ostrovsky42/speak2type/internal/audio"
	"github.com/Ostrovsky42/speak2type/internal/vad"
)

func main() {
	fmt.Println("🗣️  Speak2Type ASR Test")
	fmt.Println("===================")

	deviceIndex := flag.Int("device-index", -1, "capture device index (-1 = system default)")
	asrLang := flag.String("lang", "ru", "ASR language code (ru, en, etc)")
	singleLogit := flag.Bool("single-logit", false, "treat VAD output as logit (apply sigmoid)")
	gain := flag.Float64("gain", 1.0, "linear gain applied before VAD")
	normRMS := flag.Float64("norm-rms", 0, "normalize RMS to this target (0 = disabled)")
	bypassVAD := flag.Bool("bypass-vad", false, "bypass VAD and submit all audio to ASR")
	modelPath := flag.String("model", "models/silero_vad.onnx", "path to silero_vad.onnx")
	flag.Parse()

	// List devices
	fmt.Println("\n📋 Available audio devices:")
	devices, err := audio.ListDevices(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Failed to list devices: %v", err))
	}
	for _, dev := range devices {
		fmt.Printf("  %s\n", dev.String())
	}
	fmt.Println()

	// 1. Init Audio
	audioConfig := audio.DefaultConfig()
	if *deviceIndex >= 0 && *deviceIndex < len(devices) {
		audioConfig.DeviceID = &devices[*deviceIndex].ID
		fmt.Printf("🎚️  Using capture device: %s\n", devices[*deviceIndex].String())
	} else {
		fmt.Println("🎚️  Using capture device: system default")
	}

	audioService, err := audio.NewAudioService(audioConfig)
	if err != nil {
		panic(err)
	}
	defer audioService.Close()

	// 2. Init VAD
	vadConfig := vad.DefaultConfig()
	vadConfig.ModelPath = *modelPath
	vadConfig.SingleLogit = *singleLogit
	vadConfig.InputGain = float32(*gain)
	if *normRMS > 0 {
		vadConfig.NormalizeRMS = true
		vadConfig.TargetRMS = float32(*normRMS)
	}

	vadService, err := vad.NewVADService(vadConfig)
	if err != nil {
		panic(err)
	}
	defer vadService.Close()
	gate := vad.NewGate(vad.DefaultGateConfig())

	// 3. Init ASR
	asrConfig := asr.DefaultConfig()
	asrConfig.ModelPath = "models/ggml-base.bin"
	asrConfig.LanguageMode = *asrLang

	asrService, err := asr.NewASRService(asrConfig)
	if err != nil {
		panic(fmt.Sprintf("ASR Init Failed: %v\nDid you download the model?", err))
	}
	defer asrService.Stop()
	asrService.Start()

	// Start Audio
	if err := audioService.Start(); err != nil {
		panic(err)
	}
	defer audioService.Stop()

	fmt.Printf("✅ Pipeline started (Gain: %.1f, Norm: %.1f, Logit: %v). Speak...\n", *gain, *normRMS, *singleLogit)

	// Pipeline State
	var (
		speechBuffer []float32
		isSpeaking   bool
		silenceStart time.Time
	)

	// Constants
	const (
		MaxSpeechDuration = 7 * time.Second // Split every 7s
		SilenceSplit      = 500 * time.Millisecond
	)

	ticker := time.NewTicker(32 * time.Millisecond)
	defer ticker.Stop()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		for res := range asrService.Results() {
			if res.Error != nil {
				fmt.Printf("\n❌ ASR Error: %v\n", res.Error)
			} else {
				fmt.Printf("\n📝 [%s]: %s\n", res.Language, res.Text)
			}
		}
	}()

	chunk := make([]float32, vadConfig.ChunkSize)

	for {
		select {
		case <-sigChan:
			return
		case <-ticker.C:
			// Check available samples
			stats := audioService.GetStats()
			if stats.BufferStats.Available < vadConfig.ChunkSize {
				continue
			}

			// Read chunk (consumes data)
			n := audioService.Read(chunk)
			if n < vadConfig.ChunkSize {
				continue
			}

			// VAD
			var prob float32
			var active bool
			if !*bypassVAD {
				prob, err = vadService.Process(chunk)
				if err != nil {
					continue
				}
				_, active = gate.Process(prob)
			} else {
				// In bypass mode, we consider it "active" if there is any signal
				active = true
				prob = 0.5 // dummy
			}

			// Visual feedback (Live probability)
			bars := int(prob * 10)
			graph := strings.Repeat("█", bars) + strings.Repeat("░", 10-bars)
			fmt.Printf("\rVAD: [%.4f] %s | ", prob, graph)

			// Logic: Accumulate speech
			if active {
				if !isSpeaking {
					fmt.Print("🔴 SPEAKING ") // Start speaking
					isSpeaking = true
				}
				speechBuffer = append(speechBuffer, chunk...)

				// Force split if too long
				if time.Duration(len(speechBuffer)/16000)*time.Second > MaxSpeechDuration {
					submitASR(asrService, speechBuffer)
					speechBuffer = nil
					fmt.Print("\n✂️  SPLIT ")
				}
			} else {
				if isSpeaking {
					// Just stopped speaking
					silenceStart = time.Now()
					isSpeaking = false
					fmt.Print("⚫ SILENCE ") // Silence
				}

				// If silent for enough time and have data, submit
				if len(speechBuffer) > 0 && time.Since(silenceStart) > SilenceSplit {
					fmt.Print("🚀 SUBMITTING ") // Sent
					submitASR(asrService, speechBuffer)
					speechBuffer = nil
					fmt.Println()
				}
			}
		}
	}
}

func submitASR(s *asr.ASRService, samples []float32) {
	// Copy samples to avoid race if buffer reused (though here we set to nil)
	// AudioWindow expects its own slice
	buf := make([]float32, len(samples))
	copy(buf, samples)

	s.Submit(asr.AudioWindow{Samples: buf})
}
