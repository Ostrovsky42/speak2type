package input

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func commandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func requireCommand(name string) error {
	if commandAvailable(name) {
		return nil
	}
	return fmt.Errorf("%s not found in PATH", name)
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return commandError(name, err, out)
	}
	return nil
}

func runCommandWithInput(input, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return commandError(name, err, out)
	}
	return nil
}

func outputCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	if err != nil {
		var stderr []byte
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = exitErr.Stderr
		}
		return "", commandError(name, err, stderr)
	}
	return string(out), nil
}

func commandError(name string, err error, output []byte) error {
	msg := string(bytes.TrimSpace(output))
	if msg == "" {
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return fmt.Errorf("%s failed: %w: %s", name, err, msg)
}
