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

func (h *HotkeyListener) Start() {
	fmt.Println("⚠️  Global Hotkeys are disabled in this build (missing X11 headers).")
	fmt.Println("💡 Tip: Use 'Enter' in this terminal to toggle recording if supported.")
}

func (h *HotkeyListener) Stop() {}
