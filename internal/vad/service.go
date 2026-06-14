// Package vad provides Voice Activity Detection using Silero VAD (ONNX).
// This package implements production-ready VAD with LSTM state management.
package vad

import (
	"fmt"
	"math"
	"os"

	onnxruntime "github.com/yalue/onnxruntime_go"
)

// VADService performs Voice Activity Detection using Silero VAD model.
type VADService struct {
	session    *onnxruntime.AdvancedSession
	config     VADConfig
	sampleRate int
	isV5       bool

	// Input Tensors (Persistent)
	tInput *onnxruntime.Tensor[float32]
	tSR    *onnxruntime.Tensor[int64]

	// V5: Combined state [2, 1, 128]
	tStateIn  *onnxruntime.Tensor[float32]
	tStateOut *onnxruntime.Tensor[float32]

	// V4: Separate H and C [2, 1, 64] - Note: Confirmed [2, 1, 64] via probe
	tHIn, tCIn   *onnxruntime.Tensor[float32]
	tHOut, tCOut *onnxruntime.Tensor[float32]

	// Output Probability
	tOutput *onnxruntime.Tensor[float32]
}

// VADConfig defines VAD model parameters
type VADConfig struct {
	ModelPath    string  // Path to silero_vad.onnx
	SampleRate   int     // 16000 Hz or 8000 Hz
	ChunkSize    int     // 512, 1024, or 1536 samples
	Threshold    float32 // Probability threshold (0.0-1.0)
	DebugRMS     bool    // Print RMS/min/max (debug only)
	DebugOut     bool    // Print raw model probabilities (debug only)
	InputGain    float32 // Linear gain applied to input (default 1.0)
	SingleLogit  bool    // Treat single-value output as logit
	InvertOut    bool    // Invert final probability
	NormalizeRMS bool    // Normalize input RMS
	TargetRMS    float32 // Target RMS for normalization
	LibPath      string  // Path to libonnxruntime.so
}

// DefaultConfig returns production-ready VAD configuration
func DefaultConfig() VADConfig {
	return VADConfig{
		ModelPath:  "models/silero_vad_v4.onnx",
		SampleRate: 16000,
		ChunkSize:  512,
		Threshold:  0.5,
		InputGain:  1.0,
		TargetRMS:  0.05,
		LibPath:    "third_party/lib/libonnxruntime.so",
	}
}

func NewVADService(config VADConfig) (*VADService, error) {
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	ortPath, err := FindLibPath(config.LibPath)
	if err != nil {
		return nil, fmt.Errorf("%v. Please run: ./scripts/download_libs.sh", err)
	}
	config.LibPath = ortPath
	onnxruntime.SetSharedLibraryPath(ortPath)

	err = onnxruntime.InitializeEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ONNX Runtime: %w", err)
	}

	modelData, err := os.ReadFile(config.ModelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read model file: %w", err)
	}

	service := &VADService{
		config:     config,
		sampleRate: config.SampleRate,
	}

	// 1. Common Tensors
	if err := service.initCommonTensors(); err != nil {
		return nil, err
	}

	// 2. Try V5
	if err := service.tryInitV5(modelData); err == nil {
		service.isV5 = true
		return service, nil
	} else {
		service.cleanupAfterV5()
	}

	// 3. Try V4
	if err := service.tryInitV4(modelData); err == nil {
		service.isV5 = false
		return service, nil
	}

	service.destroyTensors()
	return nil, fmt.Errorf("model incompatible with V4 or V5 signatures (tried path: %s)", config.ModelPath)
}

func (v *VADService) initCommonTensors() error {
	var err error
	v.tInput, err = onnxruntime.NewTensor(onnxruntime.NewShape(1, int64(v.config.ChunkSize)), make([]float32, v.config.ChunkSize))
	if err != nil {
		return err
	}
	v.tSR, err = onnxruntime.NewTensor(onnxruntime.NewShape(1), []int64{int64(v.sampleRate)})
	if err != nil {
		return err
	}
	v.tOutput, err = onnxruntime.NewTensor(onnxruntime.NewShape(1, 1), make([]float32, 1))
	return err
}

func (v *VADService) tryInitV5(data []byte) error {
	var err error
	shape := onnxruntime.NewShape(2, 1, 128)
	v.tStateIn, err = onnxruntime.NewTensor(shape, make([]float32, 2*1*128))
	if err != nil {
		return err
	}
	v.tStateOut, err = onnxruntime.NewTensor(shape, make([]float32, 2*1*128))
	if err != nil {
		return err
	}

	v.session, err = onnxruntime.NewAdvancedSessionWithONNXData(
		data,
		[]string{"input", "state", "sr"},
		[]string{"output", "stateN"},
		[]onnxruntime.Value{v.tInput, v.tStateIn, v.tSR},
		[]onnxruntime.Value{v.tOutput, v.tStateOut},
		nil,
	)
	if err != nil {
		return err
	}

	// PING test to validate names
	return v.session.Run()
}

func (v *VADService) cleanupAfterV5() {
	if v.session != nil {
		v.session.Destroy()
		v.session = nil
	}
	if v.tStateIn != nil {
		v.tStateIn.Destroy()
		v.tStateIn = nil
	}
	if v.tStateOut != nil {
		v.tStateOut.Destroy()
		v.tStateOut = nil
	}
}

func (v *VADService) tryInitV4(data []byte) error {
	var err error
	shape := onnxruntime.NewShape(2, 1, 64) // Confirmed [2, 1, 64] for V4
	v.tHIn, err = onnxruntime.NewTensor(shape, make([]float32, 2*1*64))
	if err != nil {
		return err
	}
	v.tCIn, err = onnxruntime.NewTensor(shape, make([]float32, 2*1*64))
	if err != nil {
		return err
	}
	v.tHOut, err = onnxruntime.NewTensor(shape, make([]float32, 2*1*64))
	if err != nil {
		return err
	}
	v.tCOut, err = onnxruntime.NewTensor(shape, make([]float32, 2*1*64))
	if err != nil {
		return err
	}

	v.session, err = onnxruntime.NewAdvancedSessionWithONNXData(
		data,
		[]string{"input", "sr", "h", "c"},
		[]string{"output", "hn", "cn"},
		[]onnxruntime.Value{v.tInput, v.tSR, v.tHIn, v.tCIn},
		[]onnxruntime.Value{v.tOutput, v.tHOut, v.tCOut},
		nil,
	)
	if err != nil {
		return err
	}

	// PING test
	return v.session.Run()
}

func (v *VADService) Process(chunk []float32) (float32, error) {
	if len(chunk) != v.config.ChunkSize {
		return 0, fmt.Errorf("chunk size mismatch")
	}

	var rms float32
	if v.config.DebugRMS || v.config.NormalizeRMS {
		var sumSq float64
		for _, s := range chunk {
			sumSq += float64(s * s)
		}
		rms = float32(math.Sqrt(sumSq / float64(len(chunk))))
		if v.config.DebugRMS {
			fmt.Printf("VAD DEBUG: rms=%.6f\n", rms)
		}
	}

	input := v.tInput.GetData()
	gain := v.config.InputGain
	if v.config.NormalizeRMS && rms > 0 {
		gain *= v.config.TargetRMS / rms
	}

	for i := range chunk {
		input[i] = chunk[i] * gain
	}

	if err := v.session.Run(); err != nil {
		return 0, err
	}

	out := v.tOutput.GetData()
	prob, _ := computeProbability(out, v.config.SingleLogit)

	if v.config.InvertOut {
		prob = 1 - prob
	}
	if v.config.DebugOut {
		fmt.Printf("VAD RAW OUT: prob=%.6f vals=%v\n", prob, out)
	}

	// Propagate State
	if v.isV5 {
		copy(v.tStateIn.GetData(), v.tStateOut.GetData())
	} else {
		copy(v.tHIn.GetData(), v.tHOut.GetData())
		copy(v.tCIn.GetData(), v.tCOut.GetData())
	}

	return prob, nil
}

func (v *VADService) IsSpeech(probability float32) bool {
	return probability >= v.config.Threshold
}

func (v *VADService) ResetState() error {
	if v.isV5 {
		d := v.tStateIn.GetData()
		for i := range d {
			d[i] = 0
		}
	} else {
		h := v.tHIn.GetData()
		c := v.tCIn.GetData()
		for i := range h {
			h[i] = 0
		}
		for i := range c {
			c[i] = 0
		}
	}
	return nil
}

func (v *VADService) Close() error {
	if v.session != nil {
		v.session.Destroy()
	}
	v.destroyTensors()
	onnxruntime.DestroyEnvironment()
	return nil
}

func (v *VADService) destroyTensors() {
	ts := []*onnxruntime.Tensor[float32]{
		v.tInput, v.tOutput, v.tStateIn, v.tStateOut,
		v.tHIn, v.tHOut, v.tCIn, v.tCOut,
	}
	for _, t := range ts {
		if t != nil {
			t.Destroy()
		}
	}
	if v.tSR != nil {
		v.tSR.Destroy()
	}
}

func computeProbability(out []float32, logitMode bool) (float32, error) {
	if len(out) == 0 {
		return 0, nil
	}
	if len(out) == 1 {
		if logitMode {
			return 1 / (1 + float32(math.Exp(-float64(out[0])))), nil
		}
		return out[0], nil
	}
	a, b := out[0], out[1]
	m := a
	if b > a {
		m = b
	}
	ea := math.Exp(float64(a - m))
	eb := math.Exp(float64(b - m))
	return float32(eb / (ea + eb)), nil
}

func validateConfig(c VADConfig) error {
	if c.SampleRate != 16000 && c.SampleRate != 8000 {
		return fmt.Errorf("bad sr")
	}
	return nil
}
