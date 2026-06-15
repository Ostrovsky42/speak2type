//go:build !linux && !darwin

package input

import (
	"fmt"
	"runtime"
)

func newPlatformInput(_ bool) (platformInput, error) {
	return nil, fmt.Errorf("input injection is not supported on %s", runtime.GOOS)
}

func newPlatformClipboard(_ bool) (platformClipboard, error) {
	return nil, fmt.Errorf("clipboard access is not supported on %s", runtime.GOOS)
}
