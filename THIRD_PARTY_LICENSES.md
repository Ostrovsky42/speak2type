# Third-Party Licenses

Speak2Type is licensed under the MIT License. The project also depends on third-party open-source components and model artifacts. Their copyright notices and license terms must be preserved when redistributing source or release artifacts.

## Silero VAD

- Project: Silero VAD
- Upstream: https://github.com/snakers4/silero-vad
- Copyright: Silero Team
- License: MIT License
- Used artifacts:
  - `models/silero_vad.onnx` from `snakers4/silero-vad` tag `v5.1.2`
  - `models/silero_vad_v4.onnx` from `snakers4/silero-vad` tag `v4.0`

## Whisper

- Project: Whisper
- Upstream: https://github.com/openai/whisper
- Copyright: OpenAI
- License: MIT License
- Used artifacts: Whisper model weights converted to GGML format for whisper.cpp compatibility.

## whisper.cpp and ggml

- Project: whisper.cpp / ggml
- Upstream: https://github.com/ggerganov/whisper.cpp
- Copyright: Georgi Gerganov and contributors
- License: MIT License
- Used for: native ASR runtime and Go bindings.

## ONNX Runtime

- Project: ONNX Runtime
- Upstream: https://github.com/microsoft/onnxruntime
- Copyright: Microsoft Corporation
- License: MIT License
- Used for: Silero VAD ONNX inference runtime.

## Release Packaging Note

Binary release artifacts should include this file and `LICENSE` alongside the executable, bundled libraries, and model files.
