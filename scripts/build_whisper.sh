#!/bin/bash
# scripts/build_whisper.sh
# Builds the whisper.cpp library as a shared object.

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WHISPER_ROOT="$PROJECT_ROOT/third_party/whisper.cpp"

echo "🔨 Building whisper.cpp..."

if [ ! -d "$WHISPER_ROOT" ]; then
    echo "❌ whisper.cpp directory not found. Did you clone submodules?"
    echo "   git submodule update --init --recursive"
    exit 1
fi

cd "$WHISPER_ROOT"

# Ensure clean build if needed, or incremental
# cmake -B build -DWHISPER_BUILD_SHARED=ON
# We strictly need SHARED libs for CGO linking usually, or static.
# The user instruction mentioned WHISPER_BUILD_SHARED=ON.

echo "   Configuring CMake..."
cmake -B build -DWHISPER_BUILD_SHARED=ON -DGGML_NATIVE=OFF 
# Note: GGML_NATIVE=OFF improves portability, ON optimizes for current CPU (AVX etc)
# Let's keep defaults or allow env override.

echo "   Compiling..."
cmake --build build -j$(nproc) --config Release

echo "✅ whisper.cpp built successfully."
echo "   Libs should be in $WHISPER_ROOT/build/src and $WHISPER_ROOT/build/ggml/src"
