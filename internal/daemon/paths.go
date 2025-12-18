package daemon

import (
	"os"
	"path/filepath"
)

// GetXDGPath returns a path in XDG_RUNTIME_DIR or Fallback
func GetXDGPath(subdir, filename string, isState bool) string {
	var base string
	if isState {
		base = os.Getenv("XDG_STATE_HOME")
		if base == "" {
			base = filepath.Join(os.Getenv("HOME"), ".local", "state")
		}
	} else {
		base = os.Getenv("XDG_RUNTIME_DIR")
		if base == "" {
			base = filepath.Join(os.Getenv("HOME"), ".cache") // Fallback
		}
	}

	dir := filepath.Join(base, subdir)
	os.MkdirAll(dir, 0700)
	return filepath.Join(dir, filename)
}

func getPIDPath() string  { return GetXDGPath("speak2type", "speak2type.pid", false) }
func getLockPath() string { return GetXDGPath("speak2type", "speak2type.lock", false) }

func GetLogPath() string    { return GetXDGPath("speak2type", "speak2type.log", true) }
func GetSocketPath() string { return GetXDGPath("speak2type", "speak2type.sock", false) }
func GetPIDPath() string    { return getPIDPath() }
func GetLockPath() string   { return getLockPath() }
