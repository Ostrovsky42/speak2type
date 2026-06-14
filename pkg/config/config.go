// Package config provides application configuration management.
// Supports JSON-based persistent configuration with platform-specific paths.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Ostrovsky42/speak2type/internal/version"
)

// Config represents the complete Speak2Type configuration
type Config struct {
	Version       string             `json:"version"`
	Audio         AudioConfig        `json:"audio"`
	VAD           VADConfig          `json:"vad"`
	ASR           ASRConfig          `json:"asr"`
	Merger        MergerConfig       `json:"merger"`
	Session       SessionConfig      `json:"session"`
	UI            UIConfig           `json:"ui"`
	Notifications NotificationConfig `json:"notifications"`
	Logging       LoggingConfig      `json:"logging"`
}

// AudioConfig defines audio capture parameters
type AudioConfig struct {
	DeviceID         *int `json:"device_id"`          // nil = default device
	SampleRate       int  `json:"sample_rate"`        // 16000 Hz
	BufferDurationMS int  `json:"buffer_duration_ms"` // 30ms
}

// VADConfig defines Voice Activity Detection parameters
type VADConfig struct {
	Enabled              bool    `json:"enabled"`
	Threshold            float32 `json:"threshold"`               // 0.5
	MinSpeechDurationMS  int     `json:"min_speech_duration_ms"`  // 300ms
	MinSilenceDurationMS int     `json:"min_silence_duration_ms"` // 500ms
}

// ASRConfig defines Automatic Speech Recognition parameters
type ASRConfig struct {
	ModelPath        string `json:"model_path"`        // Path to ggml model
	LanguageMode     string `json:"language_mode"`     // "auto", "ru", "en"
	PrimaryLanguage  string `json:"primary_language"`  // "ru"
	FallbackLanguage string `json:"fallback_language"` // "en"
	Translate        bool   `json:"translate"`         // false (transcribe, don't translate)
}

// MergerConfig defines text merging parameters
type MergerConfig struct {
	StabilityThreshold int `json:"stability_threshold"` // 2
	UnsafeZoneTokens   int `json:"unsafe_zone_tokens"`  // 5
}

// SessionConfig defines session behavior
type SessionConfig struct {
	Mode            string `json:"mode"`             // "quick_note" | "continuous"
	Hotkey          string `json:"hotkey"`           // "f8"
	AutoCapitalize  bool   `json:"auto_capitalize"`  // true
	AutoPunctuation bool   `json:"auto_punctuation"` // true
}

// UIConfig defines user interface parameters
type UIConfig struct {
	ShowOverlay     bool   `json:"show_overlay"`     // true
	OverlayPosition string `json:"overlay_position"` // "top-right"
	Theme           string `json:"theme"`            // "auto"
}

// NotificationConfig defines system notification behavior.
type NotificationConfig struct {
	Errors    bool `json:"errors"`    // true
	Done      bool `json:"done"`      // false
	Recording bool `json:"recording"` // false
}

// LoggingConfig defines logging verbosity.
type LoggingConfig struct {
	Level string `json:"level"` // "info"
}

// Default returns production-ready default configuration
func Default() *Config {
	return &Config{
		Version: version.Version,
		Audio: AudioConfig{
			DeviceID:         nil,
			SampleRate:       16000,
			BufferDurationMS: 30,
		},
		VAD: VADConfig{
			Enabled:              true,
			Threshold:            0.5,
			MinSpeechDurationMS:  300,
			MinSilenceDurationMS: 500,
		},
		ASR: ASRConfig{
			ModelPath:        "models/ggml-base.bin",
			LanguageMode:     "auto",
			PrimaryLanguage:  "ru",
			FallbackLanguage: "en",
			Translate:        false,
		},
		Merger: MergerConfig{
			StabilityThreshold: 2,
			UnsafeZoneTokens:   5,
		},
		Session: SessionConfig{
			Mode:            "quick_note",
			Hotkey:          "f8",
			AutoCapitalize:  true,
			AutoPunctuation: true,
		},
		UI: UIConfig{
			ShowOverlay:     true,
			OverlayPosition: "top-right",
			Theme:           "auto",
		},
		Notifications: NotificationConfig{
			Errors:    true,
			Done:      false,
			Recording: false,
		},
		Logging: LoggingConfig{
			Level: "info",
		},
	}
}

// ConfigPath returns the platform-specific configuration file path
// Linux: ~/.config/speak2type/config.json
// macOS: ~/Library/Application Support/speak2type/config.json
// Windows: %APPDATA%\speak2type\config.json
func ConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config dir: %w", err)
	}

	appConfigDir := filepath.Join(configDir, "speak2type")
	configFile := filepath.Join(appConfigDir, "config.json")

	return configFile, nil
}

// Load reads configuration from the standard config file location.
// If the file doesn't exist, returns default configuration.
func Load() (*Config, error) {
	configPath, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	// If config file doesn't exist, return defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return Default(), nil
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse JSON
	cfg := Default() // Start with defaults
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return cfg, nil
}

// Save writes configuration to the standard config file location.
// Creates parent directories if they don't exist.
func (c *Config) Save() error {
	configPath, err := ConfigPath()
	if err != nil {
		return err
	}

	// Create config directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Validate performs basic validation on configuration values
func (c *Config) Validate() error {
	// Audio validation
	if c.Audio.SampleRate != 16000 {
		return fmt.Errorf("sample rate must be 16000 Hz for Whisper compatibility")
	}

	// ASR validation
	validLanguageModes := map[string]bool{"auto": true, "ru": true, "en": true}
	if !validLanguageModes[c.ASR.LanguageMode] {
		return fmt.Errorf("invalid language_mode: %s (must be 'auto', 'ru', or 'en')", c.ASR.LanguageMode)
	}

	// Session validation
	validModes := map[string]bool{"quick_note": true, "continuous": true}
	if !validModes[c.Session.Mode] {
		return fmt.Errorf("invalid session mode: %s (must be 'quick_note' or 'continuous')", c.Session.Mode)
	}

	// Merger validation
	if c.Merger.StabilityThreshold < 1 {
		return fmt.Errorf("stability_threshold must be >= 1")
	}

	// Logging validation
	switch c.Logging.Level {
	case "", "debug", "info", "warn", "warning", "error":
	default:
		return fmt.Errorf("invalid logging level: %s (must be debug, info, warn, or error)", c.Logging.Level)
	}

	return nil
}
