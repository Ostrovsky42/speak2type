package logging

import (
	"strings"

	"github.com/Ostrovsky42/speak2type/internal/event"
)

// Level represents logging verbosity.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Parse converts a string into a Level. Defaults to info.
func Parse(value string) Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return LevelDebug
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// FromEventLevel maps an event level to a log level.
func FromEventLevel(level event.Level) Level {
	switch level {
	case event.LevelWarn:
		return LevelWarn
	case event.LevelError:
		return LevelError
	default:
		return LevelInfo
	}
}

// Allowed reports whether a message with level msg is allowed at threshold.
func Allowed(threshold, msg Level) bool {
	return msg >= threshold
}
