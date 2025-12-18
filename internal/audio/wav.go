package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// ReadWavFile is a minimal WAV reader for 16kHz Mono 16-bit PCM files.
// Used primarily for regression testing and E2E validation.
func ReadWavFile(path string) ([]float32, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	header := make([]byte, 44)
	if _, err := io.ReadFull(f, header); err != nil {
		return nil, fmt.Errorf("failed to read wav header: %w", err)
	}

	// Basic sanity check: "RIFF" and "WAVE"
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return nil, fmt.Errorf("not a valid RIFF/WAVE file")
	}

	// Read PCM data
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	// Convert 16-bit PCM (little endian) to float32
	sampleCount := len(data) / 2
	samples := make([]float32, sampleCount)
	for i := 0; i < sampleCount; i++ {
		bits := binary.LittleEndian.Uint16(data[i*2 : i*2+2])
		// Convert to signed int16
		val := int16(bits)
		// Normalize to float32 [-1.0, 1.0]
		samples[i] = float32(val) / 32768.0
	}

	return samples, nil
}
