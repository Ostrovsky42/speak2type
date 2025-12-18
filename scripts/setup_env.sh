#!/bin/bash
# scripts/setup_env.sh
# Usage: source scripts/setup_env.sh
# Sets up environment variables for building and running Speak2Type with CGO.

# Get the absolute path to the project root
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WHISPER_ROOT="$PROJECT_ROOT/third_party/whisper.cpp"

if [ ! -d "$WHISPER_ROOT" ]; then
    echo "❌ Error: whisper.cpp not found at $WHISPER_ROOT"
    echo "   Please run: ./scripts/build_whisper.sh"
    return 1
fi

# CGO Compilation Flags
export CGO_CFLAGS="-I$WHISPER_ROOT/include -I$WHISPER_ROOT/ggml/include"
export CGO_LDFLAGS="-L$WHISPER_ROOT/build/src -L$WHISPER_ROOT/build/ggml/src -lwhisper -lggml -lstdc++ -lm"


# Runtime Library Path
# We include third_party/lib for ONNX Runtime (Variant A: Bundled)
export LD_LIBRARY_PATH="$WHISPER_ROOT/build/src:$WHISPER_ROOT/build/ggml/src:$PROJECT_ROOT/third_party/lib:$LD_LIBRARY_PATH"

# macOS
export DYLD_LIBRARY_PATH="$WHISPER_ROOT/build/src:$WHISPER_ROOT/build/ggml/src:$PROJECT_ROOT/third_party/lib:$DYLD_LIBRARY_PATH"

# Check for ONNX Runtime (Variant A)
if [ ! -f "$PROJECT_ROOT/third_party/lib/libonnxruntime.so" ] && [ ! -f "$PROJECT_ROOT/third_party/lib/libonnxruntime.so.1.20.0" ]; then
    echo "⚠️  Warning: libonnxruntime.so not found in third_party/lib"
    echo "   VAD might fail. Run: ./scripts/download_libs.sh"
fi

# Check for whisper build
if [ ! -f "$WHISPER_ROOT/build/src/libwhisper.so" ]; then
    echo "⚠️  Warning: libwhisper.so not found in $WHISPER_ROOT/build/src"
    echo "   ASR will fail. Run: ./scripts/build_whisper.sh"
fi

echo "✅ Environment variables set for Speak2Type."
echo "   CGO_CFLAGS=$CGO_CFLAGS"
echo "   LD_LIBRARY_PATH=$LD_LIBRARY_PATH"
