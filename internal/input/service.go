package input

import (
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"runtime"
	"sync"
	"time"

	"github.com/go-vgo/robotgo"
)

// KeyboardInjector handles keyboard input simulation & text injection.
type KeyboardInjector struct {
	mu           sync.Mutex
	enabled      bool
	isWayland    bool
	conf         Config
	targetWindow *targetWindow
}

type targetWindow struct {
	handle robotgo.Handle
	label  string
}

var ErrInjectionDisabled = errors.New("input injection disabled")

// Config for KeyboardInjector
type Config struct {
	TypingSpeed  time.Duration // Default: 10ms
	Enabled      bool          // Default: true
	ForceWayland bool          // Allow injection on Wayland despite risks
	DryRun       bool          // If true, only log actions, don't execute
	FocusDelay   time.Duration // Delay before paste (e.g. 100ms)
	SettleDelay  time.Duration // Delay after write before Ctrl+V (e.g. 100ms)
	PasteDelay   time.Duration // Delay after Ctrl+V before restore (e.g. 700ms)
}

// NewKeyboardInjector creates a new input injector.
func NewKeyboardInjector(cfg Config) (*KeyboardInjector, error) {
	sessionType := os.Getenv("XDG_SESSION_TYPE")
	isWayland := sessionType == "wayland"

	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && !isWayland {
		return nil, fmt.Errorf("DISPLAY not set and not in Wayland: input injection impossible")
	}

	injector := &KeyboardInjector{
		enabled:   cfg.Enabled,
		isWayland: isWayland,
		conf:      cfg,
	}

	if isWayland && !cfg.ForceWayland {
		log.Println("⚠️  Wayland detected. Input injection DISABLED for safety.")
		injector.enabled = false
	} else if isWayland && cfg.ForceWayland {
		log.Println("⚠️  Wayland detected. Input injection FORCED (experimental).")
	}

	return injector, nil
}

// CaptureTargetWindow stores the active window so later paste can return focus to it.
func (s *KeyboardInjector) CaptureTargetWindow() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	label := activeWindowLabel()
	s.targetWindow = &targetWindow{
		handle: robotgo.GetActive(),
		label:  label,
	}
	return label
}

// FocusTargetWindow activates the window captured at recording start.
func (s *KeyboardInjector) FocusTargetWindow() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.targetWindow == nil {
		return nil
	}
	if !s.enabled {
		return ErrInjectionDisabled
	}
	robotgo.SetActive(s.targetWindow.handle)
	time.Sleep(120 * time.Millisecond)
	return nil
}

// IsTargetWindowActive reports whether the captured target window is active now.
func (s *KeyboardInjector) IsTargetWindowActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.targetWindow == nil {
		return true
	}
	return reflect.DeepEqual(robotgo.GetActive(), s.targetWindow.handle)
}

func activeWindowLabel() string {
	title := robotgo.GetTitle()
	if title != "" {
		return title
	}
	return fmt.Sprintf("PID:%d", robotgo.GetPid())
}

// Type simulates character-by-character typing.
func (s *KeyboardInjector) Type(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conf.DryRun {
		log.Printf("[DRY-RUN] Would type: %q", text)
		return nil
	}

	if !s.enabled {
		return ErrInjectionDisabled
	}

	log.Printf("⌨️  Typing text: %q", text)
	robotgo.Type(text)
	return nil
}

// Paste uses clipboard + Ctrl/Cmd+V to inject text instantly.
// Accepts restoreClipboard boolean to optionally preserve user's clipboard.
func (s *KeyboardInjector) Paste(text string, restoreClipboard bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conf.DryRun {
		log.Printf("[DRY-RUN] Would paste: %q (restore=%v)", text, restoreClipboard)
		return nil
	}

	if !s.enabled {
		return ErrInjectionDisabled
	}

	// 0. Focus Delay
	if s.conf.FocusDelay > 0 {
		time.Sleep(s.conf.FocusDelay)
	}

	// 1. Save current clipboard if requested
	var originalContent string
	var err error
	if restoreClipboard {
		originalContent, err = robotgo.ReadAll()
		if err != nil {
			log.Printf("⚠️  Failed to read clipboard for restore: %v", err)
			restoreClipboard = false
		}
	}

	log.Printf("📋 Pasting text: %q", text)

	// 2. Set new content
	robotgo.WriteAll(text)

	// 3. Settle delay
	if s.conf.SettleDelay > 0 {
		time.Sleep(s.conf.SettleDelay)
	} else {
		time.Sleep(150 * time.Millisecond) // Slightly more conservative
	}

	// 4. Trigger Paste (Ctrl+V / Cmd+V)
	if runtime.GOOS == "darwin" {
		robotgo.KeyDown("command")
		time.Sleep(20 * time.Millisecond)
		robotgo.KeyTap("v")
		time.Sleep(20 * time.Millisecond)
		robotgo.KeyUp("command")
	} else {
		robotgo.KeyDown("control")
		time.Sleep(20 * time.Millisecond)
		robotgo.KeyTap("v")
		time.Sleep(20 * time.Millisecond)
		robotgo.KeyUp("control")
	}

	// 5. Paste delay
	if s.conf.PasteDelay > 0 {
		time.Sleep(s.conf.PasteDelay)
	} else {
		time.Sleep(700 * time.Millisecond) // Give X11/Wayland clients time to request clipboard data
	}

	// 6. Restore original content
	if restoreClipboard {
		robotgo.WriteAll(originalContent)
		log.Println("📋 Restored original clipboard content")
	}

	return nil
}

// GetActiveWindow returns a string identifying the current foreground window.
// It uses title or PID depending on what robotgo provides best.
func (s *KeyboardInjector) GetActiveWindow() string {
	return activeWindowLabel()
}

// Enable toggles injection functionality.
func (s *KeyboardInjector) Enable(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = v
}
