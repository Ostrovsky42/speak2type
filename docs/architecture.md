# Architecture Notes

## Model Strategy

Speak2Type keeps runtime model files outside Git. `make deps` downloads pinned artifacts into `models/` and `third_party/`, then verifies SHA256 checksums. A fresh clone is not runnable until `make deps` completes successfully.

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


## Model Provenance

| Artifact | Source | Pin | SHA256 |
| :--- | :--- | :--- | :--- |
| `models/silero_vad.onnx` | `https://github.com/snakers4/silero-vad/raw/v5.1.2/files/silero_vad.onnx` | `v5.1.2` | `1a153a22f4509e292a94e67d6f9b85e8deb25b4988682b7e174c65279d8788e3` |
| `models/silero_vad_v4.onnx` | `https://github.com/snakers4/silero-vad/raw/v4.0/files/silero_vad.onnx` | `v4.0` | `a35ebf52fd3ce5f1469b2a36158dba761bc47b973ea3382b3186ca15b1f5af28` |
| `models/ggml-base.bin` | `https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin` | `main` | `60ed5bc3dd14eea856493d334349b405782ddcaf0028d4b5df4088345fba2efe` |
| `third_party/lib/libonnxruntime.so.1.20.0` | `https://github.com/microsoft/onnxruntime/releases/download/v1.20.0/onnxruntime-linux-x64-1.20.0.tgz` | `1.20.0` | `6097fe8cedc8b5b3c8e107e9c2acf04eb50f58f0f045e3d7c5c50ead38112c72` |
| `third_party/lib/libonnxruntime.dylib` (macOS x86_64) | `https://github.com/microsoft/onnxruntime/releases/download/v1.20.0/onnxruntime-osx-x86_64-1.20.0.tgz` | `1.20.0` | dylib: `542ffd4568821088ff3e42a3aa19c37dbbd73b522bfe58505520de332e581b4d`; archive: `d28e603b47b74050f2c30a7069bf3fb371cfba7205d7771f22cabc7b02953757` |
| `third_party/lib/libonnxruntime.dylib` (macOS arm64) | `https://github.com/microsoft/onnxruntime/releases/download/v1.20.0/onnxruntime-osx-arm64-1.20.0.tgz` | `1.20.0` | dylib: `d8be733cb8dd097cfe2b21e069a7462b5ff561625141d9c4b98d866f15bfb852`; archive: `2bcfaafa9ff0a3a94f78e3af2f135ffde5bb2d79b08e83a50dbc450b0d20ddae` |
| `third_party/whisper.cpp` | `https://github.com/ggerganov/whisper.cpp.git` | `19ceec8eac980403b714d603e5ca31653cd42a3f` | Git commit pin |

Silero VAD v6.x is tracked as a candidate upgrade only. Its newer ONNX graph variants must be validated against `internal/vad` before changing the default download.
