package event

import "time"

// Type defines the kind of domain event.
type Type string

// Level defines the severity level for an event.
type Level string

// State defines the UX state visible to users.
type State string

const (
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

const (
	StateIdle         State = "idle"
	StateRecording    State = "recording"
	StateTranscribing State = "transcribing"
	StateInjecting    State = "injecting"
	StateDone         State = "done"
	StateError        State = "error"
)

const (
	TypeAppStarted            Type = "app_started"
	TypeHotkeyRegistered      Type = "hotkey_registered"
	TypeRecordingStarted      Type = "recording_started"
	TypeRecordingStopped      Type = "recording_stopped"
	TypeTranscriptionStarted  Type = "transcription_started"
	TypeTranscriptionFinished Type = "transcription_finished"
	TypeInjectionStarted      Type = "injection_started"
	TypeInjectionFinished     Type = "injection_finished"
	TypeDone                  Type = "done"
	TypeError                 Type = "error"
	TypeSubscriberDropped     Type = "subscriber_dropped"
)

// Event captures a single UX-relevant milestone.
type Event struct {
	ID    uint64    `json:"id"`
	Type  Type      `json:"type"`
	Time  time.Time `json:"time"`
	Level Level     `json:"level"`
	State State     `json:"state"`

	TextLen   int    `json:"text_len,omitempty"`
	Hotkey    string `json:"hotkey,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
	Message   string `json:"message,omitempty"`
	Hint      string `json:"hint,omitempty"`
}

// Publisher is implemented by types that can publish events.
type Publisher interface {
	Publish(Event)
}
