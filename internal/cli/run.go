package cli

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/Ostrovsky42/speak2type/internal/asr"
	"github.com/Ostrovsky42/speak2type/internal/audio"
	"github.com/Ostrovsky42/speak2type/internal/input"
	"github.com/Ostrovsky42/speak2type/internal/merger"
	"github.com/Ostrovsky42/speak2type/internal/session"
	"github.com/Ostrovsky42/speak2type/internal/vad"
)

// RunSession executes the main session loop.
func RunSession(args []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	deviceIndex := fs.Int("device-index", -1, "capture device index")
	lang := fs.String("lang", "ru", "ASR language")
	modelPath := fs.String("model", "models/silero_vad_v4.onnx", "path to silero_vad.onnx")
	singleLogit := fs.Bool("single-logit", false, "treat VAD output as logit")
	forceWayland := fs.Bool("force-wayland-inject", false, "allow text injection on Wayland (risky)")
	noRestore := fs.Bool("no-restore", false, "don't restore clipboard after paste")

	fs.Parse(args)

	fmt.Println("🎹 Speak2Type Session Orchestrator")
	fmt.Println("===============================")

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
		fmt.Printf("❌ Audio Error: %v\n", err)
		return 1
	}
	defer audioSvc.Close()

	// 2. Init VAD
	fmt.Println("Initializing VAD...")
	vadConfig := vad.DefaultConfig()
	vadConfig.ModelPath = *modelPath
	vadConfig.SingleLogit = *singleLogit
	vadSvc, err := vad.NewVADService(vadConfig)
	if err != nil {
		fmt.Printf("❌ VAD Error: %v\n", err)
		return 1
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
		fmt.Printf("❌ ASR Error: %v\n", err)
		return 1
	}
	defer asrSvc.Stop()
	asrSvc.Start()

	// 4. Init Merger
	fmt.Println("Initializing Merger...")
	mergerSvc := merger.NewMergerService(merger.DefaultConfig())

	// 5. Init Input Service
	fmt.Println("Initializing Input Injector...")
	inputConfig := input.Config{
		Enabled:      true,
		ForceWayland: *forceWayland,
	}
	inputSvc, err := input.NewKeyboardInjector(inputConfig)
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
		NoRestore:  *noRestore,
	}

	orch := session.NewOrchestrator(orchConfig, deps)
	orch.Start()
	defer orch.Stop()

	// 7. UI Loop
	fmt.Println("\n✅ System Ready.")
	fmt.Println("Commands:")
	fmt.Println("  [Enter]    Toggle Recording (Continuous Mode)")
	fmt.Println("  exit/quit  Exit")
	fmt.Println("---------------------------------------------------")

	// Event Printer
	go func() {
		for evt := range orch.Events() {
			switch evt.Type {
			case session.EventStateChange:
				fmt.Printf("\n🔄 State -> %s\n", evt.State)
			case session.EventFullText:
				fmt.Printf("\r\033[K📝 %s", evt.Text)
			case session.EventError:
				fmt.Printf("\n❌ Error: %v\n", evt.Error)
			}
		}
	}()

	// 8. Hotkey Listener (F8)
	isRecording := false
	toggleFunc := func() {
		if !isRecording {
			fmt.Println("\n▶️  [Hotkey] Starting Session...")
			if err := orch.StartSession(session.ModeContinuous); err != nil {
				fmt.Printf("Failed: %v\n", err)
			} else {
				isRecording = true
			}
		} else {
			fmt.Println("\n⏹️  [Hotkey] Stopping Session...")
			orch.StopSession()
			isRecording = false
		}
	}

	// Start listener in background
	hl := NewHotkeyListener("f8", toggleFunc)
	hl.Start()
	defer hl.Stop()

	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := scanner.Text()

		if line == "exit" || line == "quit" {
			break
		}

		// Toggle logic (Enter key)
		toggleFunc()
	}
	return 0
}
