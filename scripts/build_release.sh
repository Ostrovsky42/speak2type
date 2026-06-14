#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="$ROOT_DIR/dist"

if [ ! -f "$DIST_DIR/speak2type" ]; then
  echo "dist/speak2type not found. Run 'make dist' or 'make dist-tray' first."
  exit 1
fi

VERSION="$(cat "$ROOT_DIR/VERSION")"
ARCH="$(uname -m)"
OUT="$DIST_DIR/speak2type-${VERSION}-linux-${ARCH}.tar.gz"

tar -C "$DIST_DIR" -czf "$OUT" speak2type lib models README.txt
echo "Release bundle created at $OUT"
