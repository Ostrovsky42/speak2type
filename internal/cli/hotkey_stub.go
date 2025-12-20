//go:build nohook

package cli

import (
	"fmt"
)

type HotkeyListener struct {
	onPress func()
}

func NewHotkeyListener(key string, onPress func()) *HotkeyListener {
	return &HotkeyListener{onPress: onPress}
}

func (h *HotkeyListener) Start() error {
	fmt.Println("⚠️  Global Hotkeys are disabled in this build (missing X11 headers).")
	fmt.Println("💡 Tip: Use 'Enter' in this terminal to toggle recording if supported.")
	return fmt.Errorf("global hotkeys are disabled in this build")
}

func (h *HotkeyListener) Stop() {}
