package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Ostrovsky42/speak2type/internal/daemon"
)

// GetXDGPath returns a path in XDG_RUNTIME_DIR or Fallback
func GetXDGPath(subdir, filename string, isState bool) string {
	return daemon.GetXDGPath(subdir, filename, isState)
}

func getPIDPath() string    { return GetXDGPath("speak2type", "speak2type.pid", false) }
func getLockPath() string   { return GetXDGPath("speak2type", "speak2type.lock", false) }
func GetLogPath() string    { return GetXDGPath("speak2type", "speak2type.log", true) }
func GetSocketPath() string { return GetXDGPath("speak2type", "speak2type.sock", false) }

// RunStop finds the running daemon via PID file and kills it.
func RunStop() int {
	pidPath := getPIDPath()
	data, err := os.ReadFile(pidPath)
	if err != nil {
		fmt.Printf("❌ No active Speak2Type process found (checked %s)\n", pidPath)
		return 1
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		fmt.Printf("❌ Invalid PID in %s: %v\n", pidPath, err)
		return 1
	}

	// Double check it's actually speak2type
	// On Linux we can check /proc/<pid>/cmdline
	cmdline, _ := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if !strings.Contains(string(cmdline), "speak2type") {
		fmt.Printf("⚠️  PID %d exists but doesn't look like Speak2Type. Clearing stale PID file.\n", pid)
		os.Remove(pidPath)
		return 1
	}

	fmt.Printf("🛑 Stopping Speak2Type (PID %d)...\n", pid)

	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Printf("❌ Failed to find process %d: %v\n", pid, err)
		os.Remove(pidPath)
		return 1
	}

	// Send SIGTERM
	process.Signal(syscall.SIGTERM)

	// Wait a bit for it to cleanup
	timeWait := 2 * time.Second
	start := time.Now()
	for time.Since(start) < timeWait {
		if err := process.Signal(syscall.Signal(0)); err != nil {
			fmt.Println("✅ Speak2Type stopped.")
			os.Remove(pidPath)
			return 0
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Force kill if still there
	fmt.Println("⚠️  Process didn't stop, sending SIGKILL...")
	process.Signal(syscall.SIGKILL)
	os.Remove(pidPath)
	return 0
}

// RunStatus shows info about the daemon
func RunStatus() int {
	pidPath := getPIDPath()
	data, err := os.ReadFile(pidPath)
	if err != nil {
		fmt.Println("Status: NOT RUNNING")
		return 0
	}

	pidStr := strings.TrimSpace(string(data))
	pid, _ := strconv.Atoi(pidStr)

	process, err := os.FindProcess(pid)
	if err == nil && process.Signal(syscall.Signal(0)) == nil {
		fmt.Printf("Status: RUNNING (PID %d)\n", pid)
		fmt.Printf("Log file: %s\n", GetLogPath())
		// Peek at environment of the process? (Optional, might be complex)
		return 0
	}

	fmt.Printf("Status: STALE (PID %d found in file but process is dead)\n", pid)
	return 0
}

var lockFile *os.File

func AcquireLock() error {
	path := getLockPath()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}

	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		f.Close()
		return fmt.Errorf("speak2type is already running (locked at %s)", path)
	}
	lockFile = f // Keep it open
	return nil
}

func WritePID() error {
	pid := os.Getpid()
	return os.WriteFile(getPIDPath(), []byte(strconv.Itoa(pid)), 0644)
}

func RemovePID() {
	os.Remove(getPIDPath())
	if lockFile != nil {
		syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		lockFile.Close()
		os.Remove(getLockPath())
	}
}

// IsDaemonRunning returns true if the daemon process is active and running.
func IsDaemonRunning() bool {
	pidPath := getPIDPath()
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return false
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return false
	}

	// Double check the process actually exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Signal 0 checks process existence / permissions
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return false
	}

	// Check cmdline to avoid stale PID conflicts
	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return false
	}
	return strings.Contains(string(cmdline), "speak2type")
}

// StartDaemonIfNeeded checks if the daemon is running and, if not, starts it.
func StartDaemonIfNeeded() error {
	if IsDaemonRunning() {
		return nil
	}

	// Find the path to the current executable
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	logPath := GetLogPath()

	cmd := exec.Command(execPath, "run", "--daemon")
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "SPEAK2TYPE_DAEMON=1")

	// Create directories for log file if they don't exist
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		defer logFile.Close()
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to spawn daemon: %w", err)
	}

	// Wait up to 3 seconds for the daemon socket to be initialized
	socketPath := GetSocketPath()
	for i := 0; i < 30; i++ {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}
