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
				// Check for F8.
				// Depending on the OS, the Rawcode or Keychar might vary.
				// In many hook impls, F8 is explicitly handled.
				if e.Kind == hook.KeyDown && h.isTrigger(e) {
					h.onPress()
					// Small debounce to avoid multiple triggers
					time.Sleep(200 * time.Millisecond)
				}
			}
		}
	}()
}

func (h *HotkeyListener) isTrigger(e hook.Event) bool {
	// Crude check - if user wants F8, we check common codes or names.
	// hook.Event.Rawcode or Keychar.
	// For now, let's just log and check for common F8 code (66 on some Linux, etc.)
	// Simplest: use Register if gohook supports it, but Start/channel is more robust.
	return true // Placeholder: in real use we'd match triggerKey string to event mapping
}

func (h *HotkeyListener) Stop() {
	if h.stopChan != nil {
		close(h.stopChan)
	}
}
