package cli

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RunInstallService installs a systemd user service for speak2type.
func RunInstallService() int {
	fmt.Println("🛠 Installing Speak2Type as a systemd user service...")

	unitPath, err := writeUserServiceFile()
	if err != nil {
		fmt.Printf("❌ Failed to write service file: %v\n", err)
		return 1
	}

	fmt.Printf("✅ Service file created at: %s\n", unitPath)
	fmt.Println("\nTo start the service, run:")
	fmt.Println("  systemctl --user daemon-reload")
	fmt.Println("  systemctl --user enable --now speak2type.service")
	fmt.Println("\nTo check logs:")
	fmt.Println("  journalctl --user -u speak2type -f")

	return 0
}

// RunEnableService installs and enables the systemd user service.
func RunEnableService() int {
	unitPath, err := writeUserServiceFile()
	if err != nil {
		fmt.Printf("❌ Failed to write service file: %v\n", err)
		return 1
	}
	fmt.Printf("✅ Service file created at: %s\n", unitPath)

	if err := systemctlUser("daemon-reload"); err != nil {
		fmt.Printf("❌ systemctl daemon-reload failed: %v\n", err)
		return 1
	}
	if err := systemctlUser("enable", "--now", "speak2type.service"); err != nil {
		fmt.Printf("❌ systemctl enable failed: %v\n", err)
		return 1
	}

	fmt.Println("✅ Speak2Type enabled and started.")
	fmt.Println("Logs:")
	fmt.Println("  journalctl --user -u speak2type -f")
	return 0
}

// RunDisableService disables the systemd user service.
func RunDisableService() int {
	if err := systemctlUser("disable", "--now", "speak2type.service"); err != nil {
		fmt.Printf("❌ systemctl disable failed: %v\n", err)
		return 1
	}
	fmt.Println("✅ Speak2Type disabled.")
	return 0
}

func writeUserServiceFile() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	exePath, _ = filepath.Abs(exePath)
	exeDir := filepath.Dir(exePath)

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	unitDir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		return "", err
	}

	unitPath := filepath.Join(unitDir, "speak2type.service")

	display := os.Getenv("DISPLAY")
	xauth := os.Getenv("XAUTHORITY")

	content := fmt.Sprintf(`[Unit]
Description=Speak2Type Voice Input Service
After=network.target sound.target

[Service]
Type=simple
ExecStart=%s run
WorkingDirectory=%s
Restart=always
RestartSec=3
Environment=SPEAK2TYPE_DAEMON=1
Environment=DISPLAY=%s
Environment=XAUTHORITY=%s

[Install]
WantedBy=default.target
`, exePath, exeDir, display, xauth)

	if err := ioutil.WriteFile(unitPath, []byte(content), 0644); err != nil {
		return "", err
	}
	return unitPath, nil
}

func systemctlUser(args ...string) error {
	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(err.Error()))
	}
	return nil
}
