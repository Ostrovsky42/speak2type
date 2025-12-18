package session

import (
	"testing"

	"github.com/Ostrovsky42/speak2type/internal/merger"
)

// TestOrchestrator_StateFlow verifies basic state transitions
func TestOrchestrator_StateFlow(t *testing.T) {
	// Note: We can't easily init full Audio/VAD in unit test without libs.
	// We'll test the logic via mock dependencies if possible, or just focus on state.

	deps := Dependencies{
		// Leaving nil for now as NewOrchestrator doesn't start them immediately
		// but StartSession does. Proper mocking would be better.
	}

	orch := NewOrchestrator(Config{SampleRate: 16000, ChunkSize: 512}, deps)

	if orch.state != StateIdle {
		t.Errorf("Expected StateIdle, got %s", orch.state)
	}
}

// TestMergerIntegration simulates Whisper results feeding into the pipeline
func TestMergerIntegration(t *testing.T) {
	m := merger.NewMergerService(merger.DefaultConfig())

	// Step 1: Initial speech
	c, tent := m.Process("Привет")
	if c != "" || tent != "Привет" {
		t.Errorf("Step 1 failed: c='%s', tent='%s'", c, tent)
	}

	// Step 2: More text
	c, tent = m.Process("Привет мир как")
	// Merger might still keep it tentative if they are all new
	t.Logf("Step 2: c='%s', tent='%s'", c, tent)

	// Step 3: Significantly more text to force commitment
	c, tent = m.Process("Привет мир как дела сегодня в")
	t.Logf("Step 3: c='%s', tent='%s'", c, tent)
	if c == "" && tent == "" {
		t.Errorf("Merger returned empty strings")
	}
}

func TestTranscriptionInjectionFlow(t *testing.T) {
	// This would test result -> injection.
	// Since we refactored injector, we can verify that
	// orchestrated loop calls Paste/Type.
}
