package asr

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// AudioWindow represents a chunk of audio to be transcribed.
type AudioWindow struct {
	Samples []float32 // 16kHz, mono
}

// TranscriptionChunk represents the result of transcription.
type TranscriptionChunk struct {
	Text     string
	Language string // "ru", "en", "auto"
	StartSec float32
	EndSec   float32
	Prob     float32
	Error    error
}

// ASRService handles speech recognition through a pluggable provider.
// It keeps the queue/worker behavior stable for the session orchestrator.
type ASRService struct {
	mu       sync.RWMutex
	config   ASRConfig
	provider Provider

	// Worker channels
	jobs    chan AudioWindow
	results chan TranscriptionChunk

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewASRService creates and initializes the ASR service.
func NewASRService(config ASRConfig) (*ASRService, error) {
	config.Provider = normalizeProvider(config.Provider)
	if config.SampleRate == 0 {
		config.SampleRate = 16000
	}

	provider, err := newProvider(config)
	if err != nil {
		return nil, err
	}

	serviceCtx, cancel := context.WithCancel(context.Background())

	s := &ASRService{
		config:   config,
		provider: provider,
		jobs:     make(chan AudioWindow, 3),
		results:  make(chan TranscriptionChunk, 10),
		ctx:      serviceCtx,
		cancel:   cancel,
	}

	return s, nil
}

// Start launches the worker goroutine.
func (s *ASRService) Start() {
	s.wg.Add(1)
	go s.workerLoop()
}

// Stop terminates the worker and releases resources.
func (s *ASRService) Stop() {
	s.cancel()
	s.wg.Wait()

	if closer, ok := s.provider.(closeProvider); ok {
		_ = closer.Close()
	}

	close(s.jobs)
	close(s.results)
}

// Submit adds an audio window to the processing queue.
// If the queue is full, it drops the OLDEST item to maintain real-time responsiveness.
// This is a non-blocking call and returns how many queued windows were dropped.
func (s *ASRService) Submit(window AudioWindow) (dropped int, err error) {
	select {
	case s.jobs <- window:
		return 0, nil
	default:
	}

	// Queue is full. Drop oldest queued item.
	select {
	case <-s.jobs:
		dropped = 1
	default:
	}

	// Try push again
	select {
	case s.jobs <- window:
		return dropped, nil
	default:
		return dropped, errors.New("queue full, dropped oldest but still failed to push")
	}
}

// Results returns the channel with transcription results.
func (s *ASRService) Results() <-chan TranscriptionChunk {
	return s.results
}

func (s *ASRService) workerLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		case window := <-s.jobs:
			s.processWindow(window)
		}
	}
}

func (s *ASRService) processWindow(window AudioWindow) {
	if len(window.Samples) == 0 {
		return
	}

	s.mu.RLock()
	languageMode := s.config.LanguageMode
	timeout := s.config.Timeout
	provider := s.provider
	sampleRate := s.config.SampleRate

	ctx := s.ctx
	cancel := func() {}
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	}

	text, err := provider.Transcribe(ctx, window.Samples)
	cancel()
	s.mu.RUnlock()
	if err != nil {
		s.results <- TranscriptionChunk{Error: fmt.Errorf("processing failed: %w", err)}
		return
	}

	// Always emit a completion event so callers can track in-flight work,
	// even if the provider didn't produce any text.
	s.results <- TranscriptionChunk{
		Text:     text,
		Language: languageMode,
		StartSec: 0,
		EndSec:   float32(len(window.Samples)) / float32(sampleRate),
		Prob:     1.0,
	}
}

// Reconfigure swaps the active ASR provider while keeping queues and workers alive.
func (s *ASRService) Reconfigure(config ASRConfig) error {
	config.Provider = normalizeProvider(config.Provider)
	if config.SampleRate == 0 {
		config.SampleRate = 16000
	}

	provider, err := newProvider(config)
	if err != nil {
		return err
	}

	s.mu.Lock()
	oldProvider := s.provider
	s.config = config
	s.provider = provider
	s.mu.Unlock()

	if closer, ok := oldProvider.(closeProvider); ok {
		_ = closer.Close()
	}
	return nil
}

// Config returns a snapshot of the active ASR configuration.
func (s *ASRService) Config() ASRConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// SetLanguageMode updates the ASR language mode at runtime.
func (s *ASRService) SetLanguageMode(lang string) {
	s.mu.Lock()
	s.config.LanguageMode = lang
	if setter, ok := s.provider.(languageSetter); ok {
		setter.SetLanguageMode(lang)
	}
	s.mu.Unlock()
}

// LanguageMode returns the current ASR language mode.
func (s *ASRService) LanguageMode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.LanguageMode
}
