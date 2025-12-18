package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Ostrovsky42/speak2type/internal/asr"
	"github.com/Ostrovsky42/speak2type/internal/audio"
	"github.com/Ostrovsky42/speak2type/internal/input"
	"github.com/Ostrovsky42/speak2type/internal/merger"
	"github.com/Ostrovsky42/speak2type/internal/session"
	"github.com/Ostrovsky42/speak2type/internal/vad"
)

func main() {
	fmt.Println("🎹 Speak2Type Session Orchestrator Demo")
	fmt.Println("=====================================")

	// Flags
	deviceIndex := flag.Int("device-index", -1, "capture device index")
	lang := flag.String("lang", "ru", "ASR language")
	modelPath := flag.String("model", "models/silero_vad.onnx", "path to silero_vad.onnx")
	singleLogit := flag.Bool("single-logit", false, "treat VAD output as logit")
	flag.Parse()

	// 1. Init Audio
	fmt.Println("Initializing Audio...")
	audioConfig := audio.DefaultConfig()
	devices, _ := audio.ListDevices(context.Background())
	if *deviceIndex >= 0 && *deviceIndex < len(devices) {
		audioConfig.DeviceID = &devices[*deviceIndex].ID
		fmt.Printf("  Using device: %s\n", devices[*deviceIndex].String())
	}

	audioSvc, err := audio.NewAudioService(audioConfig)
	if err != nil {
		panic(err)
	}
	defer audioSvc.Close()

	// 2. Init VAD
	fmt.Println("Initializing VAD...")
	vadConfig := vad.DefaultConfig()
	vadConfig.ModelPath = *modelPath
	vadConfig.SingleLogit = *singleLogit
	vadSvc, err := vad.NewVADService(vadConfig)
	if err != nil {
		panic(err)
	}
	defer vadSvc.Close()

	gate := vad.NewGate(vad.DefaultGateConfig())

	// 3. Init ASR
	fmt.Println("Initializing ASR...")
	asrConfig := asr.DefaultConfig()
	asrConfig.ModelPath = "models/ggml-base.bin"
	asrConfig.LanguageMode = *lang

	asrSvc, err := asr.NewASRService(asrConfig)
	if err != nil {
		panic(fmt.Sprintf("ASR Init Failed: %v", err))
	}
	defer asrSvc.Stop()
	asrSvc.Start()

	// 4. Init Merger
	fmt.Println("Initializing Merger...")
	mergerSvc := merger.NewMergerService(merger.DefaultConfig())

	// 5. Init Input Service
	fmt.Println("Initializing Input Injector...")
	inputSvc, err := input.NewKeyboardInjector()
	if err != nil {
		fmt.Printf("  ⚠️  Input Injector Warning: %v (continuing with text-only mode)\n", err)
	}

	// 6. Init Orchestrator
	fmt.Println("Initializing Orchestrator...")
	deps := session.Dependencies{
		Audio:  audioSvc,
		VAD:    vadSvc,
		Gate:   gate,
		ASR:    asrSvc,
		Merger: mergerSvc,
		Input:  inputSvc,
	}

	orchConfig := session.Config{
		SampleRate: 16000,
		ChunkSize:  vadConfig.ChunkSize,
	}

	orch := session.NewOrchestrator(orchConfig, deps)
	orch.Start()
	defer orch.Stop()

	// 7. UI Loop
	fmt.Println("\n✅ System Ready.")
	fmt.Println("Commands:")
	fmt.Println("  [Enter]    Toggle Recording (Continuous Mode)")
	fmt.Println("  hold       Simulate 'Hold to Record' (Quick Note) - NOT IMPLEMENTED in CLI easy way")
	fmt.Println("  exit/quit  Exit")
	fmt.Println("---------------------------------------------------")

	// Event Printer
	go func() {
		for evt := range orch.Events() {
			switch evt.Type {
			case session.EventStateChange:
				fmt.Printf("\n🔄 State -> %s\n", evt.State)
			case session.EventFullText:
				// Clear line/redraw? For now just print updates cleanly.
				// fmt.Printf("\r\033[K%s", evt.Text)
				fmt.Printf("\r\033[K📝 %s", evt.Text)
			case session.EventError:
				fmt.Printf("\n❌ Error: %v\n", evt.Error)
			}
		}
	}()

	scanner := bufio.NewScanner(os.Stdin)
	isRecording := false

	for scanner.Scan() {
		line := scanner.Text()

		if line == "exit" || line == "quit" {
			break
		}

		// Toggle logic
		if !isRecording {
			fmt.Println("▶️  Starting Session...")
			if err := orch.StartSession(session.ModeContinuous); err != nil {
				fmt.Printf("Failed to start: %v\n", err)
			} else {
				isRecording = true
			}
		} else {
			fmt.Println("\n⏹️  Stopping Session...")
			orch.StopSession()
			isRecording = false
			fmt.Println("Waiting for finalization...")
			// In real app, we'd wait for StateIdle event
			time.Sleep(1 * time.Second)
			fmt.Println("Done.")
		}
	}
}
