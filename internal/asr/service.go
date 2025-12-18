package asr

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go"
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

// ASRConfig defines configuration for the ASR service.
type ASRConfig struct {
	ModelPath    string
	LanguageMode string // "auto", "ru", "en"
	Threads      int    // Default: 4
}

// DefaultConfig returns a safe default configuration.
func DefaultConfig() ASRConfig {
	return ASRConfig{
		ModelPath:    "models/ggml-base.bin",
		LanguageMode: "auto",
		Threads:      4,
	}
}

// ASRService handles speech recognition using whisper.cpp.
// It uses a single-worker pattern to safely manage the CGO context.
type ASRService struct {
	config  ASRConfig
	context *whisper.Context

	// Worker channels
	jobs    chan AudioWindow
	results chan TranscriptionChunk

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewASRService creates and initializes the ASR service.
// It loads the model, which may take some time.
func NewASRService(config ASRConfig) (*ASRService, error) {
	//if err := ValidateModel(config.ModelPath); err != nil {
	//	return nil, err
	//}

	// Load model and create context (Whisper_init does both in these bindings)
	ctx := whisper.Whisper_init(config.ModelPath)
	if ctx == nil {
		return nil, fmt.Errorf("failed to initialize whisper context (check model path)")
	}

	// Create service
	serviceCtx, cancel := context.WithCancel(context.Background())

	s := &ASRService{
		config:  config,
		context: ctx,
		jobs:    make(chan AudioWindow, 3), // Buffered queue, size 3
		results: make(chan TranscriptionChunk, 10),
		ctx:     serviceCtx,
		cancel:  cancel,
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

	if s.context != nil {
		s.context.Whisper_free()
	}

	close(s.jobs)
	close(s.results)
}

// Submit adds an audio window to the processing queue.
// If the queue is full, it drops the OLDEST item to maintain real-time responsiveness.
// This is a non-blocking call.
func (s *ASRService) Submit(window AudioWindow) error {
	select {
	case s.jobs <- window:
		return nil
	default:
		// Queue is full. Drop oldest.
		select {
		case <-s.jobs: // Drop one
			// Log drop?
		default:
		}

		// Try push again
		select {
		case s.jobs <- window:
			return nil
		default:
			return errors.New("queue full, dropped oldest but still failed to push")
		}
	}
}

// Results returns the channel with transcription results.
func (s *ASRService) Results() <-chan TranscriptionChunk {
	return s.results
}

// workerLoop is the single point of interaction with the CGO context.
func (s *ASRService) workerLoop() {
	defer s.wg.Done()

	// Lock OS thread for CGO safety
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

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

	// Prepare params
	params := s.context.Whisper_full_default_params(whisper.SAMPLING_GREEDY)
	params.SetThreads(s.config.Threads)
	params.SetPrintProgress(false)
	params.SetPrintRealtime(false)
	params.SetPrintTimestamps(false)

	// Language handling
	langID := -1 // Auto
	if s.config.LanguageMode != "auto" {
		langID = s.context.Whisper_lang_id(s.config.LanguageMode)
	}
	params.SetLanguage(langID)

	// Run inference
	// Note: Whisper_full expects []float32.
	if err := s.context.Whisper_full(params, window.Samples, nil, nil, nil); err != nil {
		s.results <- TranscriptionChunk{Error: fmt.Errorf("processing failed: %w", err)}
		return
	}

	// Iterate segments
	nSegments := s.context.Whisper_full_n_segments()
	if nSegments == 0 {
		return
	}

	var fullText string
	var start, end float32

	for i := 0; i < nSegments; i++ {
		text := s.context.Whisper_full_get_segment_text(i)
		t0 := s.context.Whisper_full_get_segment_t0(i)
		t1 := s.context.Whisper_full_get_segment_t1(i)

		fullText += text

		if i == 0 {
			start = float32(t0) / 100.0 // Whisper time is 10ms units
		}
		end = float32(t1) / 100.0
	}

	if fullText != "" {
		s.results <- TranscriptionChunk{
			Text:     fullText,
			Language: s.config.LanguageMode,
			StartSec: start,
			EndSec:   end,
			Prob:     1.0,
		}
	}
}
