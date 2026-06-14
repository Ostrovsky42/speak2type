#!/usr/bin/env bash
# Builds the pinned whisper.cpp library as shared objects for CGO.

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WHISPER_ROOT="$PROJECT_ROOT/third_party/whisper.cpp"
LIB_DIR="$PROJECT_ROOT/third_party/lib"

echo "🔨 Building whisper.cpp..."

if [ ! -f "$WHISPER_ROOT/include/whisper.h" ]; then
    echo "❌ whisper.cpp headers not found. Run: ./scripts/sync_whisper.sh"
    exit 1
fi

mkdir -p "$LIB_DIR"
cd "$WHISPER_ROOT"

echo "   Configuring CMake..."
cmake -B build -DWHISPER_BUILD_SHARED=ON -DGGML_NATIVE=OFF

echo "   Compiling..."
JOBS="${JOBS:-$(getconf _NPROCESSORS_ONLN 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 2)}"
cmake --build build -j"$JOBS" --config Release

echo "   Copying shared libraries to $LIB_DIR..."
find build -type f -o -type l | while read -r file; do
    case "$(basename "$file")" in
        libwhisper.so*|libwhisper.dylib*|libggml.so*|libggml.dylib*|libggml-base.so*|libggml-base.dylib*|libggml-cpu.so*|libggml-cpu.dylib*)
            cp -L "$file" "$LIB_DIR/"
            ;;
    esac
done

echo "✅ whisper.cpp built successfully."
ls -lh "$LIB_DIR"/libwhisper.* "$LIB_DIR"/libggml*.*
