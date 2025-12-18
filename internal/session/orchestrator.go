package session

import (
	"fmt"
	"sync"
	"time"

	"github.com/Ostrovsky42/speak2type/internal/asr"
	"github.com/Ostrovsky42/speak2type/internal/audio"
	"github.com/Ostrovsky42/speak2type/internal/input"
	"github.com/Ostrovsky42/speak2type/internal/ipc"
	"github.com/Ostrovsky42/speak2type/internal/merger"
	"github.com/Ostrovsky42/speak2type/internal/vad"
)

// Config holds configuration for the orchestrator.
type Config struct {
	SampleRate int
	ChunkSize  int
	NoRestore  bool
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

	state   State
	mode    Mode
	lang    string
	profile ProfileConfig

	events chan Event
	stop   chan struct{} // Signal to stop the main loop

	ipc *ipc.Server

	// Pipeline state
	speechBuffer []float32
	isSpeaking   bool
	silenceStart time.Time // Track silence duration

	pendingASR int // Counter for in-flight ASR jobs

	// Pre-roll buffer (stores recent silence chunks)
	preRollBuffer [][]float32
	preRollLimit  int

	focusWindow string // Identifier of the window active when session started
}

// NewOrchestrator creates a new session orchestrator.
func NewOrchestrator(cfg Config, deps Dependencies) *Orchestrator {
	return &Orchestrator{
		deps:   deps,
		conf:   cfg,
		state:  StateIdle,
		events: make(chan Event, 100),
		stop:   make(chan struct{}),
		// 500ms pre-roll: 500ms / 32ms (approx) = ~16 chunks
		preRollLimit: 16,
		profile:      GetProfile(ProfileDictation),
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

// SetIPC registers an IPC server for broadcasting state
func (o *Orchestrator) SetIPC(s *ipc.Server) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.ipc = s
}

func (o *Orchestrator) SetProfile(t ProfileType) {
	o.mu.Lock()
	defer o.mu.Unlock()

	p := GetProfile(t)
	o.profile = p
	o.deps.Gate.SetConfig(p.VAD)
	o.deps.Merger.SetMinStability(p.MergerMinStability)

	if o.ipc != nil {
		o.ipc.Broadcast("state", o.getIPCStateLocked())
	}
}

// GetIPCState returns the current state in IPC format
func (o *Orchestrator) GetIPCState() ipc.StateInfo {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.getIPCStateLocked()
}

func (o *Orchestrator) getIPCStateLocked() ipc.StateInfo {
	var stateStr string
	switch o.state {
	case StateIdle:
		stateStr = "idle"
	case StateListening:
		stateStr = "listening"
	case StateProcessing:
		stateStr = "processing"
	default:
		stateStr = "unknown"
	}

	return ipc.StateInfo{
		State:       stateStr,
		Recording:   o.state == StateListening,
		Language:    o.lang,
		Profile:     string(o.profile.Name),
		FocusWindow: o.focusWindow,
	}
}

func (o *Orchestrator) Toggle() error {
	o.mu.Lock()
	state := o.state
	o.mu.Unlock()

	if state == StateIdle {
		return o.StartSession(ModeContinuous)
	} else if state == StateListening {
		o.StopSession()
		return nil
	}
	return fmt.Errorf("cannot toggle in state: %s", state)
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
	o.preRollBuffer = nil

	// Capture focus window
	if o.deps.Input != nil {
		o.focusWindow = o.deps.Input.GetActiveWindow()
		fmt.Printf(" [Orch] Focus captured: %s\n", o.focusWindow)
	}

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

	// Flush any remaining speech buffer up to ASR
	o.flushAudioLocked()

	o.setState(StateIdle)
}

// setState updates the state and emits an event. (Caller must hold lock)
func (o *Orchestrator) setState(s State) {
	o.state = s
	evt := Event{
		Type:  EventStateChange,
		State: s,
	}
	o.events <- evt

	if o.ipc != nil {
		o.ipc.Broadcast("state", o.getIPCStateLocked())
	}
}

// loop is the main coordination loop.
func (o *Orchestrator) loop() {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	chunk := make([]float32, o.conf.ChunkSize)

	for {
		select {
		case <-o.stop:
			return

		case res := <-o.deps.ASR.Results():
			if res.Error != nil {
				o.events <- Event{Type: EventError, Error: res.Error}
				continue
			}
			if res.Text == "" {
				continue
			}

			committed, tentative := o.deps.Merger.Process(res.Text)

			if o.deps.Input != nil && committed != "" {
				currentWindow := o.deps.Input.GetActiveWindow()
				if currentWindow == o.focusWindow {
					fmt.Printf(" [Orch] Focus match: %q. Pasting...\n", currentWindow)
					o.deps.Input.Paste(committed+" ", !o.conf.NoRestore)
				} else {
					fmt.Printf(" [Orch] Focus mismatch: %q != %q\n", currentWindow, o.focusWindow)
					o.events <- Event{
						Type:  EventError,
						Text:  committed,
						Error: fmt.Errorf("focus guard: window changed from %q to %q (injection cancelled)", o.focusWindow, currentWindow),
					}
				}
			}

			o.mu.Lock()
			o.pendingASR--
			shouldFlush := (o.state == StateIdle && o.pendingASR == 0)
			o.mu.Unlock()

			if shouldFlush {
				o.flushMerger()
			}

			full := committed
			if tentative != "" {
				full += " " + tentative
			}

			state := o.currentState()
			// Log lang/comm/tent for debugging
			fmt.Printf(" [Orch] Lang: [%s] Comm: %q Tent: %q\n", res.Language, committed, tentative)

			o.events <- Event{
				Type:      EventFullText,
				Text:      full,
				Committed: committed,
				Tentative: tentative,
				State:     state,
			}

		case <-ticker.C:
			o.mu.Lock()
			if o.state != StateListening {
				o.mu.Unlock()
				continue
			}

			stats := o.deps.Audio.GetStats()
			if stats.BufferStats.Available < o.conf.ChunkSize {
				o.mu.Unlock()
				continue
			}

			n := o.deps.Audio.Read(chunk)
			if n < o.conf.ChunkSize {
				o.mu.Unlock()
				continue
			}

			// Run VAD (Can run under lock as it's fast)
			prob, err := o.deps.VAD.Process(chunk)
			if err != nil {
				o.mu.Unlock()
				continue
			}

			_, active := o.deps.Gate.Process(prob)

			if active {
				if !o.isSpeaking {
					fmt.Println(" [Orch] Speech START detected")
					o.isSpeaking = true
					// Prepend pre-roll
					for _, pr := range o.preRollBuffer {
						o.speechBuffer = append(o.speechBuffer, pr...)
					}
					o.preRollBuffer = nil
				}
				o.speechBuffer = append(o.speechBuffer, chunk...)
				if len(o.speechBuffer) > 16000*7 {
					o.flushAudioLocked()
				}
			} else {
				if o.isSpeaking {
					fmt.Println(" [Orch] Speech END detected")
					o.isSpeaking = false
					o.silenceStart = time.Now()
				}

				c := make([]float32, len(chunk))
				copy(c, chunk)
				o.preRollBuffer = append(o.preRollBuffer, c)
				if len(o.preRollBuffer) > o.preRollLimit {
					o.preRollBuffer = o.preRollBuffer[1:]
				}

				if len(o.speechBuffer) > 0 && time.Since(o.silenceStart) > 500*time.Millisecond {
					o.flushAudioLocked()
				}
			}
			o.mu.Unlock()
		}
	}
}

// flushAudio submits the current buffer to ASR and clears it.
func (o *Orchestrator) flushAudio() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.flushAudioLocked()
}

func (o *Orchestrator) flushAudioLocked() {
	if len(o.speechBuffer) == 0 {
		return
	}

	// Copy buffer to avoid race (ASR runs async)
	buf := make([]float32, len(o.speechBuffer))
	copy(buf, o.speechBuffer)

	o.pendingASR++
	o.deps.ASR.Submit(asr.AudioWindow{Samples: buf})
	o.speechBuffer = nil
}

func (o *Orchestrator) flushMerger() {
	flushed := o.deps.Merger.Flush()
	if flushed != "" {
		if o.deps.Input != nil {
			currentWindow := o.deps.Input.GetActiveWindow()
			if currentWindow == o.focusWindow {
				fmt.Printf(" [Orch] Focus match (flush): %q. Pasting...\n", currentWindow)
				o.deps.Input.Paste(flushed+" ", !o.conf.NoRestore)
			} else {
				fmt.Printf(" [Orch] Focus mismatch (flush): %q != %q\n", currentWindow, o.focusWindow)
				o.events <- Event{
					Type:  EventError,
					Text:  flushed,
					Error: fmt.Errorf("focus guard (flush): window changed from %q to %q (injection cancelled)", o.focusWindow, currentWindow),
				}
			}
		}
		o.events <- Event{
			Type:      EventFullText,
			Text:      flushed,
			Committed: flushed,
			State:     StateIdle,
		}
	}
}

func (o *Orchestrator) currentState() State {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.state
}
