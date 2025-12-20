package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Ostrovsky42/speak2type/internal/asr"
	"github.com/Ostrovsky42/speak2type/internal/audio"
	"github.com/Ostrovsky42/speak2type/internal/input"
	"github.com/Ostrovsky42/speak2type/internal/ipc"
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
	dryRun := fs.Bool("dry-run", false, "don't perform actual injection, only log")
	observe := fs.Duration("observe", 0, "observe and print active window changes for the given duration (e.g. 10s)")
	focusDelay := fs.Int("focus-delay-ms", 0, "delay before paste (ms)")
	pasteDelay := fs.Int("paste-delay-ms", 200, "delay after Ctrl+V (ms) (default 200)")
	settleDelay := fs.Int("settle-delay-ms", 150, "delay after clipboard write (ms) (default 150)")
	daemon := fs.Bool("daemon", false, "run in background")
	hotkey := fs.String("hotkey", "f8", "global hotkey (default f8)")

	var sigChan chan os.Signal
	fs.Parse(args)

	if *daemon && os.Getenv("SPEAK2TYPE_DAEMON") != "1" {
		fmt.Println("🚀 Spawning Speak2Type in background...")
		logPath := GetLogPath()

		cmd := exec.Command(os.Args[0], os.Args[1:]...)
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, "SPEAK2TYPE_DAEMON=1")

		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err == nil {
			cmd.Stdout = logFile
			cmd.Stderr = logFile
		}

		if err := cmd.Start(); err != nil {
			fmt.Printf("❌ Failed to spawn daemon: %v\n", err)
			return 1
		}
		fmt.Printf("✅ Daemon started with PID %d.\n", cmd.Process.Pid)
		fmt.Printf("📝 Logs: %s\n", logPath)
		fmt.Println("💡 Use './speak2type stop' to terminate.")
		return 0
	}

	if os.Getenv("SPEAK2TYPE_DAEMON") == "1" {
		// Acquire lock before writing PID
		if err := AcquireLock(); err != nil {
			log.Fatalf("❌ Lock failed: %v", err)
		}

		if err := WritePID(); err != nil {
			log.Printf("⚠️  Failed to write PID file: %v", err)
		}
		defer RemovePID()

		// Setup signal handling for clean exit
		sigChan = make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	}

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
		DryRun:       *dryRun,
		FocusDelay:   time.Duration(*focusDelay) * time.Millisecond,
		SettleDelay:  time.Duration(*settleDelay) * time.Millisecond,
		PasteDelay:   time.Duration(*pasteDelay) * time.Millisecond,
	}
	inputSvc, err := input.NewKeyboardInjector(inputConfig)
	if err != nil {
		fmt.Printf("  ⚠️  Input Injector Warning: %v (continuing with text-only mode)\n", err)
	}

	// 6. Observe Mode
	if *observe > 0 {
		if inputSvc == nil {
			fmt.Println("⚠️  Observe Mode disabled: input injector unavailable")
		} else {
			fmt.Printf("🔭 Observe Mode Active for %v (monitoring active windows)...\n", *observe)
			go func() {
				start := time.Now()
				lastWindow := ""
				for time.Since(start) < *observe {
					current := inputSvc.GetActiveWindow()
					if current != lastWindow {
						fmt.Printf(" [Observe] Active Window: %s\n", current)
						lastWindow = current
					}
					time.Sleep(500 * time.Millisecond)
				}
				fmt.Println("🔭 Observe Mode Finished.")
			}()
		}
	}

	// 7. Init Orchestrator
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

	// 5. IPC Server
	ipcPath := GetSocketPath()
	ipcServer := ipc.NewServer(ipcPath)
	ipcServer.HandleFunc = func(msg ipc.Message) (interface{}, error) {
		switch msg.Command {
		case "status":
			return orch.GetIPCState(), nil
		case "toggle":
			err := orch.Toggle()
			return orch.GetIPCState(), err
		case "set_profile":
			var p struct {
				Profile string `json:"profile"`
			}
			if err := json.Unmarshal(msg.Params, &p); err == nil {
				orch.SetProfile(session.ProfileType(p.Profile))
			}
			return orch.GetIPCState(), nil
		case "ping":
			return map[string]string{"result": "pong"}, nil
		default:
			return nil, fmt.Errorf("unknown command: %s", msg.Command)
		}
	}
	if err := ipcServer.Start(); err != nil {
		fmt.Printf("⚠️  IPC Server failed to start: %v\n", err)
	} else {
		fmt.Printf("📡 IPC Server listening at: %s\n", ipcPath)
		orch.SetIPC(ipcServer)
		defer ipcServer.Stop()
	}

	orch.Start()
	defer orch.Stop()

	// 7. UI Loop
	fmt.Println("\n✅ System Ready.")
	fmt.Println("Commands:")
	fmt.Println("  [Enter]    Toggle Recording (Continuous Mode)")
	fmt.Println("  exit/quit  Exit")
	fmt.Println("---------------------------------------------------")

	isDaemon := os.Getenv("SPEAK2TYPE_DAEMON") == "1"

	// Event Printer
	go func() {
		for evt := range orch.Events() {
			switch evt.Type {
			case session.EventStateChange:
				fmt.Printf("\n🔄 State -> %s\n", evt.State)
			case session.EventFullText:
				if isDaemon {
					fmt.Printf("\n📝 %s\n", evt.Text)
				} else {
					fmt.Printf("\r\033[K📝 %s", evt.Text)
				}
			case session.EventError:
				if evt.Text != "" {
					fmt.Printf("\n⚠️  Injection Blocked: %v (text: %q)\n", evt.Error, evt.Text)
				} else {
					fmt.Printf("\n❌ Error: %v\n", evt.Error)
				}
			}
		}
	}()

	// 8. Hotkey Listener (F8)
	var toggleMu sync.Mutex
	toggleFunc := func() {
		toggleMu.Lock()
		defer toggleMu.Unlock()

		st := orch.GetIPCState()
		switch st.State {
		case "idle":
			fmt.Println("\n▶️  [Hotkey] Starting Session...")
			if err := orch.StartSession(session.ModeContinuous); err != nil {
				fmt.Printf("Failed: %v\n", err)
			}
		case "listening":
			fmt.Println("\n⏹️  [Hotkey] Stopping Session...")
			orch.StopSession()
		case "processing":
			if st.PendingASR > 0 {
				fmt.Printf("\n⏳  [Hotkey] Still processing... (%d pending)\n", st.PendingASR)
			} else {
				fmt.Println("\n⏳  [Hotkey] Still processing... please wait")
			}
		}
	}

	// Start listener in background
	hl := NewHotkeyListener(*hotkey, toggleFunc)
	hl.Start()
	defer hl.Stop()

	if os.Getenv("SPEAK2TYPE_DAEMON") == "1" {
		// Wait for signal instead of stdin
		s := <-sigChan
		log.Printf("Received signal %v, shutting down daemon...", s)
		return 0
	}

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
