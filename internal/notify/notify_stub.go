//go:build !linux

package notify

import (
	"fmt"
	"time"
)

// Notifier is a no-op stub for non-Linux platforms.
type Notifier struct{}

// New returns an error on unsupported platforms.
func New(appName string) (*Notifier, error) {
	return nil, fmt.Errorf("notifications are only supported on linux")
}

// Notify is a no-op on unsupported platforms.
func (n *Notifier) Notify(summary, body string, urgency uint8, timeout time.Duration) error {
	return fmt.Errorf("notifications are only supported on linux")
}

// Close is a no-op on unsupported platforms.
func (n *Notifier) Close() error {
	return nil
}
