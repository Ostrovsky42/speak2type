//go:build darwin

package input

import (
	"fmt"
	"strings"
	"time"
)

type darwinInput struct{}

type darwinClipboard struct{}

func newPlatformInput(_ bool) (platformInput, error) {
	if err := requireCommand("osascript"); err != nil {
		return nil, fmt.Errorf("macOS input requires osascript: %w", err)
	}
	if _, err := newPlatformClipboard(false); err != nil {
		return nil, err
	}
	return &darwinInput{}, nil
}

func newPlatformClipboard(_ bool) (platformClipboard, error) {
	for _, command := range []string{"pbcopy", "pbpaste"} {
		if err := requireCommand(command); err != nil {
			return nil, fmt.Errorf("macOS clipboard requires %s: %w", command, err)
		}
	}
	return darwinClipboard{}, nil
}

func (d *darwinInput) TypeText(text string, _ time.Duration) error {
	return runAppleScript(
		`tell application "System Events"`,
		"keystroke "+appleScriptQuote(text),
		"end tell",
	)
}

func (d *darwinInput) PasteShortcut() error {
	return runAppleScript(
		`tell application "System Events"`,
		"key code 9 using command down",
		"end tell",
	)
}

func (d *darwinInput) ReadClipboard() (string, error) {
	return darwinClipboard{}.Read()
}

func (d *darwinInput) WriteClipboard(text string) error {
	return darwinClipboard{}.Write(text)
}

func (darwinClipboard) Read() (string, error) {
	return outputCommand("pbpaste")
}

func (darwinClipboard) Write(text string) error {
	return runCommandWithInput(text, "pbcopy")
}

func (d *darwinInput) ActiveWindow() string {
	window, err := outputAppleScript(activeWindowScript()...)
	if err != nil {
		return "unknown"
	}
	window = strings.TrimSpace(window)
	if window == "" {
		return "unknown"
	}
	return window
}

func (d *darwinInput) CheckKeyboardAccess() error {
	if _, err := outputAppleScript(activeWindowScript()...); err != nil {
		return fmt.Errorf("macOS Accessibility permission is required for input injection: %w", err)
	}
	return nil
}

func activeWindowScript() []string {
	return []string{
		`tell application "System Events"`,
		`set frontApp to first application process whose frontmost is true`,
		`set appName to name of frontApp`,
		`set winTitle to ""`,
		`try`,
		`set winTitle to name of front window of frontApp`,
		`end try`,
		`return appName & ":" & winTitle`,
		`end tell`,
	}
}

func runAppleScript(lines ...string) error {
	_, err := outputAppleScript(lines...)
	return err
}

func outputAppleScript(lines ...string) (string, error) {
	args := make([]string, 0, len(lines)*2)
	for _, line := range lines {
		args = append(args, "-e", line)
	}
	return outputCommand("osascript", args...)
}

func appleScriptQuote(text string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + replacer.Replace(text) + `"`
}
