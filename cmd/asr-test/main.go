// cmd/asr-test demonstrates the full audio pipeline: Audio -> VAD -> ASR.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Ostrovsky42/speak2type/internal/asr"
	"github.com/Ostrovsky42/speak2type/internal/audio"
	"github.com/Ostrovsky42/speak2type/internal/vad"
)

func main() {
	fmt.Println("🗣️  Speak2Type ASR Test")
	fmt.Println("===================")

	// 1. Init Audio
	audioConfig := audio.DefaultConfig()
	audioService, err := audio.NewAudioService(audioConfig)
	if err != nil {
		panic(err)
	}
	defer audioService.Close()

	// 2. Init VAD
	vadConfig := vad.DefaultConfig()
	vadService, err := vad.NewVADService(vadConfig)
	if err != nil {
		panic(err)
	}
	defer vadService.Close()
	gate := vad.NewGate(vad.DefaultGateConfig())

	// 3. Init ASR
	asrConfig := asr.DefaultConfig()
	asrConfig.ModelPath = "models/ggml-base.bin"
	asrConfig.LanguageMode = "ru" // Force Russian for demo

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

	fmt.Println("✅ Pipeline started. Speak in Russian...")

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
			samples := audioService.SnapshotLatest(vadConfig.ChunkSize)
			if len(samples) < vadConfig.ChunkSize {
				continue
			}
			copy(chunk, samples)

			// VAD
			prob, err := vadService.Process(chunk)
			if err != nil {
				continue
			}
			_, active := gate.Process(prob)

			// Logic: Accumulate speech
			if active {
				if !isSpeaking {
					fmt.Print("🔴") // Start speaking
					isSpeaking = true
				}
				speechBuffer = append(speechBuffer, chunk...)

				// Force split if too long
				if time.Duration(len(speechBuffer)/16000)*time.Second > MaxSpeechDuration {
					submitASR(asrService, speechBuffer)
					speechBuffer = nil
					fmt.Print("✂️")
				}
			} else {
				if isSpeaking {
					// Just stopped speaking
					silenceStart = time.Now()
					isSpeaking = false
					fmt.Print("⚫") // Silence
				}

				// If silent for enough time and have data, submit
				if len(speechBuffer) > 0 && time.Since(silenceStart) > SilenceSplit {
					submitASR(asrService, speechBuffer)
					speechBuffer = nil
					fmt.Print("🚀") // Sent
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
