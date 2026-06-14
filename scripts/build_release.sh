#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="$ROOT_DIR/dist"

if [ ! -f "$DIST_DIR/speak2type" ]; then
  echo "dist/speak2type not found. Run 'make dist' or 'make dist-tray' first."
  exit 1
fi

cp "$ROOT_DIR/LICENSE" "$DIST_DIR/LICENSE"
cp "$ROOT_DIR/THIRD_PARTY_LICENSES.md" "$DIST_DIR/THIRD_PARTY_LICENSES.md"

VERSION="$(cat "$ROOT_DIR/VERSION")"
ARCH="$(uname -m)"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
OUT="$DIST_DIR/speak2type-${VERSION}-${OS}-${ARCH}.tar.gz"

tar -C "$DIST_DIR" -czf "$OUT" speak2type lib models README.txt LICENSE THIRD_PARTY_LICENSES.md
echo "Release bundle created at $OUT"
