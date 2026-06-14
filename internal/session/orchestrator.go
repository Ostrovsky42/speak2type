package session

import (
	"fmt"
	"sync"
	"time"

	"github.com/Ostrovsky42/speak2type/internal/asr"
	"github.com/Ostrovsky42/speak2type/internal/audio"
	"github.com/Ostrovsky42/speak2type/internal/event"
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
	bus event.Publisher

	// Pipeline state
	speechBuffer []float32
	isSpeaking   bool
	silenceStart time.Time // Track silence duration

	pendingASR int // Counter for in-flight ASR jobs

	committedLen              int
	transcriptionFinished     bool
	processingInjectionFailed bool
	inputUnavailableReported  bool

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

// SetEventBus registers a publisher for UX events.
func (o *Orchestrator) SetEventBus(p event.Publisher) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.bus = p
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

// SetLanguage updates the current ASR language and broadcasts state.
func (o *Orchestrator) SetLanguage(lang string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.lang = lang
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
		PendingASR:  o.pendingASR,
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

	if o.pendingASR != 0 {
		return fmt.Errorf("cannot start session: ASR still draining (%d pending)", o.pendingASR)
	}

	// Reset components
	o.deps.Gate.Reset()
	o.deps.Merger.Reset()
	o.speechBuffer = nil
	o.isSpeaking = false
	o.preRollBuffer = nil
	o.committedLen = 0
	o.transcriptionFinished = false
	o.processingInjectionFailed = false
	o.inputUnavailableReported = false

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
	o.publishEvent(event.Event{
		Type:  event.TypeRecordingStarted,
		Level: event.LevelInfo,
		State: event.StateRecording,
	})
	return nil
}

// StopSession ends the current session.
func (o *Orchestrator) StopSession() {
	o.mu.Lock()
	if o.state != StateListening {
		o.mu.Unlock()
		return
	}

	o.setState(StateProcessing)
	o.publishEvent(event.Event{
		Type:  event.TypeRecordingStopped,
		Level: event.LevelInfo,
		State: event.StateTranscribing,
	})
	o.publishEvent(event.Event{
		Type:  event.TypeTranscriptionStarted,
		Level: event.LevelInfo,
		State: event.StateTranscribing,
	})
	o.mu.Unlock()

	// Stop Audio immediately to stop capturing
	_ = o.deps.Audio.Stop()

	// Flush any remaining speech buffer up to ASR
	o.mu.Lock()
	o.flushAudioLocked()
	done := o.pendingASR == 0
	o.mu.Unlock()

	if done {
		o.finishProcessing()
	}
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

func (o *Orchestrator) publishEvent(evt event.Event) {
	if o.bus == nil {
		return
	}
	o.bus.Publish(evt)
}

func (o *Orchestrator) publishError(code, message, hint string) {
	o.publishEvent(event.Event{
		Type:      event.TypeError,
		Level:     event.LevelError,
		State:     event.StateError,
		ErrorCode: code,
		Message:   message,
		Hint:      hint,
	})
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

		case res, ok := <-o.deps.ASR.Results():
			if !ok {
				err := fmt.Errorf("ASR results channel closed")
				o.events <- Event{Type: EventError, Error: err}
				o.publishError("asr_results_closed", err.Error(), "restart the daemon")
				return
			}
			if res.Error != nil {
				o.events <- Event{Type: EventError, Error: res.Error}
				o.publishError("asr_error", res.Error.Error(), "check models and CPU compatibility")
			}
			if res.Text != "" {
				committed, tentative := o.deps.Merger.Process(res.Text)

				if committed != "" {
					o.addCommittedLen(len(committed))
					o.injectText(committed)
				}

				// Log lang/comm/tent for debugging
				fmt.Printf(" [Orch] Lang: [%s] Comm: %q Tent: %q\n", res.Language, committed, tentative)

				if committed != "" || tentative != "" {
					o.emitFullText(committed, tentative)
				}
			}

			shouldFinalize := false
			o.mu.Lock()
			if o.pendingASR > 0 {
				o.pendingASR--
			}
			shouldFinalize = o.state == StateProcessing && o.pendingASR == 0
			o.mu.Unlock()

			if shouldFinalize {
				o.finishProcessing()
			}

		case <-ticker.C:
			o.mu.Lock()
			if o.state != StateListening {
				o.mu.Unlock()
				continue
			}
			o.mu.Unlock()

			stats := o.deps.Audio.GetStats()
			if stats.BufferStats.Available < o.conf.ChunkSize {
				continue
			}

			n := o.deps.Audio.Read(chunk)
			if n < o.conf.ChunkSize {
				continue
			}

			// Run VAD (avoid holding orchestrator lock; ONNX can stall)
			prob, err := o.deps.VAD.Process(chunk)
			if err != nil {
				continue
			}

			_, active := o.deps.Gate.Process(prob)

			o.mu.Lock()
			if o.state != StateListening {
				o.mu.Unlock()
				continue
			}

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

// flushAudioLocked submits the current buffer to ASR and clears it.

func (o *Orchestrator) flushAudioLocked() {
	if len(o.speechBuffer) == 0 {
		return
	}

	// Copy buffer to avoid race (ASR runs async)
	buf := make([]float32, len(o.speechBuffer))
	copy(buf, o.speechBuffer)

	dropped, err := o.deps.ASR.Submit(asr.AudioWindow{Samples: buf})
	if dropped > 0 {
		if dropped > o.pendingASR {
			o.pendingASR = 0
		} else {
			o.pendingASR -= dropped
		}
	}
	if err == nil {
		o.pendingASR++
	} else {
		o.events <- Event{Type: EventError, Error: err}
		o.publishError("asr_submit_failed", err.Error(), "check model files and available memory")
	}
	o.speechBuffer = nil
}

func (o *Orchestrator) addCommittedLen(n int) {
	if n <= 0 {
		return
	}
	o.mu.Lock()
	o.committedLen += n
	o.mu.Unlock()
}

func (o *Orchestrator) emitFullText(committed, tentative string) {
	full := committed
	if tentative != "" {
		if full != "" {
			full += " "
		}
		full += tentative
	}
	if full == "" {
		return
	}
	state := o.currentState()
	o.events <- Event{
		Type:      EventFullText,
		Text:      full,
		Committed: committed,
		Tentative: tentative,
		State:     state,
	}
}

func (o *Orchestrator) markInjectionFailed() {
	o.mu.Lock()
	o.processingInjectionFailed = true
	o.mu.Unlock()
}

func (o *Orchestrator) injectText(text string) {
	if text == "" {
		return
	}

	state := o.currentState()
	emitState := event.State("")
	if state == StateProcessing {
		emitState = event.StateInjecting
	}

	o.publishEvent(event.Event{
		Type:  event.TypeInjectionStarted,
		Level: event.LevelInfo,
		State: emitState,
	})

	if o.deps.Input == nil {
		o.mu.Lock()
		shouldReport := !o.inputUnavailableReported
		if shouldReport {
			o.inputUnavailableReported = true
		}
		o.mu.Unlock()
		if shouldReport {
			o.publishError("inject_unavailable", "input injector unavailable", "run on X11 or enable injection")
		}
		if state == StateProcessing {
			o.markInjectionFailed()
		}
		return
	}

	currentWindow := o.deps.Input.GetActiveWindow()
	if currentWindow != o.focusWindow {
		errMsg := fmt.Sprintf("focus guard: window changed from %q to %q (injection cancelled)", o.focusWindow, currentWindow)
		o.events <- Event{
			Type:  EventError,
			Text:  text,
			Error: fmt.Errorf("%s", errMsg),
		}
		o.publishError("focus_guard", errMsg, "refocus the target window and retry")
		if state == StateProcessing {
			o.markInjectionFailed()
		}
		return
	}

	if err := o.deps.Input.Paste(text+" ", !o.conf.NoRestore); err != nil {
		o.publishError("inject_failed", err.Error(), "check permissions and active window focus")
		if state == StateProcessing {
			o.markInjectionFailed()
		}
		return
	}

	o.publishEvent(event.Event{
		Type:  event.TypeInjectionFinished,
		Level: event.LevelInfo,
		State: emitState,
	})
}

func (o *Orchestrator) emitTranscriptionFinished() {
	o.mu.Lock()
	if o.transcriptionFinished {
		o.mu.Unlock()
		return
	}
	o.transcriptionFinished = true
	textLen := o.committedLen
	o.mu.Unlock()

	o.publishEvent(event.Event{
		Type:    event.TypeTranscriptionFinished,
		Level:   event.LevelInfo,
		State:   event.StateTranscribing,
		TextLen: textLen,
	})
}

func (o *Orchestrator) currentState() State {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.state
}

func (o *Orchestrator) finishProcessing() {
	flushed := o.deps.Merger.Flush()
	if flushed != "" {
		o.addCommittedLen(len(flushed))
	}

	o.emitTranscriptionFinished()

	if flushed != "" {
		o.injectText(flushed)
		o.emitFullText(flushed, "")
	}

	o.mu.Lock()
	failed := o.processingInjectionFailed
	shouldIdle := o.state == StateProcessing && o.pendingASR == 0
	o.mu.Unlock()

	if !failed {
		o.publishEvent(event.Event{
			Type:  event.TypeDone,
			Level: event.LevelInfo,
			State: event.StateDone,
		})
	}

	if shouldIdle {
		o.mu.Lock()
		o.setState(StateIdle)
		o.mu.Unlock()
	}
}
