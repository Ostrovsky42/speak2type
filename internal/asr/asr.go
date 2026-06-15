package asr

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	ProviderLocal  = "local"
	ProviderOpenAI = "openai"
	ProviderGroq   = "groq"
)

// Provider is the common contract for speech recognition backends.
type Provider interface {
	Transcribe(ctx context.Context, wavSamples []float32) (string, error)
}

type languageSetter interface {
	SetLanguageMode(lang string)
}

type closeProvider interface {
	Close() error
}

// ASRConfig defines configuration for the ASR service.
type ASRConfig struct {
	Provider       string
	ModelPath      string
	Model          string
	Endpoint       string
	APIKey         string
	OpenAIAPIKey   string
	GroqAPIKey     string
	APIKeyEnv      string
	LanguageMode   string // "auto", "ru", "en"
	Prompt         string
	ResponseFormat string
	Threads        int // Default: 4
	SampleRate     int // Default: 16000
	Timeout        time.Duration
}

// DefaultConfig returns a safe default configuration.
func DefaultConfig() ASRConfig {
	return ASRConfig{
		Provider:       ProviderLocal,
		ModelPath:      "models/ggml-base.bin",
		LanguageMode:   "auto",
		ResponseFormat: "json",
		Threads:        4,
		SampleRate:     16000,
		Timeout:        30 * time.Second,
	}
}

func newProvider(config ASRConfig) (Provider, error) {
	switch normalizeProvider(config.Provider) {
	case ProviderLocal:
		return NewLocalProvider(config)
	case ProviderOpenAI:
		return NewOpenAIProvider(config)
	case ProviderGroq:
		return NewGroqProvider(config)
	default:
		return nil, fmt.Errorf("unsupported ASR provider: %s", config.Provider)
	}
}

func normalizeProvider(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return ProviderLocal
	}
	return provider
}
