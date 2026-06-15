//go:build no_whisper

package asr

import "fmt"

// NewLocalProvider is unavailable when the no_whisper build tag is set.
func NewLocalProvider(config ASRConfig) (Provider, error) {
	return nil, fmt.Errorf("local whisper.cpp ASR provider is disabled by the no_whisper build tag")
}
