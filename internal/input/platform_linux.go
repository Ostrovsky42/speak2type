//go:build linux

package input

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type linuxInput struct {
	clipboard platformClipboard
}

func newPlatformInput(isWayland bool) (platformInput, error) {
	if err := requireCommand("xdotool"); err != nil {
		return nil, fmt.Errorf("linux input requires xdotool: %w", err)
	}

	clipboard, err := newPlatformClipboard(isWayland)
	if err != nil {
		return nil, err
	}

	return &linuxInput{clipboard: clipboard}, nil
}

func newPlatformClipboard(isWayland bool) (platformClipboard, error) {
	if isWayland {
		if commandAvailable("wl-copy") && commandAvailable("wl-paste") {
			return waylandClipboard{}, nil
		}
		return nil, fmt.Errorf("wayland clipboard requires wl-clipboard (wl-copy and wl-paste)")
	}

	if commandAvailable("xclip") {
		return xclipClipboard{}, nil
	}
	if commandAvailable("xsel") {
		return xselClipboard{}, nil
	}
	return nil, fmt.Errorf("x11 clipboard requires xclip or xsel")
}

func (l *linuxInput) TypeText(text string, delay time.Duration) error {
	delayMS := int(delay / time.Millisecond)
	if delayMS < 0 {
		delayMS = 0
	}
	return runCommand("xdotool", "type", "--clearmodifiers", "--delay", strconv.Itoa(delayMS), "--", text)
}

func (l *linuxInput) PasteShortcut() error {
	return runCommand("xdotool", "key", "--clearmodifiers", "ctrl+v")
}

func (l *linuxInput) ReadClipboard() (string, error) {
	return l.clipboard.Read()
}

func (l *linuxInput) WriteClipboard(text string) error {
	return l.clipboard.Write(text)
}

func (l *linuxInput) ActiveWindow() string {
	title, err := outputCommand("xdotool", "getactivewindow", "getwindowname")
	if err == nil {
		title = strings.TrimSpace(title)
		if title != "" {
			return title
		}
	}

	windowID, err := outputCommand("xdotool", "getactivewindow")
	if err == nil {
		windowID = strings.TrimSpace(windowID)
		if windowID != "" {
			return "XID:" + windowID
		}
	}

	return "unknown"
}

func (l *linuxInput) CheckKeyboardAccess() error {
	if _, err := outputCommand("xdotool", "getactivewindow"); err != nil {
		return fmt.Errorf("xdotool cannot access the active window: %w", err)
	}
	return nil
}

type xclipClipboard struct{}

func (xclipClipboard) Read() (string, error) {
	return outputCommand("xclip", "-selection", "clipboard", "-out")
}

func (xclipClipboard) Write(text string) error {
	return runCommandWithInput(text, "xclip", "-selection", "clipboard", "-in")
}

type xselClipboard struct{}

func (xselClipboard) Read() (string, error) {
	return outputCommand("xsel", "--clipboard", "--output")
}

func (xselClipboard) Write(text string) error {
	return runCommandWithInput(text, "xsel", "--clipboard", "--input")
}

type waylandClipboard struct{}

func (waylandClipboard) Read() (string, error) {
	return outputCommand("wl-paste")
}

func (waylandClipboard) Write(text string) error {
	return runCommandWithInput(text, "wl-copy")
}
