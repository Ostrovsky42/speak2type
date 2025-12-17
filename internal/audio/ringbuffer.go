// Package audio provides real-time audio capture and buffering primitives.
// This package is designed for production use with strict zero-allocation guarantees
// in the hot path (audio callbacks).
package audio

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// RingBuffer is a thread-safe circular buffer for audio samples.
// It implements a zero-copy design for write operations and provides
// snapshot capabilities for reading without blocking the writer.
//
// Design constraints:
// - Pre-allocated fixed-size buffer (no dynamic growth)
// - Write operations MUST NOT allocate heap memory
// - Read operations create defensive copies (acceptable overhead)
// - Thread-safe for concurrent single-writer, multiple-reader scenarios
type RingBuffer struct {
	// Pre-allocated storage
	data []float32
	size int

	// Atomic position counters (avoid mutex in hot path)
	writePos atomic.Int64
	readPos  atomic.Int64

	// Mutex for snapshot operations only
	// NOT used in Write() to maintain zero-allocation guarantee
	mutex sync.Mutex

	// Statistics
	overflows  atomic.Uint64 // Count of buffer overflow events
	writeCount atomic.Uint64 // Total samples written
}

// RingBufferConfig defines ring buffer initialization parameters
type RingBufferConfig struct {
	DurationSeconds float64 // Buffer duration (e.g., 10.0 for 10 seconds)
	SampleRate      int     // Samples per second (e.g., 16000 Hz)
}

// NewRingBuffer creates a pre-allocated ring buffer.
// This is the ONLY allocation that occurs - all subsequent operations
// must not allocate additional memory in the hot path.
//
// Example:
//
//	buf := NewRingBuffer(RingBufferConfig{
//	    DurationSeconds: 10.0,
//	    SampleRate:      16000,
//	})
func NewRingBuffer(config RingBufferConfig) *RingBuffer {
	size := int(config.DurationSeconds * float64(config.SampleRate))
	if size <= 0 {
		panic("ring buffer size must be positive")
	}

	rb := &RingBuffer{
		data: make([]float32, size), // Single allocation
		size: size,
	}

	rb.writePos.Store(0)
	rb.readPos.Store(0)

	return rb
}

// Write appends samples to the ring buffer.
// CRITICAL: This function is called from audio callback thread.
// It MUST NOT allocate heap memory (but uses mutex for thread-safety).
//
// Thread-safety: Safe for concurrent writers and readers.
// Performance: O(n) where n = len(samples), mutex overhead is minimal.
//
// Returns:
//   - Number of samples actually written
//   - If buffer overflows, oldest data is overwritten
func (rb *RingBuffer) Write(samples []float32) int {
	if len(samples) == 0 {
		return 0
	}

	// Use mutex to prevent race on position updates
	rb.mutex.Lock()
	defer rb.mutex.Unlock()

	written := 0
	wPos := int(rb.writePos.Load())
	rPos := int(rb.readPos.Load())

	for _, sample := range samples {
		rb.data[wPos] = sample
		wPos = (wPos + 1) % rb.size
		written++

		// Overflow detection: write caught up to read
		if wPos == rPos {
			// Advance read position to maintain buffer invariants
			rPos = (rPos + 1) % rb.size
			rb.readPos.Store(int64(rPos))
			rb.overflows.Add(1)
		}
	}

	// Update write position
	rb.writePos.Store(int64(wPos))
	rb.writeCount.Add(uint64(written))

	return written
}

// Snapshot creates a copy of the last N samples from the buffer.
// This operation allocates a new slice (intentional - safe for reads).
//
// Args:
//   - durationSeconds: How many seconds of audio to extract
//   - sampleRate: Expected sample rate (for validation)
//
// Returns:
//   - []float32: Copy of requested samples (oldest to newest)
//   - Safe to use in separate goroutine without blocking writer
//
// Thread-safety: Safe for concurrent access with Write().
func (rb *RingBuffer) Snapshot(durationSeconds float64, sampleRate int) []float32 {
	rb.mutex.Lock()
	defer rb.mutex.Unlock()

	requestedSamples := int(durationSeconds * float64(sampleRate))
	if requestedSamples <= 0 {
		return nil
	}

	// Clamp to available data
	available := rb.Available()
	if requestedSamples > available {
		requestedSamples = available
	}

	if requestedSamples == 0 {
		return nil
	}

	// Allocate result buffer (acceptable for read operations)
	result := make([]float32, requestedSamples)

	wPos := int(rb.writePos.Load())
	startPos := (wPos - requestedSamples + rb.size) % rb.size

	// Copy samples in circular order
	for i := 0; i < requestedSamples; i++ {
		pos := (startPos + i) % rb.size
		result[i] = rb.data[pos]
	}

	return result
}

// SnapshotLatest extracts the most recent samples (convenience wrapper).
// Equivalent to Snapshot() but uses sample count instead of duration.
func (rb *RingBuffer) SnapshotLatest(sampleCount int) []float32 {
	rb.mutex.Lock()
	defer rb.mutex.Unlock()

	if sampleCount <= 0 {
		return nil
	}

	available := rb.Available()
	if sampleCount > available {
		sampleCount = available
	}

	if sampleCount == 0 {
		return nil
	}

	result := make([]float32, sampleCount)
	wPos := int(rb.writePos.Load())
	startPos := (wPos - sampleCount + rb.size) % rb.size

	for i := 0; i < sampleCount; i++ {
		pos := (startPos + i) % rb.size
		result[i] = rb.data[pos]
	}

	return result
}

// Read consumes samples from the buffer into the provided slice.
// Advances the read position, freeing up space.
//
// Returns:
//   - Number of samples actually read
func (rb *RingBuffer) Read(out []float32) int {
	rb.mutex.Lock()
	defer rb.mutex.Unlock()

	available := rb.Available()
	if available == 0 {
		return 0
	}

	toRead := len(out)
	if toRead > available {
		toRead = available
	}

	rPos := int(rb.readPos.Load())

	// Copy samples
	for i := 0; i < toRead; i++ {
		out[i] = rb.data[rPos]
		rPos = (rPos + 1) % rb.size
	}

	rb.readPos.Store(int64(rPos))
	return toRead
}

// Available returns the number of samples currently buffered.
// This is an estimate and may change between call and use.
func (rb *RingBuffer) Available() int {
	wPos := int(rb.writePos.Load())
	rPos := int(rb.readPos.Load())

	if wPos >= rPos {
		return wPos - rPos
	}
	return rb.size - (rPos - wPos)
}

// Capacity returns the maximum buffer capacity in samples.
func (rb *RingBuffer) Capacity() int {
	return rb.size
}

// Stats returns current buffer statistics.
type RingBufferStats struct {
	Capacity    int     // Total buffer size
	Available   int     // Currently buffered samples
	Overflows   uint64  // Number of overflow events
	WriteCount  uint64  // Total samples written
	Utilization float64 // Percentage full (0.0 - 1.0)
}

// GetStats returns current buffer statistics (useful for monitoring).
func (rb *RingBuffer) GetStats() RingBufferStats {
	available := rb.Available()
	return RingBufferStats{
		Capacity:    rb.size,
		Available:   available,
		Overflows:   rb.overflows.Load(),
		WriteCount:  rb.writeCount.Load(),
		Utilization: float64(available) / float64(rb.size),
	}
}

// Reset clears the buffer and resets counters.
// Not safe to call concurrently with Write().
func (rb *RingBuffer) Reset() {
	rb.mutex.Lock()
	defer rb.mutex.Unlock()

	rb.writePos.Store(0)
	rb.readPos.Store(0)
	rb.overflows.Store(0)
	// Note: writeCount is cumulative, not reset
}

// String implements fmt.Stringer for debugging.
func (rb *RingBuffer) String() string {
	stats := rb.GetStats()
	return fmt.Sprintf("RingBuffer{cap=%d, avail=%d, util=%.1f%%, overflows=%d}",
		stats.Capacity, stats.Available, stats.Utilization*100, stats.Overflows)
}
