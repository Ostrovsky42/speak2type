//go:build !nohook

package cli

import (
	"fmt"
	"time"

	hook "github.com/robotn/gohook"
)

// HotkeyListener manages global hotkey events.
type HotkeyListener struct {
	triggerKey string
	onPress    func()
	stopChan   chan struct{}
}

func NewHotkeyListener(key string, onPress func()) *HotkeyListener {
	return &HotkeyListener{
		triggerKey: key,
		onPress:    onPress,
		stopChan:   make(chan struct{}),
	}
}

// Start begins listening for the hotkey.
func (h *HotkeyListener) Start() {
	fmt.Printf("⌨️  Global Hotkey Active: [%s]\n", h.triggerKey)

	go func() {
		// Register a hook for the trigger key.
		// hook.Register accepts events like hook.KeyDown.
		// For simplicity, let's assume f8 corresponds to a specific keycode or use Register.

		// robotgo/hook doesn't have a simple "AddEvent(string)" that works like the old one.
		// We use hook.Register(hook.KeyDown, []string{"f8"}, func(e hook.Event) { ... })

		evChan := hook.Start()
		defer hook.End()

		for {
			select {
			case <-h.stopChan:
				return
			case e := <-evChan:
				if e.Kind == hook.KeyDown {
					if h.isTrigger(e) {
						fmt.Printf(" [Hotkey] Triggered: %s\n", h.triggerKey)
						h.onPress()
						// Debounce
						time.Sleep(500 * time.Millisecond)
					}
				}
			}
		}
	}()
}

func (h *HotkeyListener) isTrigger(e hook.Event) bool {
	// On Linux (X11), F8 usually has Rawcode 74 or Keycode 66.
	// We'll check for F8 (66) in Keycode or 74 in Rawcode.
	// This is defensive.
	return (e.Keycode == 66 || e.Rawcode == 74)
}

func (h *HotkeyListener) Stop() {
	if h.stopChan != nil {
		close(h.stopChan)
	}
}
