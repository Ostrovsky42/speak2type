// Package vad provides Voice Activity Detection using Silero VAD (ONNX).
// This package implements production-ready VAD with LSTM state management
// and is optimized for keyboard noise rejection.
package vad

import (
	"fmt"
	"math"
	"os"

	onnxruntime "github.com/yalue/onnxruntime_go"
)

// VADService performs Voice Activity Detection using Silero VAD model.
//
// Design principles:
// - ONNX Runtime AdvancedSession for zero-allocation inference
// - Pre-allocated tensors for inputs and outputs
// - Manual LSTM state propagation (copy out -> in)
type VADService struct {
	session    *onnxruntime.AdvancedSession
	config     VADConfig
	sampleRate int

	// Input Tensors (Persistent)
	tInput   *onnxruntime.Tensor[float32]
	tSR      *onnxruntime.Tensor[int64]
	tStateIn *onnxruntime.Tensor[float32] // Combined H+C [2, 1, 128]

	// Output Tensors (Persistent)
	tOutput   *onnxruntime.Tensor[float32]
	tStateOut *onnxruntime.Tensor[float32]
}

// VADConfig defines VAD model parameters
type VADConfig struct {
	ModelPath    string  // Path to silero_vad.onnx
	SampleRate   int     // 16000 Hz (Whisper compatibility)
	ChunkSize    int     // 512, 1024, or 1536 samples
	Threshold    float32 // Probability threshold (0.0-1.0)
	DebugRMS     bool    // Print RMS/min/max for incoming chunks (debug only)
	DebugOut     bool    // Print raw model outputs (debug only)
	InputGain    float32 // Linear gain applied to input audio (default 1.0)
	SingleLogit  bool    // Treat single-value output as a logit (apply sigmoid)
	InvertOut    bool    // Invert final probability (prob = 1 - prob)
	NormalizeRMS bool    // Normalize input RMS to TargetRMS before inference
	TargetRMS    float32 // Target RMS when NormalizeRMS is true (e.g., 0.05)
}

// DefaultConfig returns production-ready VAD configuration
func DefaultConfig() VADConfig {
	return VADConfig{
		ModelPath:    "models/silero_vad.onnx",
		SampleRate:   16000,
		ChunkSize:    512, // 32ms @ 16kHz (lowest latency)
		Threshold:    0.5, // Balanced threshold
		DebugRMS:     false,
		DebugOut:     false,
		InputGain:    1.0,
		SingleLogit:  false,
		InvertOut:    false,
		NormalizeRMS: false,
		TargetRMS:    0.05,
	}
}

// NewVADService creates a new VAD service with loaded ONNX model.
func NewVADService(config VADConfig) (*VADService, error) {
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Initialize ONNX Runtime
	onnxruntime.SetSharedLibraryPath("libonnxruntime.so")
	err := onnxruntime.InitializeEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ONNX Runtime: %w", err)
	}

	// Load model file
	modelData, err := os.ReadFile(config.ModelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read model file: %w", err)
	}

	service := &VADService{
		config:     config,
		sampleRate: config.SampleRate,
	}

	// Initialize all tensors
	if err := service.initTensors(); err != nil {
		return nil, err
	}

	// Create ONNX session with pre-allocated tensors
	// Silero v5 names: input, state, sr -> output, stateN
	inputNames := []string{"input", "state", "sr"}
	outputNames := []string{"output", "stateN"}

	inputs := []onnxruntime.Value{
		service.tInput,
		service.tStateIn,
		service.tSR,
	}

	outputs := []onnxruntime.Value{
		service.tOutput,
		service.tStateOut,
	}

	session, err := onnxruntime.NewAdvancedSessionWithONNXData(
		modelData,
		inputNames,
		outputNames,
		inputs,
		outputs,
		nil, // Default options
	)
	if err != nil {
		service.destroyTensors()
		return nil, fmt.Errorf("failed to create ONNX session: %w", err)
	}
	service.session = session

	return service, nil
}

func (v *VADService) initTensors() error {
	var err error

	// 1. Input Audio: [1, ChunkSize]
	inputShape := onnxruntime.NewShape(1, int64(v.config.ChunkSize))
	inputData := make([]float32, v.config.ChunkSize)
	v.tInput, err = onnxruntime.NewTensor(inputShape, inputData)
	if err != nil {
		return fmt.Errorf("input tensor: %w", err)
	}

	// 2. Sample Rate: [1]
	srShape := onnxruntime.NewShape(1)
	srData := []int64{int64(v.sampleRate)}
	v.tSR, err = onnxruntime.NewTensor(srShape, srData)
	if err != nil {
		return fmt.Errorf("sr tensor: %w", err)
	}

	// 3. State In: [2, 1, 128] (Combined H+C)
	stateShape := onnxruntime.NewShape(2, 1, 128)
	stateData := make([]float32, 2*1*128)
	v.tStateIn, err = onnxruntime.NewTensor(stateShape, stateData)
	if err != nil {
		return fmt.Errorf("state_in tensor: %w", err)
	}

	// 4. Output Probability: [1, 1] (common Silero export). We handle 1 or 2
	// values in computeProbability to stay compatible with dual-output models.
	outShape := onnxruntime.NewShape(1, 1)
	outData := make([]float32, 1)
	v.tOutput, err = onnxruntime.NewTensor(outShape, outData)
	if err != nil {
		return fmt.Errorf("output tensor: %w", err)
	}

	// 5. State Out: [2, 1, 128]
	v.tStateOut, err = onnxruntime.NewTensor(stateShape, make([]float32, 2*1*128))
	if err != nil {
		return fmt.Errorf("state_out tensor: %w", err)
	}

	return nil
}

func (v *VADService) destroyTensors() {
	if v.tInput != nil {
		v.tInput.Destroy()
	}
	if v.tSR != nil {
		v.tSR.Destroy()
	}
	if v.tStateIn != nil {
		v.tStateIn.Destroy()
	}
	if v.tOutput != nil {
		v.tOutput.Destroy()
	}
	if v.tStateOut != nil {
		v.tStateOut.Destroy()
	}
}

// Process analyzes an audio chunk and returns speech probability.
//
// Args:
//   - chunk: Audio samples (must be len(chunk) == config.ChunkSize)
//
// Returns:
//   - float32: Speech probability (0.0 = silence, 1.0 = speech)
//   - error: If processing fails
//
// Thread-safety: NOT thread-safe. Caller must serialize access.
func (v *VADService) Process(chunk []float32) (float32, error) {
	if len(chunk) != v.config.ChunkSize {
		return 0, fmt.Errorf("chunk size mismatch: expected %d, got %d",
			v.config.ChunkSize, len(chunk))
	}

	var rms float32
	if v.config.DebugRMS || v.config.NormalizeRMS {
		var sumSq float64
		minVal := float32(1e9)
		maxVal := float32(-1e9)

		for _, s := range chunk {
			sumSq += float64(s * s)
			if s < minVal {
				minVal = s
			}
			if s > maxVal {
				maxVal = s
			}
		}

		if len(chunk) > 0 {
			rms = float32(sumSq / float64(len(chunk)))
		}

		if v.config.DebugRMS {
			fmt.Printf("VAD DEBUG: len=%d rms=%.6f min=%.6f max=%.6f\n",
				len(chunk), rms, minVal, maxVal)
		}
	}

	// 1. Copy chunk data to input tensor
	// GetData() returns a slice backed by C memory, so we copy into it
	inputData := v.tInput.GetData()
	gain := v.config.InputGain
	if v.config.NormalizeRMS && rms > 0 {
		gain *= v.config.TargetRMS / rms
	}

	if gain == 1.0 {
		copy(inputData, chunk)
	} else {
		for i := range chunk {
			inputData[i] = chunk[i] * gain
		}
	}

	// 2. Run inference
	if err := v.session.Run(); err != nil {
		return 0, fmt.Errorf("inference failed: %w", err)
	}

	// 3. Get output probability
	outputData := v.tOutput.GetData()
	probability, err := computeProbability(outputData, v.config.SingleLogit)
	if err != nil {
		return 0, err
	}

	if v.config.InvertOut {
		probability = 1 - probability
	}

	if v.config.DebugOut {
		fmt.Printf("VAD RAW OUT: len=%d vals=%v prob=%.6f\n", len(outputData), outputData, probability)
	}

	// 4. Propagate state: Output -> Input for next step
	// We must copy data from Out tensors to In tensors
	stateIn := v.tStateIn.GetData()
	stateOut := v.tStateOut.GetData()
	copy(stateIn, stateOut)

	return probability, nil
}

// IsSpeech returns true if probability exceeds threshold.
func (v *VADService) IsSpeech(probability float32) bool {
	return probability >= v.config.Threshold
}

// ResetState resets LSTM state to zeros.
func (v *VADService) ResetState() error {
	stateIn := v.tStateIn.GetData()
	for i := range stateIn {
		stateIn[i] = 0
	}
	return nil
}

// Close releases resources.
func (v *VADService) Close() error {
	if v.session != nil {
		v.session.Destroy()
	}
	v.destroyTensors()
	onnxruntime.DestroyEnvironment()
	return nil
}

// GetConfig returns current VAD configuration.
func (v *VADService) GetConfig() VADConfig {
	return v.config
}

func computeProbability(outputData []float32, singleIsLogit bool) (float32, error) {
	if len(outputData) == 0 {
		return 0, fmt.Errorf("output tensor is empty")
	}

	switch len(outputData) {
	case 1:
		if singleIsLogit {
			return sigmoid(outputData[0]), nil
		}
		// Most Silero exports already return probability in [0,1].
		return outputData[0], nil
	case 2:
		// Silero v4/v5 often outputs logits as [non-speech, speech].
		return softmax2(outputData[0], outputData[1]), nil
	default:
		// Fallback: use last element as-is.
		return outputData[len(outputData)-1], nil
	}
}

func sigmoid(x float32) float32 {
	return 1 / (1 + float32(math.Exp(-float64(x))))
}

func softmax2(a, b float32) float32 {
	max := a
	if b > max {
		max = b
	}
	expA := math.Exp(float64(a - max))
	expB := math.Exp(float64(b - max))
	return float32(expB / (expA + expB))
}

func validateConfig(config VADConfig) error {
	if config.SampleRate != 8000 && config.SampleRate != 16000 {
		return fmt.Errorf("sample rate must be 8000 or 16000")
	}
	validChunkSizes := map[int]bool{512: true, 1024: true, 1536: true}
	if !validChunkSizes[config.ChunkSize] {
		return fmt.Errorf("chunk size must be 512, 1024, or 1536")
	}
	if _, err := os.Stat(config.ModelPath); os.IsNotExist(err) {
		return fmt.Errorf("model file not found: %s", config.ModelPath)
	}
	return nil
}
