package vad

import (
	"os"
	"testing"
)

// TestVADService_Integration tests the actual ONNX model execution.
// Requires libonnxruntime.so and models/silero_vad.onnx to be present.
func TestVADService_Integration(t *testing.T) {
	// Check if dependencies exist
	if _, err := os.Stat("../../libonnxruntime.so"); os.IsNotExist(err) {
		t.Skip("libonnxruntime.so not found, skipping integration test")
	}
	if _, err := os.Stat("../../models/silero_vad.onnx"); os.IsNotExist(err) {
		t.Skip("models/silero_vad.onnx not found, skipping integration test")
	}

	// Change working dir to project root for the test to find libs/models
	// or configure paths relative to test file
	config := DefaultConfig()
	config.ModelPath = "../../models/silero_vad.onnx"

	// We need to set the shared lib path relative to where the test binary runs
	// usually /tmp/..., so absolute path is safest or relative to project root
	// For this test, we'll assume we run from project root via `go test ./...`
	// but `go test` changes CWD to package dir.
	// Let's try to handle this.

	// Note: SetSharedLibraryPath is global.
	// We need to point to the lib we downloaded to project root.
	// Since test runs in internal/vad, project root is ../..

	// However, onnxruntime_go might need absolute path or just filename if in LD_LIBRARY_PATH.
	// Let's try relative path.

	// Create service
	// We need to override the library path setting in NewVADService or set it before.
	// But NewVADService calls SetSharedLibraryPath("libonnxruntime.so").
	// We should probably make that configurable or smart.
	// For now, let's symlink the lib to the package dir for the test?
	// Or better, update NewVADService to take lib path or use a default that works.

	// Let's assume the user runs tests from root, but go test changes dir.
	// We will skip this test if we can't load the lib, but we want it to pass.

	// HACK: Create a symlink to libonnxruntime.so in the package dir for the test
	_ = os.Symlink("../../libonnxruntime.so", "libonnxruntime.so")
	defer os.Remove("libonnxruntime.so")

	service, err := NewVADService(config)
	if err != nil {
		t.Fatalf("Failed to create VAD service: %v", err)
	}
	defer service.Close()

	// Create a silence chunk (zeros)
	chunk := make([]float32, config.ChunkSize)

	// Process silence
	prob, err := service.Process(chunk)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	t.Logf("Silence probability: %.4f", prob)
	if prob > 0.1 {
		t.Errorf("Expected low probability for silence, got %.4f", prob)
	}

	// Create "noise" chunk (random values) - Silero is robust, might still be low prob
	// but let's verify it runs without crashing.
	for i := range chunk {
		chunk[i] = 0.01 // Small DC offset/noise
	}

	prob, err = service.Process(chunk)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	t.Logf("Noise probability: %.4f", prob)

	// Reset state
	err = service.ResetState()
	if err != nil {
		t.Errorf("ResetState failed: %v", err)
	}
}
