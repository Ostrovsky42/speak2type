package cli

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// RunInstallService installs a systemd user service for speak2type.
func RunInstallService() int {
	fmt.Println("🛠 Installing Speak2Type as a systemd user service...")

	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("❌ Failed to get executable path: %v\n", err)
		return 1
	}
	exePath, _ = filepath.Abs(exePath)

	home := os.Getenv("HOME")
	unitDir := filepath.Join(home, ".config", "systemd", "user")
	os.MkdirAll(unitDir, 0755)

	unitPath := filepath.Join(unitDir, "speak2type.service")

	// Create service content
	content := fmt.Sprintf(`[Unit]
Description=Speak2Type Voice Input Service
After=network.target sound.target

[Service]
Type=simple
ExecStart=%s run
Restart=always
RestartSec=3
Environment=DISPLAY=%s
Environment=XAUTHORITY=%s

[Install]
WantedBy=default.target
`, exePath, os.Getenv("DISPLAY"), os.Getenv("XAUTHORITY"))

	err = ioutil.WriteFile(unitPath, []byte(content), 0644)
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
