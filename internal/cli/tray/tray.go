package tray

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/getlantern/systray"
	"github.com/Ostrovsky42/speak2type/internal/daemon"
	"github.com/Ostrovsky42/speak2type/internal/ipc"
)

//go:embed assets/idle.png
var iconIdle []byte

//go:embed assets/recording.png
var iconRecording []byte

var (
	client *ipc.Client
	cMu    sync.Mutex
)

func RunTray() int {
	systray.Run(onReady, onExit)
	return 0
}

func onReady() {
	systray.SetIcon(iconIdle)
	systray.SetTitle("Speak2Type")
	systray.SetTooltip("Speak2Type: Offline")

	mToggle := systray.AddMenuItem("Toggle Recording", "Start/Stop recording")
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
					systray.SetTooltip("Speak2Type: Idle")
					// Start listener in a fresh goroutine for this connection
					go listenState(mEn, mUk, mRu, mAuto, mDic, mCom)
				} else {
					systray.SetTooltip("Speak2Type: Offline (Daemon not running)")
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
			case <-mRu.ClickedCh:
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

func listenState(mEn, mUk, mRu, mAuto, mDic, mCom *systray.MenuItem) {
	client.Listen(func(msg ipc.Message) {
		if msg.Event == "state" {
			var info ipc.StateInfo
			if err := json.Unmarshal(msg.Data, &info); err == nil {
				if info.Recording {
					systray.SetIcon(iconRecording)
					systray.SetTooltip("Speak2Type: Recording...")
				} else {
					systray.SetIcon(iconIdle)
					systray.SetTooltip("Speak2Type: Idle")
				}
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
	systray.SetIcon(iconIdle)
	systray.SetTooltip("Speak2Type: Offline")
}

func callIPC(cmd string, params interface{}) {
	cMu.Lock()
	defer cMu.Unlock()
	client.Call(cmd, params)
}

func onExit() {
}

func openTerminal(cmd string) {
	if runtime.GOOS == "linux" {
		exec.Command("x-terminal-emulator", "-e", "bash", "-c", cmd+"; read").Start()
	}
}
