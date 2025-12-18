package vad

import (
	"time"
)

// GateState represents the current state of the VAD gate.
type GateState int

const (
	StateSilence GateState = iota
	StateSpeech
)

// GateEvent represents a transition event emitted by the gate.
type GateEvent int

const (
	EventNone        GateEvent = iota
	EventSpeechStart           // Transition Silence -> Speech
	EventSpeechEnd             // Transition Speech -> Silence
)

// Gate implements hysteresis logic for stable voice activity detection.
// It filters raw probability outputs to prevent rapid toggling and enforces
// minimum duration constraints for speech and silence.
type Gate struct {
	config GateConfig

	state          GateState
	startTime      time.Time // When the current state started
	lastTransition time.Time // When the last confirmed transition occurred
	triggered      bool      // Tentative trigger state (before min duration)
	triggerTime    time.Time // When the tentative trigger happened
}

// GateConfig defines parameters for the hysteresis gate.
type GateConfig struct {
	ThresholdStart     float32       // Probability to start speech (e.g., 0.5)
	ThresholdEnd       float32       // Probability to end speech (e.g., 0.3)
	MinSpeechDuration  time.Duration // Minimum duration to consider as speech (e.g., 300ms)
	MinSilenceDuration time.Duration // Minimum duration to consider as silence (e.g., 500ms)
}

// DefaultGateConfig returns production-ready defaults.
func DefaultGateConfig() GateConfig {
	return GateConfig{
		ThresholdStart:     0.5,
		ThresholdEnd:       0.35,
		MinSpeechDuration:  300 * time.Millisecond,
		MinSilenceDuration: 500 * time.Millisecond,
	}
}

func (g *Gate) SetConfig(config GateConfig) {
	g.config = config
}

// NewGate creates a new VAD gate with the given configuration.
func NewGate(config GateConfig) *Gate {
	now := time.Now()
	return &Gate{
		config:         config,
		state:          StateSilence,
		startTime:      now,
		lastTransition: now,
	}
}

// Process updates the gate state based on the current speech probability.
// Returns the event (if any) and the current active state.
//
// active: true if speech is currently detected (stable state).
func (g *Gate) Process(probability float32) (GateEvent, bool) {
	now := time.Now()
	event := EventNone

	if g.state == StateSilence {
		// Logic for transitioning Silence -> Speech
		if probability >= g.config.ThresholdStart {
			if !g.triggered {
				// Potential start of speech
				g.triggered = true
				g.triggerTime = now
			} else {
				// Check if we've held the threshold long enough
				duration := now.Sub(g.triggerTime)
				if duration >= g.config.MinSpeechDuration {
					// Confirm transition to Speech
					g.state = StateSpeech
					g.startTime = now
					g.lastTransition = now
					g.triggered = false
					event = EventSpeechStart
				}
			}
		} else {
			// Signal dropped below threshold, reset trigger
			g.triggered = false
		}
	} else {
		// Logic for transitioning Speech -> Silence
		if probability < g.config.ThresholdEnd {
			if !g.triggered {
				// Potential end of speech (start of silence)
				g.triggered = true
				g.triggerTime = now
			} else {
				// Check if we've held the low threshold long enough
				duration := now.Sub(g.triggerTime)
				if duration >= g.config.MinSilenceDuration {
					// Confirm transition to Silence
					g.state = StateSilence
					g.startTime = now
					g.lastTransition = now
					g.triggered = false
					event = EventSpeechEnd
				}
			}
		} else {
			// Signal is strong again, reset silence trigger
			g.triggered = false
		}
	}

	return event, g.state == StateSpeech
}

// Reset resets the gate state to Silence.
func (g *Gate) Reset() {
	now := time.Now()
	g.state = StateSilence
	g.startTime = now
	g.lastTransition = now
	g.triggered = false
}

// CurrentState returns the current stable state.
func (g *Gate) CurrentState() GateState {
	return g.state
}
