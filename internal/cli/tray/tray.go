package tray

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/getlantern/systray"
	"github.com/Ostrovsky42/speak2type/internal/daemon"
	"github.com/Ostrovsky42/speak2type/internal/event"
	"github.com/Ostrovsky42/speak2type/internal/ipc"
	"github.com/Ostrovsky42/speak2type/pkg/config"
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

	systray.AddSeparator()
	mProfile := systray.AddMenuItem("Profile", "Select behavior profile")
	mDic := mProfile.AddSubMenuItemCheckbox("Dictation", "Long pauses, stable", true)
	mCom := mProfile.AddSubMenuItemCheckbox("Commands", "Fast reaction", false)

	systray.AddSeparator()
	mDoctor := systray.AddMenuItem("Run Doctor", "Check system status")
	mLogs := systray.AddMenuItem("Open Logs", "View daemon logs")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit Tray", "Exit tray application")
	mStopDaemon := systray.AddMenuItem("Stop Daemon", "Terminate background process")

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
					go listenIPC(mEn, mUk, mRu, mAuto, mDic, mCom)
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
			case <-mDic.ClickedCh:
				callIPC("set_profile", map[string]string{"profile": "dictation"})
			case <-mCom.ClickedCh:
				callIPC("set_profile", map[string]string{"profile": "commands"})
			case <-mDoctor.ClickedCh:
				openTerminal("speak2type doctor")
			case <-mLogs.ClickedCh:
				openTerminal(fmt.Sprintf("tail -f %s", daemon.GetLogPath()))
			case <-mStopDaemon.ClickedCh:
				callIPC("quit", nil)
			case <-mQuit.ClickedCh:
				systray.Quit()
			}
		}
	}()
}

func listenIPC(mEn, mUk, mRu, mAuto, mDic, mCom *systray.MenuItem) {
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
				case "uk":
					mUk.Check()
					mEn.Uncheck()
					mRu.Uncheck()
					mAuto.Uncheck()
				case "ru":
					mRu.Check()
					mEn.Uncheck()
					mUk.Uncheck()
					mAuto.Uncheck()

				default:
					mAuto.Check()
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
