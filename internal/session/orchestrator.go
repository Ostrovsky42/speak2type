package session

import (
	"fmt"
	"sync"
	"time"

	"github.com/Ostrovsky42/speak2type/internal/asr"
	"github.com/Ostrovsky42/speak2type/internal/audio"
	"github.com/Ostrovsky42/speak2type/internal/input"
	"github.com/Ostrovsky42/speak2type/internal/merger"
	"github.com/Ostrovsky42/speak2type/internal/vad"
)

// Config holds configuration for the orchestrator.
type Config struct {
	SampleRate int
	ChunkSize  int
}

// Dependencies aggregates the services required by the orchestrator.
type Dependencies struct {
	Audio  *audio.AudioService
	VAD    *vad.VADService
	Gate   *vad.Gate
	ASR    *asr.ASRService
	Merger *merger.MergerService
	Input  *input.KeyboardInjector
}

// EventType defines the type of event emitted by the orchestrator.
type EventType int

const (
	EventFullText    EventType = iota // Final merged text update
	EventStateChange                  // Session state changed
	EventError                        // Something went wrong
)

// Event represents an update from the session.
type Event struct {
	Type      EventType
	Text      string // Committed + Tentative
	Committed string
	Tentative string
	State     State
	Error     error
}

// Orchestrator coordinates the audio pipeline.
type Orchestrator struct {
	mu   sync.Mutex
	deps Dependencies
	conf Config

	state State
	mode  Mode

	events chan Event
	stop   chan struct{} // Signal to stop the main loop

	// Pipeline state
	speechBuffer []float32
	isSpeaking   bool
	silenceStart time.Time // Track silence duration
}

// NewOrchestrator creates a new session orchestrator.
func NewOrchestrator(cfg Config, deps Dependencies) *Orchestrator {
	return &Orchestrator{
		deps:   deps,
		conf:   cfg,
		state:  StateIdle,
		events: make(chan Event, 100),
		stop:   make(chan struct{}),
	}
}

// Events returns the channel for session updates.
func (o *Orchestrator) Events() <-chan Event {
	return o.events
}

// Start launches the coordination loop.
func (o *Orchestrator) Start() {
	go o.loop()
}

// Stop shuts down the orchestrator.
func (o *Orchestrator) Stop() {
	close(o.stop)
}

// StartSession begins a new recording session.
func (o *Orchestrator) StartSession(mode Mode) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.state != StateIdle {
		return fmt.Errorf("session already active (state=%s)", o.state)
	}

	// Reset components
	o.deps.Gate.Reset()
	o.deps.Merger.Reset()
	o.speechBuffer = nil
	o.isSpeaking = false

	// Start Audio
	if err := o.deps.Audio.Start(); err != nil {
		return err
	}

	o.mode = mode
	o.setState(StateListening)
	return nil
}

// StopSession ends the current session.
func (o *Orchestrator) StopSession() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.state != StateListening {
		return
	}

	o.setState(StateProcessing)

	// Stop Audio immediately to stop capturing
	o.deps.Audio.Stop()

	// Flush any remaining speech buffer to ASR
	if len(o.speechBuffer) > 0 {
		o.deps.ASR.Submit(asr.AudioWindow{Samples: o.speechBuffer})
		o.speechBuffer = nil
	}

	// In a real app, we might wait for ASR to drain.
	// For now, let's just transition to Idle after a brief pause or immediately.
	// We'll let the loop handle draining results.

	o.setState(StateIdle)
}

// setState updates the state and emits an event. (Caller must hold lock)
func (o *Orchestrator) setState(s State) {
	o.state = s
	o.events <- Event{
		Type:  EventStateChange,
		State: s,
	}
}

// loop is the main coordination loop.
func (o *Orchestrator) loop() {
	ticker := time.NewTicker(20 * time.Millisecond) // 20ms poll (adjust to chunk size?)
	// Actually, best to poll faster or match chunk size.
	// 512 samples @ 16kHz = 32ms.

	defer ticker.Stop()

	chunk := make([]float32, o.conf.ChunkSize)

	for {
		select {
		case <-o.stop:
			return

		case res := <-o.deps.ASR.Results():
			// Handle ASR output
			if res.Error != nil {
				o.events <- Event{Type: EventError, Error: res.Error}
				continue
			}

			// Empty text handling?
			if res.Text == "" {
				continue
			}

			// Merge
			committed, tentative := o.deps.Merger.Process(res.Text)

			// Inject Input if we have a service and committed text
			if o.deps.Input != nil && committed != "" {
				// We append a space for continuous dictation flow?
				// Or leave it valid. It's safer to type exactly what Merger gives.
				// Merger usually gives "word" or "word word".
				// A space separator is often needed if we are dictating sentences.
				// Let's rely on the Merger to not include trailing spaces, but we might want one.
				// Simple heuristic: always append space?

				// Issue: if we just finished a sentence, we might not want a space?
				// For now: bare inject + space.
				// Use Paste() for reliability with RU/EN. Restore clipboard = true.
				o.deps.Input.Paste(committed+" ", true)
			}

			// Emit update
			full := committed
			if tentative != "" {
				full += " " + tentative
			}

			o.events <- Event{
				Type:      EventFullText,
				Text:      full,
				Committed: committed,
				Tentative: tentative,
				State:     o.state,
			}

		case <-ticker.C:
			// Audio Processing Loop

			// Only process if Listening
			o.mu.Lock()
			state := o.state
			o.mu.Unlock()

			if state != StateListening {
				continue
			}

			// Check buffer availability
			stats := o.deps.Audio.GetStats()
			if stats.BufferStats.Available < o.conf.ChunkSize {
				continue
			}

			// Read Audio
			n := o.deps.Audio.Read(chunk)
			if n < o.conf.ChunkSize {
				continue
			}

			// Run VAD
			prob, err := o.deps.VAD.Process(chunk)
			if err != nil {
				continue // log?
			}

			_, active := o.deps.Gate.Process(prob)

			// Accumulate Speech
			if active {
				if !o.isSpeaking {
					o.isSpeaking = true
					// Speech started
				}

				// Append valid speech to buffer
				// Note: Append creates garbage. Optimization later: pre-allocated arena.
				o.speechBuffer = append(o.speechBuffer, chunk...)

				// Force split if buffer gets too large (e.g. 7s)
				if len(o.speechBuffer) > 16000*7 {
					o.flushAudio()
				}

			} else {
				if o.isSpeaking {
					// Just stopped speaking
					o.isSpeaking = false
					o.silenceStart = time.Now()
				}

				// If we have data and silence timeout passed, submit
				if len(o.speechBuffer) > 0 && time.Since(o.silenceStart) > 500*time.Millisecond {
					o.flushAudio()
				}
			}
		}
	}
}

// flushAudio submits the current buffer to ASR and clears it.
func (o *Orchestrator) flushAudio() {
	if len(o.speechBuffer) == 0 {
		return
	}
	// Copy buffer to avoid race (ASR runs async)
	// Or ASR.Submit can handle copy. Let's assume Submit needs ownership or we copy.
	// ASRService.Submit takes AudioWindow{Samples []float32}.
	// We should copy.
	buf := make([]float32, len(o.speechBuffer))
	copy(buf, o.speechBuffer)

	o.deps.ASR.Submit(asr.AudioWindow{Samples: buf})
	o.speechBuffer = nil
}
