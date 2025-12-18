#!/bin/bash
# scripts/download_models.sh
# Downloads Silero VAD (v4 and v5) and Whisper GGML models.

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODELS_DIR="$PROJECT_ROOT/models"
mkdir -p "$MODELS_DIR"

echo "📂 Models directory: $MODELS_DIR"

# 1. Silero VAD v5 (Latest)
VAD_V5_TARGET="$MODELS_DIR/silero_vad.onnx"
if [ ! -f "$VAD_V5_TARGET" ]; then
    echo "⬇️  Downloading Silero VAD v5..."
    curl -L "https://github.com/snakers4/silero-vad/raw/master/files/silero_vad.onnx" -o "$VAD_V5_TARGET"
else
    echo "✅ Silero VAD v5 found."
fi

# 2. Silero VAD v4 (Fallback)
VAD_V4_TARGET="$MODELS_DIR/silero_vad_v4.onnx"
if [ ! -f "$VAD_V4_TARGET" ]; then
    echo "⬇️  Downloading Silero VAD v4..."
    curl -L "https://github.com/snakers4/silero-vad/raw/v4.0/files/silero_vad.onnx" -o "$VAD_V4_TARGET"
else
    echo "✅ Silero VAD v4 found."
fi

# 3. Whisper GGML Base Model
WHISPER_TARGET="$MODELS_DIR/ggml-base.bin"
if [ ! -f "$WHISPER_TARGET" ]; then
    echo "⬇️  Downloading Whisper GGML Base model..."
    curl -L "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin" -o "$WHISPER_TARGET"
else
    echo "✅ Whisper model found."
fi

echo "✨ All models ready in $MODELS_DIR"
ls -lh "$MODELS_DIR"
