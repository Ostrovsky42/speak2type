#!/usr/bin/env bash
# Downloads pinned external shared libraries to third_party/lib.

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LIB_DIR="$PROJECT_ROOT/third_party/lib"
mkdir -p "$LIB_DIR"

ONNX_VERSION="${ONNX_VERSION:-1.20.0}"
ONNX_LIB_SHA256="${ONNX_LIB_SHA256:-6097fe8cedc8b5b3c8e107e9c2acf04eb50f58f0f045e3d7c5c50ead38112c72}"
OS="linux"
ARCH="x64"

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

if [ -f "$LIB_DIR/libonnxruntime.so.$ONNX_VERSION" ]; then
    verify_sha256 "$LIB_DIR/libonnxruntime.so.$ONNX_VERSION" "$ONNX_LIB_SHA256"
    echo "✅ ONNX Runtime $ONNX_VERSION already present."
    exit 0
fi

echo "⬇️  Downloading ONNX Runtime v$ONNX_VERSION..."
URL="https://github.com/microsoft/onnxruntime/releases/download/v${ONNX_VERSION}/onnxruntime-${OS}-${ARCH}-${ONNX_VERSION}.tgz"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

curl -fL --retry 3 "$URL" -o "$TMP_DIR/onnx.tgz"
tar -xzf "$TMP_DIR/onnx.tgz" -C "$TMP_DIR"
EXTRACTED_DIR="$TMP_DIR/onnxruntime-${OS}-${ARCH}-${ONNX_VERSION}"

cp "$EXTRACTED_DIR/lib/"libonnxruntime.so* "$LIB_DIR/"
verify_sha256 "$LIB_DIR/libonnxruntime.so.$ONNX_VERSION" "$ONNX_LIB_SHA256"

echo "✅ ONNX Runtime libraries installed to $LIB_DIR"
ls -lh "$LIB_DIR"
