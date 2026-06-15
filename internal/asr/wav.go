package asr

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

func encodePCM16WAV(samples []float32, sampleRate int) ([]byte, error) {
	if sampleRate <= 0 {
		return nil, fmt.Errorf("sample rate must be positive")
	}
	dataSize := len(samples) * 2
	fileSize := 36 + dataSize

	buf := bytes.NewBuffer(make([]byte, 0, 44+dataSize))
	buf.WriteString("RIFF")
	_ = binary.Write(buf, binary.LittleEndian, uint32(fileSize))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	_ = binary.Write(buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(buf, binary.LittleEndian, uint32(sampleRate*2))
	_ = binary.Write(buf, binary.LittleEndian, uint16(2))
	_ = binary.Write(buf, binary.LittleEndian, uint16(16))
	buf.WriteString("data")
	_ = binary.Write(buf, binary.LittleEndian, uint32(dataSize))

	for _, sample := range samples {
		_ = binary.Write(buf, binary.LittleEndian, floatToPCM16(sample))
	}

	return buf.Bytes(), nil
}

func floatToPCM16(sample float32) int16 {
	if math.IsNaN(float64(sample)) {
		return 0
	}
	if sample > 1 {
		sample = 1
	} else if sample < -1 {
		sample = -1
	}
	if sample < 0 {
		return int16(sample * 32768)
	}
	return int16(sample * 32767)
}
