package vad

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVADService_Integration(t *testing.T) {
	// Root detection
	projectRoot, _ := filepath.Abs("../..")
	libPath := filepath.Join(projectRoot, "third_party/lib/libonnxruntime.so")
	modelPath := filepath.Join(projectRoot, "models/silero_vad_v4.onnx")

	if _, err := os.Stat(libPath); os.IsNotExist(err) {
		t.Skipf("Library not found at %s, skipping", libPath)
	}

	config := DefaultConfig()
	config.LibPath = libPath
	config.ModelPath = modelPath

	service, err := NewVADService(config)
	if err != nil {
		t.Fatalf("Failed to create VAD service: %v", err)
	}
	defer service.Close()

	// 1. Silence Test
	chunk := make([]float32, config.ChunkSize)
	prob, err := service.Process(chunk)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	t.Logf("Silence probability: %.4f", prob)
	if prob > 0.1 {
		t.Errorf("Expected low probability for silence, got %.4f", prob)
	}

	// 2. State Reset Test
	err = service.ResetState()
	if err != nil {
		t.Errorf("ResetState failed: %v", err)
	}
}
