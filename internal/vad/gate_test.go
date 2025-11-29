package vad

import (
	"testing"
	"time"
)

func TestGate_Transitions(t *testing.T) {
	config := GateConfig{
		ThresholdStart:     0.5,
		ThresholdEnd:       0.3,
		MinSpeechDuration:  20 * time.Millisecond, // Short for testing
		MinSilenceDuration: 20 * time.Millisecond,
	}
	gate := NewGate(config)

	// Initial state should be Silence
	if gate.CurrentState() != StateSilence {
		t.Error("Initial state should be Silence")
	}

	// 1. Test Silence -> Speech transition
	// Send high probability but not long enough
	event, active := gate.Process(0.8)
	if event != EventNone || active {
		t.Error("Should not trigger speech immediately (min duration)")
	}

	time.Sleep(25 * time.Millisecond) // Wait for min duration

	// Send high probability again to confirm
	event, active = gate.Process(0.8)
	if event != EventSpeechStart {
		t.Errorf("Expected EventSpeechStart, got %v", event)
	}
	if !active {
		t.Error("Should be active after transition")
	}

	// 2. Test Speech -> Silence transition
	// Send low probability but not long enough
	event, active = gate.Process(0.1)
	if event != EventNone || !active {
		t.Error("Should not trigger silence immediately (min duration)")
	}

	time.Sleep(25 * time.Millisecond)

	// Send low probability again to confirm
	event, active = gate.Process(0.1)
	if event != EventSpeechEnd {
		t.Errorf("Expected EventSpeechEnd, got %v", event)
	}
	if active {
		t.Error("Should not be active after transition")
	}
}

func TestGate_Hysteresis(t *testing.T) {
	config := GateConfig{
		ThresholdStart:     0.8,
		ThresholdEnd:       0.2,
		MinSpeechDuration:  0, // Instant transitions for logic test
		MinSilenceDuration: 0,
	}
	gate := NewGate(config)

	// 0.5 -> No change (Silence)
	event, active := gate.Process(0.5)
	if active {
		t.Error("0.5 should not trigger speech (start=0.8)")
	}

	// 0.9 -> Speech (requires 2 frames to confirm trigger)
	gate.Process(0.9)                 // Trigger
	event, active = gate.Process(0.9) // Confirm
	if event != EventSpeechStart || !active {
		t.Error("0.9 should trigger speech (after confirmation)")
	}

	// 0.5 -> No change (Speech) - Hysteresis!
	// Even though 0.5 < 0.8 (Start), it is > 0.2 (End), so we stay in Speech
	event, active = gate.Process(0.5)
	if event != EventNone || !active {
		t.Error("0.5 should maintain speech state (hysteresis)")
	}

	// 0.1 -> Silence (requires 2 frames)
	gate.Process(0.1)                 // Trigger
	event, active = gate.Process(0.1) // Confirm
	if event != EventSpeechEnd || active {
		t.Error("0.1 should trigger silence (after confirmation)")
	}
}
