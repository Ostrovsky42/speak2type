package session

// State represents the current lifecycle state of the session.
type State int

const (
	// StateIdle: Services are in standby. No audio checks, no VAD.
	StateIdle State = iota

	// StateListening: Audio capture active, VAD active.
	// Speech is being fed to ASR.
	StateListening

	// StateProcessing: User stopped listening, but pipeline is draining
	// remaining buffered audio/text (e.g. finalizing ASR).
	StateProcessing
)

// String returns the string representation of the state.
func (s State) String() string {
	switch s {
	case StateIdle:
		return "Idle"
	case StateListening:
		return "Listening"
	case StateProcessing:
		return "Processing"
	default:
		return "Unknown"
	}
}

// Mode defines the interaction model for the session.
type Mode int

const (
	// ModeQuickNote: Records while hotkey is held, stops on release.
	// Optimizes for short, command-like utterances using "single-shot" VAD logic if needed.
	ModeQuickNote Mode = iota

	// ModeContinuous: Toggles on/off. Records until stopped.
	// Suitable for long dictation.
	ModeContinuous
)
