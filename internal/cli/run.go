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
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Ostrovsky42/speak2type/internal/asr"
	"github.com/Ostrovsky42/speak2type/internal/audio"
	"github.com/Ostrovsky42/speak2type/internal/event"
	"github.com/Ostrovsky42/speak2type/internal/input"
	"github.com/Ostrovsky42/speak2type/internal/ipc"
	"github.com/Ostrovsky42/speak2type/internal/logging"
	"github.com/Ostrovsky42/speak2type/internal/merger"
	"github.com/Ostrovsky42/speak2type/internal/notify"
	"github.com/Ostrovsky42/speak2type/internal/session"
	"github.com/Ostrovsky42/speak2type/internal/vad"
	"github.com/Ostrovsky42/speak2type/internal/version"
	"github.com/Ostrovsky42/speak2type/pkg/config"
)

// RunSession executes the main session loop.
func RunSession(args []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	deviceIndex := fs.Int("device-index", -1, "capture device index")
	lang := fs.String("lang", "ru", "ASR language")
	modelPath := fs.String("model", "models/silero_vad.onnx", "path to silero_vad.onnx")
	asrModelPath := fs.String("asr-model", "", "path to local Whisper ggml model (overrides config asr.model_path)")
	asrProvider := fs.String("asr-provider", "", "ASR provider: local, openai, or groq (overrides config asr.provider)")
	asrCloudModel := fs.String("asr-cloud-model", "", "cloud ASR model ID (overrides config asr.model)")
	asrEndpoint := fs.String("asr-endpoint", "", "OpenAI-compatible transcription endpoint override")
	asrAPIKeyEnv := fs.String("asr-api-key-env", "", "environment variable containing cloud ASR API key")
	asrPrompt := fs.String("asr-prompt", "", "optional ASR context prompt for cloud providers")
	asrResponseFormat := fs.String("asr-response-format", "", "cloud ASR response format: json or text")
	asrTimeout := fs.Duration("asr-timeout", 0, "cloud ASR request timeout (e.g. 10s, overrides config asr.timeout_seconds)")
	singleLogit := fs.Bool("single-logit", false, "treat VAD output as logit")
	forceWayland := fs.Bool("force-wayland-inject", false, "allow text injection on Wayland (risky)")
	noRestore := fs.Bool("no-restore", false, "don't restore clipboard after paste")
	dryRun := fs.Bool("dry-run", false, "don't perform actual injection, only log")
	observe := fs.Duration("observe", 0, "observe and print active window changes for the given duration (e.g. 10s)")
	focusDelay := fs.Int("focus-delay-ms", 0, "delay before paste (ms)")
	pasteDelay := fs.Int("paste-delay-ms", 700, "delay after Ctrl+V (ms) (default 700)")
	settleDelay := fs.Int("settle-delay-ms", 150, "delay after clipboard write (ms) (default 150)")
	daemon := fs.Bool("daemon", false, "run in background")
	hotkey := fs.String("hotkey", "", "global hotkey (default from config, fallback f8)")
	logLevel := fs.String("log-level", "", "log level: debug, info, warn, error")
	disableFocusGuard := fs.Bool("disable-focus-guard", false, "disable focus guard (allow pasting even if active window changed)")

	var sigChan chan os.Signal
	fs.Parse(args)

	if IsDaemonRunning() {
		fmt.Println("❌ speak2type is already running.")
		return 1
	}

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

	// Acquire lock before writing PID
	if err := AcquireLock(); err != nil {
		log.Fatalf("❌ Lock failed: %v", err)
	}

	if err := WritePID(); err != nil {
		log.Printf("⚠️  Failed to write PID file: %v", err)
	}
	defer RemovePID()

	shutdownChan := make(chan struct{})
	var shutdownOnce sync.Once
	requestShutdown := func() {
		shutdownOnce.Do(func() {
			close(shutdownChan)
		})
	}

	// Setup signal handling for clean exit
	sigChan = make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		s := <-sigChan
		log.Printf("Received signal %v, shutting down...", s)
		requestShutdown()
	}()

	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		log.Printf("config load failed: %v (using defaults)", cfgErr)
		cfg = config.Default()
	}

	var cfgValue atomic.Value
	cfgValue.Store(cfg)

	bus := event.NewBus()
	logLevelOverride := strings.TrimSpace(*logLevel)
	startLogSubscriber(bus, func() logging.Level {
		if logLevelOverride != "" {
			return logging.Parse(logLevelOverride)
		}
		cfg, ok := cfgValue.Load().(*config.Config)
		if ok && cfg != nil {
			return logging.Parse(cfg.Logging.Level)
		}
		return logging.LevelInfo
	})
	startDropMonitor(bus)

	bus.Publish(event.Event{
		Type:    event.TypeAppStarted,
		Level:   event.LevelInfo,
		State:   event.StateIdle,
		Message: fmt.Sprintf("version=%s", version.Version),
	})
	if cfgErr != nil {
		bus.Publish(event.Event{
			Type:      event.TypeError,
			Level:     event.LevelWarn,
			State:     event.StateError,
			ErrorCode: "config_load_failed",
			Message:   cfgErr.Error(),
			Hint:      "fix the config file or remove it to use defaults",
		})
	}

	notifier, err := notify.New("Speak2Type")
	if err != nil {
		log.Printf("notifications disabled: %v", err)
	} else {
		defer notifier.Close()
		startNotifySubscriber(bus, notifier, &cfgValue)
	}

	hotkeyValue := strings.TrimSpace(*hotkey)
	langOverride := flagPresent(args, "lang")
	hotkeyOverride := hotkeyValue != ""
	if hotkeyValue == "" {
		if cfg.Session.Hotkey != "" {
			hotkeyValue = cfg.Session.Hotkey
		} else {
			hotkeyValue = "f8"
		}
	}
	hotkeyDisplay := formatHotkey(hotkeyValue)

	var applyConfig func(*config.Config)
	reloadConfig := func() error {
		newCfg, err := config.Load()
		if err != nil {
			bus.Publish(event.Event{
				Type:      event.TypeError,
				Level:     event.LevelError,
				State:     event.StateError,
				ErrorCode: "config_load_failed",
				Message:   err.Error(),
				Hint:      "fix the config file or remove it to use defaults",
			})
			return err
		}
		cfgValue.Store(newCfg)
		cfg = newCfg
		if applyConfig != nil {
			applyConfig(newCfg)
		}

		if !hotkeyOverride && newCfg.Session.Hotkey != "" && newCfg.Session.Hotkey != hotkeyValue {
			log.Printf("config hotkey changed to %q; restart required to apply", newCfg.Session.Hotkey)
		}
		return nil
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
		bus.Publish(event.Event{
			Type:      event.TypeError,
			Level:     event.LevelError,
			State:     event.StateError,
			ErrorCode: "audio_init_failed",
			Message:   err.Error(),
			Hint:      "check audio device access and permissions",
		})
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
		bus.Publish(event.Event{
			Type:      event.TypeError,
			Level:     event.LevelError,
			State:     event.StateError,
			ErrorCode: "vad_init_failed",
			Message:   err.Error(),
			Hint:      "check VAD model path and dependencies",
		})
		return 1
	}
	defer vadSvc.Close()

	gate := vad.NewGate(vad.DefaultGateConfig())

	// 3. Init ASR
	asrConfig := asr.DefaultConfig()
	asrConfig.Provider = strings.TrimSpace(cfg.ASR.Provider)
	if asrConfig.Provider == "" {
		asrConfig.Provider = asr.DefaultConfig().Provider
	}
	if strings.TrimSpace(*asrProvider) != "" {
		asrConfig.Provider = strings.TrimSpace(*asrProvider)
	}

	asrConfig.ModelPath = strings.TrimSpace(cfg.ASR.ModelPath)
	if asrConfig.ModelPath == "" {
		asrConfig.ModelPath = asr.DefaultConfig().ModelPath
	}
	if strings.TrimSpace(*asrModelPath) != "" {
		asrConfig.ModelPath = strings.TrimSpace(*asrModelPath)
	}

	asrConfig.Model = strings.TrimSpace(cfg.ASR.Model)
	if strings.TrimSpace(*asrCloudModel) != "" {
		asrConfig.Model = strings.TrimSpace(*asrCloudModel)
	}
	asrConfig.Endpoint = strings.TrimSpace(cfg.ASR.Endpoint)
	if strings.TrimSpace(*asrEndpoint) != "" {
		asrConfig.Endpoint = strings.TrimSpace(*asrEndpoint)
	}
	asrConfig.APIKey = strings.TrimSpace(cfg.ASR.APIKey)
	asrConfig.OpenAIAPIKey = strings.TrimSpace(cfg.ASR.OpenAIAPIKey)
	asrConfig.GroqAPIKey = strings.TrimSpace(cfg.ASR.GroqAPIKey)
	asrConfig.APIKeyEnv = strings.TrimSpace(cfg.ASR.APIKeyEnv)
	if strings.TrimSpace(*asrAPIKeyEnv) != "" {
		asrConfig.APIKeyEnv = strings.TrimSpace(*asrAPIKeyEnv)
	}
	asrConfig.Prompt = strings.TrimSpace(cfg.ASR.Prompt)
	if strings.TrimSpace(*asrPrompt) != "" {
		asrConfig.Prompt = strings.TrimSpace(*asrPrompt)
	}
	asrConfig.ResponseFormat = strings.TrimSpace(cfg.ASR.ResponseFormat)
	if asrConfig.ResponseFormat == "" {
		asrConfig.ResponseFormat = asr.DefaultConfig().ResponseFormat
	}
	if strings.TrimSpace(*asrResponseFormat) != "" {
		asrConfig.ResponseFormat = strings.TrimSpace(*asrResponseFormat)
	}
	if cfg.ASR.TimeoutSeconds > 0 {
		asrConfig.Timeout = time.Duration(cfg.ASR.TimeoutSeconds) * time.Second
	}
	if *asrTimeout > 0 {
		asrConfig.Timeout = *asrTimeout
	}
	asrConfig.SampleRate = int(audioConfig.SampleRate)

	effectiveLang := strings.TrimSpace(*lang)
	if !langOverride && cfg.ASR.LanguageMode != "" {
		effectiveLang = cfg.ASR.LanguageMode
	}
	if effectiveLang == "" {
		effectiveLang = "auto"
	}
	asrConfig.LanguageMode = effectiveLang

	fmt.Printf("Initializing ASR (%s)...\n", asrConfig.Provider)
	asrSvc, err := asr.NewASRService(asrConfig)
	if err != nil {
		fmt.Printf("❌ ASR Error: %v\n", err)
		bus.Publish(event.Event{
			Type:      event.TypeError,
			Level:     event.LevelError,
			State:     event.StateError,
			ErrorCode: "asr_init_failed",
			Message:   err.Error(),
			Hint:      "check ASR provider settings, model path, API key, and CPU/network availability",
		})
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
		bus.Publish(event.Event{
			Type:      event.TypeError,
			Level:     event.LevelWarn,
			State:     event.StateError,
			ErrorCode: "injector_init_failed",
			Message:   err.Error(),
			Hint:      "ensure X11/Wayland permissions and accessibility settings",
		})
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
		SampleRate:        16000,
		ChunkSize:         vadConfig.ChunkSize,
		NoRestore:         *noRestore,
		DisableFocusGuard: *disableFocusGuard || cfg.Session.DisableFocusGuard,
	}

	orch := session.NewOrchestrator(orchConfig, deps)
	orch.SetEventBus(bus)
	orch.SetLanguage(effectiveLang)

	applyConfig = func(newCfg *config.Config) {
		if langOverride {
			return
		}
		newLang := strings.TrimSpace(newCfg.ASR.LanguageMode)
		if newLang == "" {
			newLang = "auto"
		}
		if newLang != asrSvc.LanguageMode() {
			asrSvc.SetLanguageMode(newLang)
			orch.SetLanguage(newLang)
		}
	}

	// 5. IPC Server
	ipcPath := GetSocketPath()
	ipcServer := ipc.NewServer(ipcPath)
	ipcServer.HandleFunc = func(msg ipc.Message) (interface{}, error) {
		switch msg.Command {
		case "status":
			return orch.GetIPCState(), nil
		case "get_state":
			snap := bus.Snapshot()
			return ipc.AppState{
				State:       string(snap.State),
				Hotkey:      snap.Hotkey,
				LastEventID: snap.LastEventID,
			}, nil
		case "toggle":
			err := orch.Toggle()
			if err != nil {
				bus.Publish(event.Event{
					Type:      event.TypeError,
					Level:     event.LevelError,
					State:     event.StateError,
					ErrorCode: "toggle_failed",
					Message:   err.Error(),
					Hint:      "wait for the current session to finish",
				})
			}
			return orch.GetIPCState(), err
		case "reload_config":
			if err := reloadConfig(); err != nil {
				return nil, err
			}
			return map[string]string{"result": "reloaded"}, nil
		case "set_profile":
			var p struct {
				Profile string `json:"profile"`
			}
			if err := json.Unmarshal(msg.Params, &p); err == nil {
				orch.SetProfile(session.ProfileType(p.Profile))
			}
			return orch.GetIPCState(), nil
		case "set_lang":
			var p struct {
				Lang string `json:"lang"`
			}
			if err := json.Unmarshal(msg.Params, &p); err != nil {
				return nil, err
			}
			lang := strings.TrimSpace(p.Lang)
			if lang == "" {
				return nil, fmt.Errorf("lang is required")
			}
			asrSvc.SetLanguageMode(lang)
			orch.SetLanguage(lang)
			cfgUpdate, ok := cfgValue.Load().(*config.Config)
			if ok && cfgUpdate != nil {
				cfgUpdate.ASR.LanguageMode = lang
				if err := cfgUpdate.Save(); err != nil {
					log.Printf("config save failed: %v", err)
				}
				cfgValue.Store(cfgUpdate)
			}
			return orch.GetIPCState(), nil
		case "quit":
			requestShutdown()
			return map[string]string{"result": "shutting_down"}, nil
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
		ipcCh, cancelIPC := bus.Subscribe("ipc", 200)
		go func() {
			for evt := range ipcCh {
				ipcServer.Broadcast("app_event", evt)
			}
		}()
		defer cancelIPC()
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
				bus.Publish(event.Event{
					Type:      event.TypeError,
					Level:     event.LevelError,
					State:     event.StateError,
					ErrorCode: "session_start_failed",
					Message:   err.Error(),
					Hint:      "wait for processing to finish or check audio device",
				})
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
	hl := NewHotkeyListener(hotkeyValue, toggleFunc)
	if err := hl.Start(); err != nil {
		fmt.Printf("⚠️  Hotkey disabled: %v\n", err)
		bus.Publish(event.Event{
			Type:      event.TypeError,
			Level:     event.LevelError,
			State:     event.StateError,
			ErrorCode: "hotkey_disabled",
			Message:   err.Error(),
			Hint:      "use Enter in the terminal or fix hotkey configuration",
		})
	} else {
		bus.Publish(event.Event{
			Type:   event.TypeHotkeyRegistered,
			Level:  event.LevelInfo,
			State:  event.StateIdle,
			Hotkey: hotkeyDisplay,
		})
	}
	defer hl.Stop()

	if os.Getenv("SPEAK2TYPE_DAEMON") == "1" {
		<-shutdownChan
		return 0
	}

	lineCh := make(chan string)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			lineCh <- scanner.Text()
		}
		close(lineCh)
	}()

	for {
		select {
		case <-shutdownChan:
			log.Printf("Shutdown requested, exiting...")
			return 0
		case line, ok := <-lineCh:
			if !ok {
				return 0
			}
			if line == "exit" || line == "quit" {
				return 0
			}
			// Toggle logic (Enter key)
			toggleFunc()
		}
	}
}

func startLogSubscriber(bus *event.Bus, levelFn func() logging.Level) {
	ch, _ := bus.Subscribe("log", 200)
	go func() {
		for evt := range ch {
			threshold := levelFn()
			msgLevel := logging.FromEventLevel(evt.Level)
			if !logging.Allowed(threshold, msgLevel) {
				continue
			}
			log.Println(formatEvent(evt))
		}
	}()
}

func startDropMonitor(bus *event.Bus) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			for _, drop := range bus.DrainDrops() {
				bus.Publish(event.Event{
					Type:    event.TypeSubscriberDropped,
					Level:   event.LevelWarn,
					Message: fmt.Sprintf("subscriber=%s drops=%d", drop.Name, drop.Count),
				})
			}
		}
	}()
}

func startNotifySubscriber(bus *event.Bus, notifier *notify.Notifier, cfgValue *atomic.Value) {
	ch, _ := bus.Subscribe("notify", 50)
	go func() {
		for evt := range ch {
			cfg, ok := cfgValue.Load().(*config.Config)
			if !ok || cfg == nil {
				continue
			}
			summary, body, urgency, timeout, ok := buildNotification(evt, cfg.Notifications)
			if !ok {
				continue
			}
			if err := notifier.Notify(summary, body, urgency, timeout); err != nil {
				log.Printf("notification failed: %v", err)
			}
		}
	}()
}

func buildNotification(evt event.Event, cfg config.NotificationConfig) (string, string, uint8, time.Duration, bool) {
	switch evt.Type {
	case event.TypeError:
		if !cfg.Errors {
			return "", "", 0, 0, false
		}
		body := evt.Message
		if evt.Hint != "" {
			if body != "" {
				body += "\n"
			}
			body += evt.Hint
		}
		if body == "" {
			body = "Unknown error"
		}
		return "Speak2Type Error", body, 2, 5 * time.Second, true
	case event.TypeDone:
		if !cfg.Done {
			return "", "", 0, 0, false
		}
		return "Speak2Type", "Done", 1, 2 * time.Second, true
	case event.TypeRecordingStarted:
		if !cfg.Recording {
			return "", "", 0, 0, false
		}
		return "Speak2Type", "Recording started", 1, 2 * time.Second, true
	case event.TypeRecordingStopped:
		if !cfg.Recording {
			return "", "", 0, 0, false
		}
		return "Speak2Type", "Recording stopped", 1, 2 * time.Second, true
	default:
		return "", "", 0, 0, false
	}
}

func formatHotkey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.ToUpper(value)
}

func formatEvent(evt event.Event) string {
	level := evt.Level
	if level == "" {
		level = event.LevelInfo
	}
	fields := []string{
		"time=" + evt.Time.Format(time.RFC3339Nano),
		"level=" + string(level),
		"event=" + string(evt.Type),
		"state=" + string(evt.State),
		fmt.Sprintf("id=%d", evt.ID),
	}
	if evt.Hotkey != "" {
		fields = append(fields, fmt.Sprintf("hotkey=%q", evt.Hotkey))
	}
	if evt.TextLen > 0 {
		fields = append(fields, fmt.Sprintf("text_len=%d", evt.TextLen))
	}
	if evt.ErrorCode != "" {
		fields = append(fields, fmt.Sprintf("code=%s", evt.ErrorCode))
	}
	if evt.Message != "" {
		fields = append(fields, fmt.Sprintf("msg=%q", evt.Message))
	}
	if evt.Hint != "" {
		fields = append(fields, fmt.Sprintf("hint=%q", evt.Hint))
	}
	return strings.Join(fields, " ")
}

func flagPresent(args []string, name string) bool {
	short := "-" + name
	long := "--" + name
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == short || arg == long {
			return true
		}
		if strings.HasPrefix(arg, short+"=") || strings.HasPrefix(arg, long+"=") {
			return true
		}
	}
	return false
}
