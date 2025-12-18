package cli

import (
	"fmt"
	"io/ioutil"
	"os"
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
	data, err := ioutil.ReadFile(pidPath)
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
	cmdline, _ := ioutil.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
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
	data, err := ioutil.ReadFile(pidPath)
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
	return ioutil.WriteFile(getPIDPath(), []byte(strconv.Itoa(pid)), 0644)
}

func RemovePID() {
	os.Remove(getPIDPath())
	if lockFile != nil {
		syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		lockFile.Close()
		os.Remove(getLockPath())
	}
}
