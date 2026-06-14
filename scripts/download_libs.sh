#!/usr/bin/env bash
# Downloads pinned external shared libraries to third_party/lib.

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LIB_DIR="$PROJECT_ROOT/third_party/lib"
mkdir -p "$LIB_DIR"

ONNX_VERSION="${ONNX_VERSION:-1.20.0}"

case "$(uname -s)" in
    Linux)
        OS="linux"
        LIB_NAME="libonnxruntime.so"
        LIB_SHA256="6097fe8cedc8b5b3c8e107e9c2acf04eb50f58f0f045e3d7c5c50ead38112c72"
        ;;
    Darwin)
        OS="osx"
        LIB_NAME="libonnxruntime.dylib"
        ;;
    *)
        echo "❌ Unsupported OS: $(uname -s)"
        exit 1
        ;;
esac

case "$(uname -m)" in
    x86_64|amd64)
        if [ "$OS" = "linux" ]; then ARCH="x64"; else ARCH="x86_64"; fi
        [ "$OS" != "osx" ] || LIB_SHA256="542ffd4568821088ff3e42a3aa19c37dbbd73b522bfe58505520de332e581b4d"
        [ "$OS" != "osx" ] || ARCHIVE_SHA256="d28e603b47b74050f2c30a7069bf3fb371cfba7205d7771f22cabc7b02953757"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        if [ "$OS" != "osx" ]; then
            echo "❌ Unsupported Linux architecture for pinned ONNX Runtime: $(uname -m)"
            exit 1
        fi
        LIB_SHA256="d8be733cb8dd097cfe2b21e069a7462b5ff561625141d9c4b98d866f15bfb852"
        ARCHIVE_SHA256="2bcfaafa9ff0a3a94f78e3af2f135ffde5bb2d79b08e83a50dbc450b0d20ddae"
        ;;
    *)
        echo "❌ Unsupported architecture: $(uname -m)"
        exit 1
        ;;
esac

sha256_file() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | awk '{print $1}'
    else
        shasum -a 256 "$1" | awk '{print $1}'
    fi
}

verify_sha256() {
    local file="$1"
    local expected="$2"
    local actual
    actual="$(sha256_file "$file")"
    if [ "$actual" != "$expected" ]; then
        echo "❌ Checksum mismatch for $file"
        echo "   expected: $expected"
        echo "   actual:   $actual"
        exit 1
    fi
}

if [ -f "$LIB_DIR/$LIB_NAME" ]; then
    verify_sha256 "$LIB_DIR/$LIB_NAME" "$LIB_SHA256"
    echo "✅ ONNX Runtime $ONNX_VERSION already present."
    exit 0
fi

echo "⬇️  Downloading ONNX Runtime v$ONNX_VERSION for $OS/$ARCH..."
ARCHIVE="onnxruntime-${OS}-${ARCH}-${ONNX_VERSION}.tgz"
URL="https://github.com/microsoft/onnxruntime/releases/download/v${ONNX_VERSION}/${ARCHIVE}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

curl -fL --retry 3 "$URL" -o "$TMP_DIR/onnx.tgz"
if [ -n "${ARCHIVE_SHA256:-}" ]; then
    verify_sha256 "$TMP_DIR/onnx.tgz" "$ARCHIVE_SHA256"
fi

tar -xzf "$TMP_DIR/onnx.tgz" -C "$TMP_DIR"
EXTRACTED_DIR="$TMP_DIR/onnxruntime-${OS}-${ARCH}-${ONNX_VERSION}"

cp "$EXTRACTED_DIR/lib/"libonnxruntime* "$LIB_DIR/"
verify_sha256 "$LIB_DIR/$LIB_NAME" "$LIB_SHA256"

echo "✅ ONNX Runtime libraries installed to $LIB_DIR"
ls -lh "$LIB_DIR"
