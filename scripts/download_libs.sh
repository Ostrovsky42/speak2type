#!/bin/bash
# scripts/download_libs.sh
# Downloads external shared libraries (e.g. ONNX Runtime) to third_party.

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LIB_DIR="$PROJECT_ROOT/third_party/lib"
mkdir -p "$LIB_DIR"

# ONNX Runtime Version
ONNX_VERSION="1.20.0"
OS="linux" # Detect linux/osx? For now assume linux per requirements
ARCH="x64" # Assume x64 for now

# Check if already present
if [ -f "$LIB_DIR/libonnxruntime.so.$ONNX_VERSION" ]; then
    echo "✅ ONNX Runtime $ONNX_VERSION already present."
    exit 0
fi

echo "⬇️  Downloading ONNX Runtime v$ONNX_VERSION..."

URL="https://github.com/microsoft/onnxruntime/releases/download/v${ONNX_VERSION}/onnxruntime-${OS}-${ARCH}-${ONNX_VERSION}.tgz"
TMP_DIR=$(mktemp -d)

wget -q -O "$TMP_DIR/onnx.tgz" "$URL"
tar -xzf "$TMP_DIR/onnx.tgz" -C "$TMP_DIR"

# Move libs
# The tar usually contains a folder like onnxruntime-linux-x64-1.16.3/lib/
EXTRACTED_DIR="$TMP_DIR/onnxruntime-${OS}-${ARCH}-${ONNX_VERSION}"

cp "$EXTRACTED_DIR/lib/"libonnxruntime.so* "$LIB_DIR/"

# Cleanup
rm -rf "$TMP_DIR"

echo "✅ ONNX Runtime libraries installed to $LIB_DIR"
ls -lh "$LIB_DIR"
