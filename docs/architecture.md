# Architecture Notes

## Model Strategy

Speak2Type keeps runtime model files outside Git. `make deps` downloads pinned artifacts into `models/` and `third_party/`, then verifies checksums where practical.

- VAD primary model: Silero VAD v5 at `models/silero_vad.onnx`.
- VAD fallback model: Silero VAD v4 at `models/silero_vad_v4.onnx`.
- Silero upstream latest is v6.x, but v6 is not declared supported here until its ONNX input/output signature is validated against `internal/vad`.
- ASR default model: `ggml-base.bin`, kept small enough for CI and development.
- Recommended production ASR model for Russian: `ggml-large-v3-turbo` or a quantized variant. Use `speak2type run --asr-model <path>` or `asr.model_path` in config.

## Native Boundary Risk

ASR currently uses the official `github.com/ggerganov/whisper.cpp/bindings/go` package. This is acceptable for the Linux MVP, but it crosses the Go/C boundary through a broad binding surface. The next ASR performance step is a thin project-owned wrapper around only the calls Speak2Type needs:

- context/model lifecycle;
- synchronous full-window transcription;
- language selection;
- segment text extraction;
- no callback-heavy streaming path in the hot loop.

The wrapper should be benchmarked against the current binding before replacing it.
