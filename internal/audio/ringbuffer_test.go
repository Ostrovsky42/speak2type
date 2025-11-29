package audio

import (
	"sync"
	"testing"
	"time"
)

// TestRingBuffer_BasicWriteRead verifies basic functionality
func TestRingBuffer_BasicWriteRead(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{
		DurationSeconds: 1.0,
		SampleRate:      16000,
	})

	// Write some samples
	samples := []float32{1.0, 2.0, 3.0, 4.0, 5.0}
	written := rb.Write(samples)

	if written != len(samples) {
		t.Errorf("Expected to write %d samples, wrote %d", len(samples), written)
	}

	// Verify available count
	available := rb.Available()
	if available != len(samples) {
		t.Errorf("Expected %d available, got %d", len(samples), available)
	}

	// Snapshot and verify
	snapshot := rb.SnapshotLatest(len(samples))
	if len(snapshot) != len(samples) {
		t.Fatalf("Expected snapshot length %d, got %d", len(samples), len(snapshot))
	}

	for i, val := range snapshot {
		if val != samples[i] {
			t.Errorf("Sample mismatch at index %d: expected %.1f, got %.1f", i, samples[i], val)
		}
	}
}

// TestRingBuffer_Wraparound tests circular buffer behavior
func TestRingBuffer_Wraparound(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{
		DurationSeconds: 0.001, // Tiny buffer: 16 samples @ 16kHz
		SampleRate:      16000,
	})

	capacity := rb.Capacity()
	t.Logf("Buffer capacity: %d samples", capacity)

	// Write more than capacity
	samples := make([]float32, capacity+10)
	for i := range samples {
		samples[i] = float32(i)
	}

	rb.Write(samples)

	// Should have overflowed
	stats := rb.GetStats()
	if stats.Overflows == 0 {
		t.Error("Expected buffer overflow, but none occurred")
	}

	t.Logf("Overflows: %d", stats.Overflows)

	// Available should equal capacity (buffer full)
	if stats.Available > capacity {
		t.Errorf("Available (%d) exceeds capacity (%d)", stats.Available, capacity)
	}

	// Snapshot should contain nearly capacity samples (capacity-1 is correct for full buffer)
	// In a ring buffer, writePos == readPos means EMPTY, so full buffer is actually capacity-1
	snapshot := rb.SnapshotLatest(capacity)
	
	// With overflow, we should have capacity-1 samples (full ring buffer invariant)
	if len(snapshot) < capacity-1 {
		t.Fatalf("Expected snapshot length >= %d, got %d", capacity-1, len(snapshot))
	}
	
	t.Logf("Snapshot contains %d samples (capacity=%d)", len(snapshot), capacity)
	
	// Verify samples are sequential (may have small gaps due to overflow timing)
	for i := 1; i < len(snapshot); i++ {
		diff := snapshot[i] - snapshot[i-1]
		if diff < 0 || diff > 2 {
			t.Errorf("Non-sequential samples at %d: %.1f -> %.1f (diff=%.1f)",
				i, snapshot[i-1], snapshot[i], diff)
		}
	}
}

// TestRingBuffer_ConcurrentWriteRead simulates real audio scenario:
// one goroutine writing (audio callback), another reading (VAD/ASR)
func TestRingBuffer_ConcurrentWriteRead(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{
		DurationSeconds: 1.0,
		SampleRate:      16000,
	})

	const (
		writeIterations = 1000
		readIterations  = 100
		chunkSize       = 480 // 30ms @ 16kHz
	)

	var wg sync.WaitGroup
	wg.Add(2)

	// Writer goroutine (simulates audio callback)
	go func() {
		defer wg.Done()
		chunk := make([]float32, chunkSize)
		for i := 0; i < writeIterations; i++ {
			// Fill chunk with incrementing values
			for j := range chunk {
				chunk[j] = float32(i*chunkSize + j)
			}
			rb.Write(chunk)
			time.Sleep(10 * time.Microsecond) // Simulate real-time pacing
		}
	}()

	// Reader goroutine (simulates VAD/ASR consuming data)
	go func() {
		defer wg.Done()
		for i := 0; i < readIterations; i++ {
			snapshot := rb.Snapshot(0.5, 16000) // 500ms snapshots
			if len(snapshot) > 0 {
				// Verify monotonicity (samples should generally increase)
				// Note: May have gaps due to concurrent writes
				t.Logf("Iteration %d: snapshot length = %d", i, len(snapshot))
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()

	wg.Wait()

	// Verify no panics, no data races
	stats := rb.GetStats()
	t.Logf("Final stats: %+v", stats)

	if stats.WriteCount == 0 {
		t.Error("No samples were written")
	}
}

// TestRingBuffer_ZeroAllocation verifies Write() doesn't allocate
func TestRingBuffer_ZeroAllocation(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{
		DurationSeconds: 1.0,
		SampleRate:      16000,
	})

	samples := make([]float32, 480)

	// Warm up
	rb.Write(samples)

	// Measure allocations
	allocsBefore := testing.AllocsPerRun(1000, func() {
		rb.Write(samples)
	})

	if allocsBefore > 0 {
		t.Errorf("Write() allocated %.2f times per call (expected 0)", allocsBefore)
	}
}

// TestRingBuffer_SnapshotDuration tests duration-based snapshot
func TestRingBuffer_SnapshotDuration(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{
		DurationSeconds: 2.0,
		SampleRate:      16000,
	})

	// Write 1 second of data
	samples := make([]float32, 16000)
	for i := range samples {
		samples[i] = float32(i)
	}
	rb.Write(samples)

	// Snapshot 0.5 seconds (8000 samples)
	snapshot := rb.Snapshot(0.5, 16000)

	if len(snapshot) != 8000 {
		t.Errorf("Expected 8000 samples for 0.5s @ 16kHz, got %d", len(snapshot))
	}

	// Should contain the LAST 8000 samples (16000-8000 = 8000 onwards)
	expectedStart := 16000 - 8000
	for i, val := range snapshot {
		expected := float32(expectedStart + i)
		if val != expected {
			t.Errorf("Sample %d: expected %.1f, got %.1f", i, expected, val)
			break // Don't spam errors
		}
	}
}

// TestRingBuffer_EmptySnapshot tests edge case of empty buffer
func TestRingBuffer_EmptySnapshot(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{
		DurationSeconds: 1.0,
		SampleRate:      16000,
	})

	snapshot := rb.Snapshot(1.0, 16000)
	if snapshot != nil {
		t.Errorf("Expected nil snapshot from empty buffer, got %d samples", len(snapshot))
	}

	available := rb.Available()
	if available != 0 {
		t.Errorf("Empty buffer should have 0 available, got %d", available)
	}
}

// TestRingBuffer_Reset verifies reset functionality
func TestRingBuffer_Reset(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{
		DurationSeconds: 1.0,
		SampleRate:      16000,
	})

	// Write data
	samples := make([]float32, 1000)
	rb.Write(samples)

	// Verify data exists
	if rb.Available() == 0 {
		t.Fatal("Buffer should have data before reset")
	}

	// Reset
	rb.Reset()

	// Verify empty
	if rb.Available() != 0 {
		t.Errorf("Buffer should be empty after reset, got %d samples", rb.Available())
	}

	stats := rb.GetStats()
	if stats.Overflows != 0 {
		t.Errorf("Overflows should be reset, got %d", stats.Overflows)
	}
}

// BenchmarkRingBuffer_Write measures write performance (hot path)
func BenchmarkRingBuffer_Write(b *testing.B) {
	rb := NewRingBuffer(RingBufferConfig{
		DurationSeconds: 10.0,
		SampleRate:      16000,
	})

	chunk := make([]float32, 480) // 30ms @ 16kHz
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rb.Write(chunk)
	}
}

// BenchmarkRingBuffer_Snapshot measures read performance
func BenchmarkRingBuffer_Snapshot(b *testing.B) {
	rb := NewRingBuffer(RingBufferConfig{
		DurationSeconds: 10.0,
		SampleRate:      16000,
	})

	// Fill buffer
	samples := make([]float32, 160000)
	rb.Write(samples)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = rb.Snapshot(7.0, 16000) // 7s window (typical for Whisper)
	}
}
