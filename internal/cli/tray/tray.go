package tray

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Ostrovsky42/speak2type/internal/cli"
	"github.com/Ostrovsky42/speak2type/internal/daemon"
	"github.com/Ostrovsky42/speak2type/internal/event"
	"github.com/Ostrovsky42/speak2type/internal/ipc"
	"github.com/Ostrovsky42/speak2type/pkg/config"
	"github.com/getlantern/systray"
)

//go:embed assets/idle.png
var iconIdle []byte

//go:embed assets/recording.png
var iconRecording []byte

var (
	client *ipc.Client
	cMu    sync.Mutex

	stateMu       sync.Mutex
	currentState  event.State = event.StateIdle
	currentHotkey             = ""
	stickyError               = false
	lastEventID   uint64
	doneTimer     *time.Timer
)

func RunTray() int {
	ensureTrayTempDir()
	if err := cli.AcquireTrayLock(); err != nil {
		fmt.Printf("⚠️  Tray already running or lock failed: %v\n", err)
		return 0
	}
	if err := cli.WriteTrayPID(); err != nil {
		fmt.Printf("⚠️  Failed to write tray PID: %v\n", err)
	}
	defer cli.RemoveTrayPID()

	systray.Run(onReady, onExit)
	return 0
}

func onReady() {
	systray.SetIcon(iconIdle)
	systray.SetTitle("Speak2Type")
	systray.SetTooltip("Speak2Type: Offline")

	mToggle := systray.AddMenuItem("Start/Stop Recording", "Start or stop recording")
	mConfig := systray.AddMenuItem("Open Config", "Open config file location")
	mReload := systray.AddMenuItem("Reload Config", "Reload configuration")
	systray.AddSeparator()

	mLang := systray.AddMenuItem("Language", "Select ASR language")
	mAuto := mLang.AddSubMenuItemCheckbox("Auto-detect", "", true)
	mEn := mLang.AddSubMenuItemCheckbox("English", "", false)
	mUk := mLang.AddSubMenuItemCheckbox("Ukrainian", "", false)
	mRu := mLang.AddSubMenuItemCheckbox("Russian", "", false)
	mOther := mLang.AddSubMenuItemCheckbox("Other...", "Custom language code", false)

	systray.AddSeparator()
	mProfile := systray.AddMenuItem("Profile", "Select behavior profile")
	mDic := mProfile.AddSubMenuItemCheckbox("Dictation", "Long pauses, stable", true)
	mCom := mProfile.AddSubMenuItemCheckbox("Commands", "Fast reaction", false)

	systray.AddSeparator()
	mASR := systray.AddMenuItem("ASR Provider", "Select speech recognition provider")
	mASRLocal := mASR.AddSubMenuItemCheckbox("Local whisper.cpp", "Use local Whisper model", true)
	mASROpenAI := mASR.AddSubMenuItemCheckbox("OpenAI", "Use OpenAI audio transcriptions", false)
	mASRGroq := mASR.AddSubMenuItemCheckbox("Groq", "Use Groq speech-to-text", false)
	mASR.AddSubMenuItem("Changes apply immediately", "Saved provider settings are reloaded by the daemon").Disable()
	mOpenAIKey := mASR.AddSubMenuItem("Set OpenAI API Key", "Store OpenAI API key in config")
	mGroqKey := mASR.AddSubMenuItem("Set Groq API Key", "Store Groq API key in config")
	applyASRProviderChecks(mASRLocal, mASROpenAI, mASRGroq)

	systray.AddSeparator()
	mDoctor := systray.AddMenuItem("Run Doctor", "Check system status")
	mLogs := systray.AddMenuItem("Open Logs", "View daemon logs")
	systray.AddSeparator()
	mRestart := systray.AddMenuItem("Restart Speak2Type", "Restart daemon and tray")
	mStop := systray.AddMenuItem("Stop Speak2Type", "Stop daemon and tray")

	client = ipc.NewClient(daemon.GetSocketPath())

	// Reconnection & Listen Loop
	go func() {
		for {
			cMu.Lock()
			if !client.IsConnected() {
				if err := client.Connect(); err == nil {
					fmt.Println("🚀 Connected to Speak2Type daemon")
					applySnapshot()
					// Start listener in a fresh goroutine for this connection
					go listenIPC(mEn, mUk, mRu, mAuto, mOther, mDic, mCom)
				} else {
					setOffline()
				}
			}
			cMu.Unlock()
			time.Sleep(5 * time.Second)
		}
	}()

	// Command loop
	go func() {
		for {
			select {
			case <-mToggle.ClickedCh:
				callIPC("toggle", nil)
			case <-mConfig.ClickedCh:
				openConfigPath()
			case <-mReload.ClickedCh:
				callIPC("reload_config", nil)
			case <-mUk.ClickedCh:
				callIPC("set_lang", map[string]string{"lang": "uk"})
			case <-mEn.ClickedCh:
				callIPC("set_lang", map[string]string{"lang": "en"})
			case <-mRu.ClickedCh:
				callIPC("set_lang", map[string]string{"lang": "ru"})
			case <-mAuto.ClickedCh:
				callIPC("set_lang", map[string]string{"lang": "auto"})
			case <-mOther.ClickedCh:
				if lang := promptLanguage(); lang != "" {
					callIPC("set_lang", map[string]string{"lang": lang})
				} else {
					openConfigPath()
				}
			case <-mDic.ClickedCh:
				callIPC("set_profile", map[string]string{"profile": "dictation"})
			case <-mCom.ClickedCh:
				callIPC("set_profile", map[string]string{"profile": "commands"})
			case <-mASRLocal.ClickedCh:
				if saveASRProvider("local") == nil {
					applyASRProviderChecks(mASRLocal, mASROpenAI, mASRGroq)
				}
			case <-mASROpenAI.ClickedCh:
				if saveASRProvider("openai") == nil {
					applyASRProviderChecks(mASRLocal, mASROpenAI, mASRGroq)
				}
			case <-mASRGroq.ClickedCh:
				if saveASRProvider("groq") == nil {
					applyASRProviderChecks(mASRLocal, mASROpenAI, mASRGroq)
				}
			case <-mOpenAIKey.ClickedCh:
				if key := promptSecret("OpenAI API Key"); key != "" {
					_ = saveASRAPIKey("openai", key)
				}
			case <-mGroqKey.ClickedCh:
				if key := promptSecret("Groq API Key"); key != "" {
					_ = saveASRAPIKey("groq", key)
				}
			case <-mDoctor.ClickedCh:
				openTerminal("speak2type doctor")
			case <-mLogs.ClickedCh:
				openTerminal(fmt.Sprintf("tail -f %s", daemon.GetLogPath()))
			case <-mRestart.ClickedCh:
				execPath, _ := os.Executable()
				cmd := exec.Command(execPath, "restart")
				_ = cmd.Start()
			case <-mStop.ClickedCh:
				callIPC("quit", nil)
				time.AfterFunc(200*time.Millisecond, systray.Quit)
			}
		}
	}()
}

func listenIPC(mEn, mUk, mRu, mAuto, mOther, mDic, mCom *systray.MenuItem) {
	client.Listen(func(msg ipc.Message) {
		switch msg.Event {
		case "app_event":
			var evt event.Event
			if err := json.Unmarshal(msg.Data, &evt); err == nil {
				applyEvent(evt)
			}
		case "state":
			var info ipc.StateInfo
			if err := json.Unmarshal(msg.Data, &info); err == nil {
				// Update language checkboxes
				switch info.Language {
				case "en":
					mEn.Check()
					mRu.Uncheck()
					mUk.Uncheck()
					mAuto.Uncheck()
					mOther.Uncheck()
				case "uk":
					mUk.Check()
					mEn.Uncheck()
					mRu.Uncheck()
					mAuto.Uncheck()
					mOther.Uncheck()
				case "ru":
					mRu.Check()
					mEn.Uncheck()
					mUk.Uncheck()
					mAuto.Uncheck()
					mOther.Uncheck()

				default:
					if info.Language != "" && info.Language != "auto" {
						mOther.Check()
						mAuto.Uncheck()
					} else {
						mAuto.Check()
						mOther.Uncheck()
					}
					mRu.Uncheck()
					mEn.Uncheck()
					mUk.Uncheck()
				}
				// Update profile checkboxes
				if info.Profile == "commands" {
					mCom.Check()
					mDic.Uncheck()
				} else {
					mDic.Check()
					mCom.Uncheck()
				}
			}
		}
	})
	// If Listen returns, it means connection was lost
	setOffline()
}

func callIPC(cmd string, params interface{}) {
	cMu.Lock()
	defer cMu.Unlock()
	_ = client.Call(cmd, params)
}

func applySnapshot() {
	raw, err := client.CallRaw("get_state", nil)
	if err != nil {
		setOffline()
		return
	}
	var snap ipc.AppState
	if err := json.Unmarshal(raw, &snap); err != nil {
		return
	}

	stateMu.Lock()
	if snap.State != "" {
		currentState = event.State(snap.State)
	} else {
		currentState = event.StateIdle
	}
	if snap.Hotkey != "" {
		currentHotkey = formatHotkey(snap.Hotkey)
	} else {
		if cfg, err := config.Load(); err == nil {
			currentHotkey = formatHotkey(cfg.Session.Hotkey)
		}
	}
	if snap.LastEventID > 0 {
		lastEventID = snap.LastEventID
	}
	stickyError = currentState == event.StateError
	stateMu.Unlock()

	if currentState == event.StateDone {
		scheduleDoneReset()
	}
	updateStatus()
}

func applyEvent(evt event.Event) {
	stateMu.Lock()
	if evt.ID != 0 && evt.ID <= lastEventID {
		stateMu.Unlock()
		return
	}
	if evt.ID != 0 {
		lastEventID = evt.ID
	}
	if evt.Hotkey != "" {
		currentHotkey = formatHotkey(evt.Hotkey)
	}

	switch evt.Type {
	case event.TypeAppStarted:
		stickyError = false
		if evt.State != "" {
			currentState = evt.State
		} else {
			currentState = event.StateIdle
		}
	case event.TypeRecordingStarted:
		stickyError = false
		currentState = event.StateRecording
	case event.TypeRecordingStopped, event.TypeTranscriptionStarted:
		if !stickyError {
			currentState = event.StateTranscribing
		}
	case event.TypeInjectionStarted, event.TypeInjectionFinished:
		if !stickyError && evt.State != "" {
			currentState = evt.State
		}
	case event.TypeDone:
		if !stickyError {
			currentState = event.StateDone
		}
	case event.TypeError:
		stickyError = true
		currentState = event.StateError
	default:
		if !stickyError && evt.State != "" {
			currentState = evt.State
		}
	}
	state := currentState
	stateMu.Unlock()

	if state == event.StateDone {
		scheduleDoneReset()
	}
	updateStatus()
}

func scheduleDoneReset() {
	stateMu.Lock()
	if doneTimer != nil {
		doneTimer.Stop()
	}
	doneTimer = time.AfterFunc(2*time.Second, func() {
		stateMu.Lock()
		if currentState == event.StateDone && !stickyError {
			currentState = event.StateIdle
		}
		stateMu.Unlock()
		updateStatus()
	})
	stateMu.Unlock()
}

func updateStatus() {
	stateMu.Lock()
	state := currentState
	hotkey := currentHotkey
	if stickyError {
		state = event.StateError
	}
	stateMu.Unlock()

	if state == "" {
		state = event.StateIdle
	}

	label := formatState(state)
	tooltip := fmt.Sprintf("Speak2Type: %s", label)
	if hotkey != "" {
		tooltip += fmt.Sprintf(" | Hotkey: %s", hotkey)
	}

	if state == event.StateRecording {
		systray.SetIcon(iconRecording)
	} else {
		systray.SetIcon(iconIdle)
	}
	systray.SetTooltip(tooltip)
}

func setOffline() {
	stateMu.Lock()
	stickyError = false
	currentState = event.StateIdle
	stateMu.Unlock()
	systray.SetIcon(iconIdle)
	systray.SetTooltip("Speak2Type: Offline")
}

func formatState(state event.State) string {
	switch state {
	case event.StateRecording:
		return "Recording"
	case event.StateTranscribing:
		return "Transcribing"
	case event.StateInjecting:
		return "Injecting"
	case event.StateDone:
		return "Done"
	case event.StateError:
		return "Error"
	default:
		return "Idle"
	}
}

func formatHotkey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.ToUpper(value)
}

func saveASRProvider(provider string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.ASR.Provider = provider
	switch provider {
	case "openai":
		if strings.TrimSpace(cfg.ASR.Model) == "" || strings.HasPrefix(cfg.ASR.Model, "whisper-") {
			cfg.ASR.Model = "gpt-4o-mini-transcribe"
		}
		if cfg.ASR.APIKeyEnv == "" {
			cfg.ASR.APIKeyEnv = "OPENAI_API_KEY"
		}
	case "groq":
		if strings.TrimSpace(cfg.ASR.Model) == "" || strings.HasPrefix(cfg.ASR.Model, "gpt-") {
			cfg.ASR.Model = "whisper-large-v3-turbo"
		}
		if cfg.ASR.APIKeyEnv == "" {
			cfg.ASR.APIKeyEnv = "GROQ_API_KEY"
		}
	}
	if err := cfg.Save(); err != nil {
		return err
	}
	callIPC("reload_config", nil)
	showInfo("ASR provider saved and reloaded.")
	return nil
}

func saveASRAPIKey(provider, key string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.ASR.Provider = provider
	key = strings.TrimSpace(key)
	if provider == "openai" {
		cfg.ASR.OpenAIAPIKey = key
		cfg.ASR.APIKeyEnv = "OPENAI_API_KEY"
		if strings.TrimSpace(cfg.ASR.Model) == "" || strings.HasPrefix(cfg.ASR.Model, "whisper-") {
			cfg.ASR.Model = "gpt-4o-mini-transcribe"
		}
	} else if provider == "groq" {
		cfg.ASR.GroqAPIKey = key
		cfg.ASR.APIKeyEnv = "GROQ_API_KEY"
		if strings.TrimSpace(cfg.ASR.Model) == "" || strings.HasPrefix(cfg.ASR.Model, "gpt-") {
			cfg.ASR.Model = "whisper-large-v3-turbo"
		}
	}
	if err := cfg.Save(); err != nil {
		return err
	}
	callIPC("reload_config", nil)
	showInfo("API key saved in config and reloaded.")
	return nil
}

func applyASRProviderChecks(local, openai, groq *systray.MenuItem) {
	provider := "local"
	if cfg, err := config.Load(); err == nil {
		provider = strings.ToLower(strings.TrimSpace(cfg.ASR.Provider))
	}
	if provider == "" {
		provider = "local"
	}
	local.Uncheck()
	openai.Uncheck()
	groq.Uncheck()
	switch provider {
	case "openai":
		openai.Check()
	case "groq":
		groq.Check()
	default:
		local.Check()
	}
}

func promptSecret(title string) string {
	if runtime.GOOS != "linux" {
		openConfigPath()
		return ""
	}
	if _, err := exec.LookPath("zenity"); err != nil {
		openConfigPath()
		return ""
	}
	out, err := exec.Command("zenity", "--password", "--title", title).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func showInfo(message string) {
	if runtime.GOOS != "linux" {
		return
	}
	if _, err := exec.LookPath("zenity"); err != nil {
		return
	}
	_ = exec.Command("zenity", "--info", "--text", message).Start()
}

func openConfigPath() {
	path, err := config.ConfigPath()
	if err != nil {
		return
	}
	openPath(path)
}

func openPath(path string) {
	if runtime.GOOS == "linux" {
		exec.Command("xdg-open", path).Start()
	}
}

func onExit() {
}

func openTerminal(cmd string) {
	if runtime.GOOS == "linux" {
		exec.Command("x-terminal-emulator", "-e", "bash", "-c", cmd+"; read").Start()
	}
}

func promptLanguage() string {
	if runtime.GOOS != "linux" {
		return ""
	}
	if _, err := exec.LookPath("zenity"); err != nil {
		return ""
	}
	out, err := exec.Command("zenity", "--entry", "--text=Language code (e.g. ru, en, de, auto)").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func ensureTrayTempDir() {
	if os.Getenv("SNAP") == "" {
		return
	}

	tmp := os.Getenv("TMPDIR")
	if tmp != "" && !strings.HasPrefix(tmp, "/tmp") {
		return
	}

	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir != "" && !strings.HasPrefix(runtimeDir, "/tmp") {
		dir := filepath.Join(runtimeDir, "speak2type")
		if err := os.MkdirAll(dir, 0700); err == nil {
			_ = os.Setenv("TMPDIR", dir)
			return
		}
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil || cacheDir == "" {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			cacheDir = filepath.Join(home, ".cache")
		}
	}
	if cacheDir == "" {
		return
	}

	dir := filepath.Join(cacheDir, "speak2type", "tmp")
	if err := os.MkdirAll(dir, 0700); err == nil {
		_ = os.Setenv("TMPDIR", dir)
	}
}
