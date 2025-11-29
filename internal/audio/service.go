package audio

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/gen2brain/malgo"
)

// AudioService manages real-time audio capture from microphone.
// Uses malgo (miniaudio) for cross-platform audio with zero external dependencies.
//
// Design principles:
// - Hardware resampling to 16kHz (Whisper requirement)
// - Zero-allocation audio callback
// - Thread-safe start/stop operations
// - Real-time performance monitoring
type AudioService struct {
	// malgo (miniaudio) components
	ctx    *malgo.AllocatedContext
	device *malgo.Device
	config AudioServiceConfig

	// Ring buffer for audio samples
	ringBuffer *RingBuffer

	// State management
	isRecording  atomic.Bool
	mu           sync.Mutex
	startTime    time.Time
	callbackHits atomic.Uint64

	// Performance metrics
	droppedFrames atomic.Uint64
	totalFrames   atomic.Uint64
}

// AudioServiceConfig defines audio capture parameters
type AudioServiceConfig struct {
	// Hardware parameters
	DeviceID   *malgo.DeviceID  // nil = default device
	SampleRate uint32           // 16000 Hz (Whisper requirement)
	Channels   uint32           // 1 (mono)
	Format     malgo.FormatType // malgo.FormatF32
	BufferMS   uint32           // Buffer size in milliseconds (30ms default)

	// Ring buffer configuration
	RingBufferDuration float64 // Seconds (e.g., 10.0)
}

// DefaultConfig returns production-ready configuration for Whisper
func DefaultConfig() AudioServiceConfig {
	return AudioServiceConfig{
		DeviceID:           nil, // Default microphone
		SampleRate:         16000,
		Channels:           1,
		Format:             malgo.FormatF32,
		BufferMS:           30,   // 30ms latency
		RingBufferDuration: 10.0, // 10 seconds buffer
	}
}

// NewAudioService creates a new audio capture service.
// This initializes malgo but does NOT start capturing audio.
//
// Call Start() to begin audio capture.
func NewAudioService(config AudioServiceConfig) (*AudioService, error) {
	// Initialize malgo context
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize malgo: %w", err)
	}

	// Create ring buffer
	ringBuffer := NewRingBuffer(RingBufferConfig{
		DurationSeconds: config.RingBufferDuration,
		SampleRate:      int(config.SampleRate),
	})

	service := &AudioService{
		ctx:        ctx,
		config:     config,
		ringBuffer: ringBuffer,
	}

	return service, nil
}

// Start begins audio capture from the microphone.
// Audio samples are written to the internal ring buffer.
//
// Thread-safe: Can be called multiple times safely.
func (a *AudioService) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isRecording.Load() {
		return fmt.Errorf("audio service already started")
	}

	// Configure capture device
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = a.config.Format
	deviceConfig.Capture.Channels = a.config.Channels
	deviceConfig.SampleRate = a.config.SampleRate

	// Set device ID if specified (dereference pointer)
	if a.config.DeviceID != nil {
		deviceConfig.Capture.DeviceID = a.config.DeviceID.Pointer()
	}

	// Calculate period size (buffer size in frames)
	// period = (sample_rate * buffer_ms) / 1000
	periodSize := (a.config.SampleRate * a.config.BufferMS) / 1000
	deviceConfig.PeriodSizeInFrames = periodSize

	// Set up callbacks
	sizeInBytes := uint32(malgo.SampleSizeInBytes(a.config.Format))
	onRecvFrames := func(pOutputSamples, pInputSamples []byte, frameCount uint32) {
		a.onAudioCallback(pInputSamples, frameCount, sizeInBytes)
	}

	deviceConfig.Capture.ShareMode = malgo.Shared
	callbacks := malgo.DeviceCallbacks{
		Data: onRecvFrames,
	}

	// Initialize device
	device, err := malgo.InitDevice(a.ctx.Context, deviceConfig, callbacks)
	if err != nil {
		return fmt.Errorf("failed to initialize audio device: %w", err)
	}

	// Start device
	err = device.Start()
	if err != nil {
		device.Uninit()
		return fmt.Errorf("failed to start audio device: %w", err)
	}

	a.device = device
	a.isRecording.Store(true)
	a.startTime = time.Now()

	return nil
}

// Stop halts audio capture and releases resources.
//
// Thread-safe: Can be called multiple times safely.
func (a *AudioService) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isRecording.Load() {
		return nil // Already stopped
	}

	if a.device != nil {
		a.device.Uninit()
		a.device = nil
	}

	a.isRecording.Store(false)
	return nil
}

// Close releases all resources including malgo context.
// Must be called when service is no longer needed.
func (a *AudioService) Close() error {
	a.Stop()

	if a.ctx != nil {
		_ = a.ctx.Uninit()
		a.ctx.Free()
		a.ctx = nil
	}

	return nil
}

// onAudioCallback is called by malgo from the audio thread.
// CRITICAL: This MUST NOT allocate heap memory.
//
// Performance requirements:
// - Execution time < buffer duration (30ms default)
// - No heap allocations
// - No blocking operations
func (a *AudioService) onAudioCallback(pInputSamples []byte, frameCount uint32, sizeInBytes uint32) {
	// Update metrics (atomic, non-blocking)
	a.callbackHits.Add(1)
	a.totalFrames.Add(uint64(frameCount))

	// Convert []byte to []float32 using unsafe pointer (zero-copy view)
	// This is safe because malgo guarantees the lifetime of pInputSamples
	samples := unsafe.Slice((*float32)(unsafe.Pointer(&pInputSamples[0])), frameCount)

	// Write to ring buffer (mutex-protected, but fast)
	written := a.ringBuffer.Write(samples)

	// Track dropped frames
	if written < len(samples) {
		a.droppedFrames.Add(uint64(len(samples) - written))
	}
}

// Snapshot extracts recent audio samples from the buffer.
// This is safe to call concurrently with audio capture.
//
// Args:
//   - durationSeconds: How many seconds of audio to extract
//
// Returns:
//   - []float32: Audio samples (16kHz mono float32)
//   - Allocates new slice (intentional)
func (a *AudioService) Snapshot(durationSeconds float64) []float32 {
	return a.ringBuffer.Snapshot(durationSeconds, int(a.config.SampleRate))
}

// SnapshotLatest extracts the most recent N samples.
func (a *AudioService) SnapshotLatest(sampleCount int) []float32 {
	return a.ringBuffer.SnapshotLatest(sampleCount)
}

// IsRecording returns true if audio capture is active.
func (a *AudioService) IsRecording() bool {
	return a.isRecording.Load()
}

// GetStats returns current audio service statistics.
type AudioServiceStats struct {
	IsRecording   bool
	Uptime        time.Duration
	CallbackHits  uint64
	TotalFrames   uint64
	DroppedFrames uint64
	DropRate      float64 // Percentage (0.0 - 100.0)
	BufferStats   RingBufferStats
}

func (a *AudioService) GetStats() AudioServiceStats {
	stats := AudioServiceStats{
		IsRecording:   a.isRecording.Load(),
		CallbackHits:  a.callbackHits.Load(),
		TotalFrames:   a.totalFrames.Load(),
		DroppedFrames: a.droppedFrames.Load(),
		BufferStats:   a.ringBuffer.GetStats(),
	}

	if a.isRecording.Load() {
		stats.Uptime = time.Since(a.startTime)
	}

	if stats.TotalFrames > 0 {
		stats.DropRate = (float64(stats.DroppedFrames) / float64(stats.TotalFrames)) * 100.0
	}

	return stats
}

// ListDevices returns available audio capture devices.
// Useful for device selection UI.
func ListDevices(ctx context.Context) ([]DeviceInfo, error) {
	malgoCtx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to init malgo: %w", err)
	}
	defer func() {
		malgoCtx.Uninit()
		malgoCtx.Free()
	}()

	// Get capture devices
	infos, err := malgoCtx.Devices(malgo.Capture)
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate devices: %w", err)
	}

	devices := make([]DeviceInfo, 0, len(infos))
	for i, info := range infos {
		devices = append(devices, DeviceInfo{
			Index:     i,
			ID:        info.ID,
			Name:      info.Name(),
			IsDefault: info.IsDefault != 0,
		})
	}

	return devices, nil
}

// DeviceInfo describes an audio capture device
type DeviceInfo struct {
	Index     int
	ID        malgo.DeviceID
	Name      string
	IsDefault bool
}

func (d DeviceInfo) String() string {
	if d.IsDefault {
		return fmt.Sprintf("[%d] %s (default)", d.Index, d.Name)
	}
	return fmt.Sprintf("[%d] %s", d.Index, d.Name)
}
