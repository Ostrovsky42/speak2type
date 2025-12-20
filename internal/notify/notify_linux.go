//go:build linux

package notify

import (
	"time"

	"github.com/godbus/dbus/v5"
)

// Notifier sends desktop notifications over DBus.
type Notifier struct {
	conn    *dbus.Conn
	appName string
}

// New creates a DBus notifier for the current session.
func New(appName string) (*Notifier, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, err
	}
	return &Notifier{conn: conn, appName: appName}, nil
}

// Notify sends a notification with a timeout (ms) and urgency hint.
func (n *Notifier) Notify(summary, body string, urgency uint8, timeout time.Duration) error {
	if n == nil || n.conn == nil {
		return dbus.ErrClosed
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	hints := map[string]dbus.Variant{
		"urgency": dbus.MakeVariant(urgency),
	}

	obj := n.conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	call := obj.Call("org.freedesktop.Notifications.Notify", 0,
		n.appName,
		uint32(0),
		"",
		summary,
		body,
		[]string{},
		hints,
		int32(timeout.Milliseconds()),
	)
	return call.Err
}

// Close releases the DBus connection.
func (n *Notifier) Close() error {
	if n == nil || n.conn == nil {
		return nil
	}
	return n.conn.Close()
}
