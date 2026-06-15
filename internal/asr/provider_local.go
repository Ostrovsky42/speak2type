//go:build !no_whisper

package asr

import (
	"context"
	"fmt"
	"runtime"
	"sync"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go"
)

// LocalProvider transcribes audio with the local whisper.cpp binding.
type LocalProvider struct {
	mu      sync.RWMutex
	context *whisper.Context
	lang    string
	threads int
}

// NewLocalProvider creates a local whisper.cpp provider.
func NewLocalProvider(config ASRConfig) (*LocalProvider, error) {
	ctx := whisper.Whisper_init(config.ModelPath)
	if ctx == nil {
		return nil, fmt.Errorf("failed to initialize whisper context (check model path)")
	}

	threads := config.Threads
	if threads <= 0 {
		threads = 4
	}

	return &LocalProvider{
		context: ctx,
		lang:    config.LanguageMode,
		threads: threads,
	}, nil
}

// Transcribe runs local whisper.cpp inference for a 16 kHz mono float32 window.
func (p *LocalProvider) Transcribe(ctx context.Context, samples []float32) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	p.mu.RLock()
	languageMode := p.lang
	threads := p.threads
	p.mu.RUnlock()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	params := p.context.Whisper_full_default_params(whisper.SAMPLING_GREEDY)
	params.SetThreads(threads)
	params.SetPrintProgress(false)
	params.SetPrintRealtime(false)
	params.SetPrintTimestamps(false)
	params.SetTranslate(false)
	params.SetNoContext(false)

	langID := -1
	if languageMode != "" && languageMode != "auto" {
		langID = p.context.Whisper_lang_id(languageMode)
	}
	params.SetLanguage(langID)

	if err := p.context.Whisper_full(params, samples, nil, nil, nil); err != nil {
		return "", err
	}

	nSegments := p.context.Whisper_full_n_segments()
	var fullText string
	for i := 0; i < nSegments; i++ {
		fullText += p.context.Whisper_full_get_segment_text(i)
	}

	return fullText, nil
}

// SetLanguageMode updates the local whisper language hint.
func (p *LocalProvider) SetLanguageMode(lang string) {
	p.mu.Lock()
	p.lang = lang
	p.mu.Unlock()
}

// Close releases the whisper.cpp context.
func (p *LocalProvider) Close() error {
	if p.context != nil {
		p.context.Whisper_free()
		p.context = nil
	}
	return nil
}
