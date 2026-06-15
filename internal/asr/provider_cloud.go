package asr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	openAITranscriptionsEndpoint = "https://api.openai.com/v1/audio/transcriptions"
	groqTranscriptionsEndpoint   = "https://api.groq.com/openai/v1/audio/transcriptions"
	defaultOpenAIModel           = "gpt-4o-mini-transcribe"
	defaultGroqModel             = "whisper-large-v3-turbo"
)

type cloudProvider struct {
	mu             sync.RWMutex
	name           string
	endpoint       string
	apiKey         string
	model          string
	language       string
	prompt         string
	responseFormat string
	sampleRate     int
	client         *http.Client
}

// NewOpenAIProvider creates an OpenAI audio transcription provider.
func NewOpenAIProvider(config ASRConfig) (Provider, error) {
	return newCloudProvider(config, cloudDefaults{
		name:      ProviderOpenAI,
		endpoint:  openAITranscriptionsEndpoint,
		apiKeyEnv: "OPENAI_API_KEY",
		model:     defaultOpenAIModel,
	})
}

// NewGroqProvider creates a Groq OpenAI-compatible transcription provider.
func NewGroqProvider(config ASRConfig) (Provider, error) {
	return newCloudProvider(config, cloudDefaults{
		name:      ProviderGroq,
		endpoint:  groqTranscriptionsEndpoint,
		apiKeyEnv: "GROQ_API_KEY",
		model:     defaultGroqModel,
	})
}

type cloudDefaults struct {
	name      string
	endpoint  string
	apiKeyEnv string
	model     string
}

func newCloudProvider(config ASRConfig, defaults cloudDefaults) (*cloudProvider, error) {
	apiKeyEnv := strings.TrimSpace(config.APIKeyEnv)
	if apiKeyEnv == "" {
		apiKeyEnv = defaults.apiKeyEnv
	}
	apiKey := strings.TrimSpace(os.Getenv(apiKeyEnv))
	if apiKey == "" {
		return nil, fmt.Errorf("%s API key is required: set %s", defaults.name, apiKeyEnv)
	}

	endpoint := strings.TrimSpace(config.Endpoint)
	if endpoint == "" {
		endpoint = defaults.endpoint
	}

	model := strings.TrimSpace(config.Model)
	if model == "" {
		model = defaults.model
	}

	responseFormat := strings.TrimSpace(config.ResponseFormat)
	if responseFormat == "" {
		responseFormat = "json"
	}

	sampleRate := config.SampleRate
	if sampleRate == 0 {
		sampleRate = 16000
	}

	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	return &cloudProvider{
		name:           defaults.name,
		endpoint:       endpoint,
		apiKey:         apiKey,
		model:          model,
		language:       config.LanguageMode,
		prompt:         config.Prompt,
		responseFormat: responseFormat,
		sampleRate:     sampleRate,
		client:         &http.Client{Timeout: timeout},
	}, nil
}

func (p *cloudProvider) Transcribe(ctx context.Context, samples []float32) (string, error) {
	p.mu.RLock()
	endpoint := p.endpoint
	apiKey := p.apiKey
	model := p.model
	language := p.language
	prompt := p.prompt
	responseFormat := p.responseFormat
	sampleRate := p.sampleRate
	client := p.client
	p.mu.RUnlock()

	wavData, err := encodePCM16WAV(samples, sampleRate)
	if err != nil {
		return "", err
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "speech.wav")
	if err != nil {
		return "", fmt.Errorf("failed to create multipart file: %w", err)
	}
	if _, err := part.Write(wavData); err != nil {
		return "", fmt.Errorf("failed to write multipart audio: %w", err)
	}
	if err := writer.WriteField("model", model); err != nil {
		return "", fmt.Errorf("failed to write model field: %w", err)
	}
	if language != "" && language != "auto" {
		if err := writer.WriteField("language", language); err != nil {
			return "", fmt.Errorf("failed to write language field: %w", err)
		}
	}
	if prompt != "" {
		if err := writer.WriteField("prompt", prompt); err != nil {
			return "", fmt.Errorf("failed to write prompt field: %w", err)
		}
	}
	if responseFormat != "" {
		if err := writer.WriteField("response_format", responseFormat); err != nil {
			return "", fmt.Errorf("failed to write response_format field: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close multipart body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return "", fmt.Errorf("failed to create transcription request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s transcription request failed: %w", p.name, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("failed to read %s transcription response: %w", p.name, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("%s transcription failed: status %d: %s", p.name, resp.StatusCode, cloudErrorMessage(respBody))
	}

	if responseFormat == "text" {
		return string(respBody), nil
	}

	var parsed struct {
		Text  string `json:"text"`
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    any    `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("failed to decode %s transcription response: %w", p.name, err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("%s transcription failed: %s", p.name, parsed.Error.Message)
	}
	return parsed.Text, nil
}

func (p *cloudProvider) SetLanguageMode(lang string) {
	p.mu.Lock()
	p.language = lang
	p.mu.Unlock()
}

func cloudErrorMessage(body []byte) string {
	var parsed struct {
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    any    `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error != nil && parsed.Error.Message != "" {
		return parsed.Error.Message
	}
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		return "empty response body"
	}
	return msg
}
