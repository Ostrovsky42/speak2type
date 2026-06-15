package asr

import (
	"context"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEncodePCM16WAV(t *testing.T) {
	data, err := encodePCM16WAV([]float32{-2, -1, 0, 0.5, 1, 2}, 16000)
	if err != nil {
		t.Fatalf("encodePCM16WAV failed: %v", err)
	}
	if got := string(data[0:4]); got != "RIFF" {
		t.Fatalf("RIFF header = %q", got)
	}
	if got := string(data[8:12]); got != "WAVE" {
		t.Fatalf("WAVE header = %q", got)
	}
	if got := binary.LittleEndian.Uint32(data[24:28]); got != 16000 {
		t.Fatalf("sample rate = %d, want 16000", got)
	}
	if got := binary.LittleEndian.Uint32(data[40:44]); got != 12 {
		t.Fatalf("data size = %d, want 12", got)
	}
	if got := int16(binary.LittleEndian.Uint16(data[44:46])); got != -32768 {
		t.Fatalf("first sample = %d, want -32768", got)
	}
	if got := int16(binary.LittleEndian.Uint16(data[54:56])); got != 32767 {
		t.Fatalf("last sample = %d, want 32767", got)
	}
}

func TestCloudProviderTranscribe(t *testing.T) {
	t.Setenv("SPEAK2TYPE_TEST_ASR_KEY", "test-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization = %q", got)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Fatalf("content type = %q", r.Header.Get("Content-Type"))
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm failed: %v", err)
		}
		if got := r.FormValue("model"); got != "test-transcribe" {
			t.Fatalf("model = %q", got)
		}
		if got := r.FormValue("language"); got != "ru" {
			t.Fatalf("language = %q", got)
		}
		if got := r.FormValue("response_format"); got != "json" {
			t.Fatalf("response_format = %q", got)
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("file field missing: %v", err)
		}
		defer file.Close()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"тест"}`))
	}))
	defer server.Close()

	provider, err := NewGroqProvider(ASRConfig{
		Endpoint:       server.URL,
		APIKeyEnv:      "SPEAK2TYPE_TEST_ASR_KEY",
		Model:          "test-transcribe",
		LanguageMode:   "ru",
		ResponseFormat: "json",
		SampleRate:     16000,
		Timeout:        time.Second,
	})
	if err != nil {
		t.Fatalf("NewGroqProvider failed: %v", err)
	}

	text, err := provider.Transcribe(context.Background(), []float32{0, 0.25, -0.25})
	if err != nil {
		t.Fatalf("Transcribe failed: %v", err)
	}
	if text != "тест" {
		t.Fatalf("text = %q, want тест", text)
	}
}
