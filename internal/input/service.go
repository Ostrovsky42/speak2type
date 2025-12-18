package input

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/go-vgo/robotgo"
)

// KeyboardInjector handles keyboard input simulation & text injection.
type KeyboardInjector struct {
	mu          sync.Mutex
	typingSpeed time.Duration
	enabled     bool
	isWayland   bool
}

// Config for KeyboardInjector
type Config struct {
	TypingSpeed  time.Duration // Default: 10ms
	Enabled      bool          // Default: true
	ForceWayland bool          // Allow injection on Wayland despite risks
}

// NewKeyboardInjector creates a new input injector.
func NewKeyboardInjector(cfg Config) (*KeyboardInjector, error) {
	sessionType := os.Getenv("XDG_SESSION_TYPE")
	isWayland := sessionType == "wayland"

	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && !isWayland {
		return nil, fmt.Errorf("DISPLAY not set and not in Wayland. Input injection impossible.")
	}

	injector := &KeyboardInjector{
		typingSpeed: cfg.TypingSpeed,
		enabled:     cfg.Enabled,
		isWayland:   isWayland,
	}

	if isWayland && !cfg.ForceWayland {
		log.Println("⚠️  Wayland detected. Input injection DISABLED for safety.")
		injector.enabled = false
	} else if isWayland && cfg.ForceWayland {
		log.Println("⚠️  Wayland detected. Input injection FORCED (experimental).")
	}

	return injector, nil
}

// Type simulates character-by-character typing.
func (s *KeyboardInjector) Type(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.enabled {
		return nil
	}

	log.Printf("⌨️  Typing text: %q", text)
	robotgo.TypeStr(text)
	return nil
}

// Paste uses clipboard + Ctrl/Cmd+V to inject text instantly.
// Accepts restoreClipboard boolean to optionally preserve user's clipboard.
func (s *KeyboardInjector) Paste(text string, restoreClipboard bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.enabled {
		return nil
	}

	if s.isWayland {
		log.Println("⚠️  Wayland detected. Clipboard injection might be unreliable.")
	}

	// 1. Save current clipboard if requested
	var originalContent string
	var err error
	if restoreClipboard {
		// ReadAll might fail or be empty, that's fine.
		// On Linux this uses xclip/xsel.
		originalContent, err = robotgo.ReadAll()
		if err != nil {
			log.Printf("⚠️  Failed to read clipboard for restore: %v", err)
			restoreClipboard = false // Disable restore if read failed
		}
	}

	log.Printf("📋 Pasting text: %q", text)

	// 2. Set new content
	robotgo.WriteAll(text)

	// Wait a tiny bit for clipboard sync/manager to pick it up
	time.Sleep(100 * time.Millisecond)

	// 3. Trigger Paste (Ctrl+V / Cmd+V)
	if runtime.GOOS == "darwin" {
		robotgo.KeyTap("v", "command")
	} else {
		robotgo.KeyTap("v", "control")
	}

	// Wait for paste to likely complete before restoring
	time.Sleep(200 * time.Millisecond)

	// 4. Restore original content
	if restoreClipboard {
		robotgo.WriteAll(originalContent)
		log.Println("📋 Restored original clipboard content")
	}

	return nil
}

// Enable toggles injection functionality.
func (s *KeyboardInjector) Enable(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = v
}
