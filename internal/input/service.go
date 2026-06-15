package input

import (
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"time"
)

// KeyboardInjector handles keyboard input simulation & text injection.
type KeyboardInjector struct {
	mu        sync.Mutex
	enabled   bool
	isWayland bool
	conf      Config
	platform  platformInput
}

var ErrInjectionDisabled = errors.New("input injection disabled")

type platformInput interface {
	TypeText(text string, delay time.Duration) error
	PasteShortcut() error
	ReadClipboard() (string, error)
	WriteClipboard(text string) error
	ActiveWindow() string
	CheckKeyboardAccess() error
}

type platformClipboard interface {
	Read() (string, error)
	Write(text string) error
}

// Config for KeyboardInjector
type Config struct {
	TypingSpeed  time.Duration // Default: 10ms
	Enabled      bool          // Default: true
	ForceWayland bool          // Allow injection on Wayland despite risks
	DryRun       bool          // If true, only log actions, don't execute
	FocusDelay   time.Duration // Delay before paste (e.g. 100ms)
	SettleDelay  time.Duration // Delay after write before Ctrl+V (e.g. 100ms)
	PasteDelay   time.Duration // Delay after Ctrl+V before restore (e.g. 200ms)
}

// NewKeyboardInjector creates a new input injector.
func NewKeyboardInjector(cfg Config) (*KeyboardInjector, error) {
	isWayland := isWaylandSession()

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

	if injector.enabled && !cfg.DryRun {
		platform, err := newPlatformInput(isWayland)
		if err != nil {
			return nil, err
		}
		injector.platform = platform
	}

	return injector, nil
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
	if err := s.ensurePlatform(); err != nil {
		return err
	}

	log.Printf("⌨️  Typing text: %q", text)
	delay := s.conf.TypingSpeed
	if delay <= 0 {
		delay = 10 * time.Millisecond
	}
	if err := s.platform.TypeText(text, delay); err != nil {
		return fmt.Errorf("failed to type text: %w", err)
	}
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
	if err := s.ensurePlatform(); err != nil {
		return err
	}

	// 0. Focus Delay
	if s.conf.FocusDelay > 0 {
		time.Sleep(s.conf.FocusDelay)
	}

	// 1. Save current clipboard if requested
	var originalContent string
	var err error
	if restoreClipboard {
		originalContent, err = s.platform.ReadClipboard()
		if err != nil {
			log.Printf("⚠️  Failed to read clipboard for restore: %v", err)
			restoreClipboard = false
		}
	}

	log.Printf("📋 Pasting text: %q", text)

	// 2. Set new content
	if err := s.platform.WriteClipboard(text); err != nil {
		return fmt.Errorf("failed to write clipboard: %w", err)
	}

	// 3. Settle delay
	if s.conf.SettleDelay > 0 {
		time.Sleep(s.conf.SettleDelay)
	} else {
		time.Sleep(150 * time.Millisecond) // Slightly more conservative
	}

	// 4. Trigger Paste (Ctrl+V / Cmd+V)
	if err := s.platform.PasteShortcut(); err != nil {
		return fmt.Errorf("failed to send paste shortcut: %w", err)
	}

	// 5. Paste delay
	if s.conf.PasteDelay > 0 {
		time.Sleep(s.conf.PasteDelay)
	} else {
		time.Sleep(300 * time.Millisecond) // Slightly more conservative
	}

	// 6. Restore original content
	if restoreClipboard {
		if err := s.platform.WriteClipboard(originalContent); err != nil {
			log.Printf("⚠️  Failed to restore original clipboard content: %v", err)
		} else {
			log.Println("📋 Restored original clipboard content")
		}
	}

	return nil
}

func (s *KeyboardInjector) ensurePlatform() error {
	if s.platform != nil {
		return nil
	}
	platform, err := newPlatformInput(s.isWayland)
	if err != nil {
		return err
	}
	s.platform = platform
	return nil
}

// GetActiveWindow returns a string identifying the current foreground window.
func (s *KeyboardInjector) GetActiveWindow() string {
	if s == nil || s.platform == nil {
		return "unknown"
	}
	return s.platform.ActiveWindow()
}

// Enable toggles injection functionality.
func (s *KeyboardInjector) Enable(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = v
}

// CheckClipboardAccess verifies that the platform clipboard backend can read and write.
func CheckClipboardAccess() error {
	clipboard, err := newPlatformClipboard(isWaylandSession())
	if err != nil {
		return err
	}

	original, err := clipboard.Read()
	if err != nil {
		return fmt.Errorf("failed to read clipboard: %w", err)
	}
	if err := clipboard.Write(original); err != nil {
		return fmt.Errorf("failed to write clipboard: %w", err)
	}
	return nil
}

// CheckKeyboardAccess verifies that the platform keyboard backend is reachable.
func CheckKeyboardAccess() error {
	platform, err := newPlatformInput(isWaylandSession())
	if err != nil {
		return err
	}
	return platform.CheckKeyboardAccess()
}

func isWaylandSession() bool {
	return os.Getenv("XDG_SESSION_TYPE") == "wayland"
}
