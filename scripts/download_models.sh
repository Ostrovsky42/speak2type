#!/usr/bin/env bash
# Downloads checksum-pinned Silero VAD and Whisper GGML models.

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODELS_DIR="$PROJECT_ROOT/models"
mkdir -p "$MODELS_DIR"

SILERO_V5_REF="${SILERO_V5_REF:-v5.1.2}"
SILERO_V5_SHA256="${SILERO_V5_SHA256:-1a153a22f4509e292a94e67d6f9b85e8deb25b4988682b7e174c65279d8788e3}"
SILERO_V4_REF="${SILERO_V4_REF:-v4.0}"
SILERO_V4_SHA256="${SILERO_V4_SHA256:-a35ebf52fd3ce5f1469b2a36158dba761bc47b973ea3382b3186ca15b1f5af28}"
WHISPER_MODEL_REF="${WHISPER_MODEL_REF:-main}"
WHISPER_BASE_SHA256="${WHISPER_BASE_SHA256:-60ed5bc3dd14eea856493d334349b405782ddcaf0028d4b5df4088345fba2efe}"

verify_sha256() {
    local file="$1"
    local expected="$2"
    local actual
    actual="$(sha256sum "$file" | awk '{print $1}')"
    if [ "$actual" != "$expected" ]; then
        echo "❌ Checksum mismatch for $file"
        echo "   expected: $expected"
        echo "   actual:   $actual"
        exit 1
    fi
}

download_if_missing() {
    local label="$1"
    local url="$2"
    local target="$3"
    local sha="$4"

    if [ ! -f "$target" ]; then
        echo "⬇️  Downloading $label..."
        curl -fL --retry 3 "$url" -o "$target"
    else
        echo "✅ $label found."
    fi
    verify_sha256 "$target" "$sha"
}

echo "📂 Models directory: $MODELS_DIR"

download_if_missing \
    "Silero VAD v5 ($SILERO_V5_REF)" \
    "https://github.com/snakers4/silero-vad/raw/${SILERO_V5_REF}/files/silero_vad.onnx" \
    "$MODELS_DIR/silero_vad.onnx" \
    "$SILERO_V5_SHA256"

download_if_missing \
    "Silero VAD v4 ($SILERO_V4_REF)" \
    "https://github.com/snakers4/silero-vad/raw/${SILERO_V4_REF}/files/silero_vad.onnx" \
    "$MODELS_DIR/silero_vad_v4.onnx" \
    "$SILERO_V4_SHA256"

download_if_missing \
    "Whisper GGML base model" \
    "https://huggingface.co/ggerganov/whisper.cpp/resolve/${WHISPER_MODEL_REF}/ggml-base.bin" \
    "$MODELS_DIR/ggml-base.bin" \
    "$WHISPER_BASE_SHA256"

echo "✨ All models ready in $MODELS_DIR"
ls -lh "$MODELS_DIR"
