#!/usr/bin/env bash
# Synchronizes whisper.cpp to the pinned commit that matches the Go binding.

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WHISPER_ROOT="$PROJECT_ROOT/third_party/whisper.cpp"
WHISPER_CPP_URL="${WHISPER_CPP_URL:-https://github.com/ggerganov/whisper.cpp.git}"
WHISPER_CPP_REF="${WHISPER_CPP_REF:-19ceec8eac980403b714d603e5ca31653cd42a3f}"

mkdir -p "$PROJECT_ROOT/third_party"

if [ -d "$WHISPER_ROOT/.git" ]; then
    echo "🔁 Updating whisper.cpp to $WHISPER_CPP_REF..."
    git -C "$WHISPER_ROOT" fetch --depth 1 origin "$WHISPER_CPP_REF"
    git -C "$WHISPER_ROOT" checkout --detach "$WHISPER_CPP_REF"
elif [ -d "$WHISPER_ROOT" ]; then
    if [ -f "$WHISPER_ROOT/include/whisper.h" ]; then
        echo "⚠️  Existing non-git whisper.cpp tree found; cannot verify pin $WHISPER_CPP_REF."
        echo "   Remove third_party/whisper.cpp to let this script clone the pinned source."
        exit 0
    fi
    echo "❌ third_party/whisper.cpp exists but is not a usable checkout."
    exit 1
else
    echo "⬇️  Cloning whisper.cpp at $WHISPER_CPP_REF..."
    git init "$WHISPER_ROOT"
    git -C "$WHISPER_ROOT" remote add origin "$WHISPER_CPP_URL"
    git -C "$WHISPER_ROOT" fetch --depth 1 origin "$WHISPER_CPP_REF"
    git -C "$WHISPER_ROOT" checkout --detach FETCH_HEAD
fi

git -C "$WHISPER_ROOT" submodule update --init --recursive
echo "$WHISPER_CPP_REF" > "$WHISPER_ROOT/.speak2type-ref"
echo "✅ whisper.cpp ready at $WHISPER_CPP_REF"
