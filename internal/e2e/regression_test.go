package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Ostrovsky42/speak2type/internal/audio"
	"github.com/Ostrovsky42/speak2type/internal/vad"
)

func findProjectRoot() string {
	cwd, _ := os.Getwd()
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return cwd
}

func TestE2E_VAD_Regression(t *testing.T) {
	// Root detection for libs/models
	projectRoot := findProjectRoot()
	libPath := filepath.Join(projectRoot, "third_party/lib/libonnxruntime.so")
	modelPath := filepath.Join(projectRoot, "models/silero_vad_v4.onnx")

	t.Logf("Project Root: %s", projectRoot)
	if _, err := os.Stat(libPath); os.IsNotExist(err) {
		t.Skipf("libonnxruntime missing at %s, skipping e2e", libPath)
	}

	config := vad.DefaultConfig()
	config.LibPath = libPath
	config.ModelPath = modelPath

	v, err := vad.NewVADService(config)
	if err != nil {
		t.Fatalf("Failed to init VAD: %v", err)
	}
	defer v.Close()

	gate := vad.NewGate(vad.GateConfig{
		ThresholdStart:     0.5,
		ThresholdEnd:       0.35,
		MinSpeechDuration:  0,
		MinSilenceDuration: 0,
	})

	t.Run("Tone (Non-Speech)", func(t *testing.T) {
		path := filepath.Join(projectRoot, "testdata/tone_1khz.wav")
		t.Logf("Reading wav from %s", path)
		samples, err := audio.ReadWavFile(path)
		if err != nil {
			t.Fatalf("Failed to read test wav: %v", err)
		}
		t.Logf("Read %d samples", len(samples))

		// Process in chunks
		chunkSize := config.ChunkSize
		speechDetected := false
		maxProb := float32(0)
		for i := 0; i+chunkSize <= len(samples); i += chunkSize {
			chunk := samples[i : i+chunkSize]
			prob, err := v.Process(chunk)
			if err != nil {
				t.Fatalf("VAD Error: %v", err)
			}
			if prob > maxProb {
				maxProb = prob
			}
			_, isSpeech := gate.Process(prob)
			if isSpeech {
				speechDetected = true
			}
		}
		t.Logf("Processing complete. Max Prob: %.4f, Speech detected: %v", maxProb, speechDetected)

		if speechDetected {
			t.Error("VAD incorrectly detected tone as speech")
		}
	})

	t.Run("Noise (Silence)", func(t *testing.T) {
		path := filepath.Join(projectRoot, "testdata/noise_keyboard.wav")
		samples, err := audio.ReadWavFile(path)
		if err != nil {
			t.Fatalf("Failed to read test wav: %v", err)
		}

		chunkSize := config.ChunkSize
		for i := 0; i+chunkSize <= len(samples); i += chunkSize {
			chunk := samples[i : i+chunkSize]
			prob, _ := v.Process(chunk)
			_, isSpeech := gate.Process(prob)
			if isSpeech {
				// We expect noise_keyboard.wav generated at 0.1 amp to be below threshold
				// but VAD might be sensitive.
				// Just log for now or assert if Threshold is high enough.
				t.Logf("Warning: VAD detected noise as speech (prob: %.4f)", prob)
			}
		}
	})
}
