#!/usr/bin/env bash
set -euo pipefail

APPIMAGE_TOOL="${APPIMAGE_TOOL:-appimagetool}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="$ROOT_DIR/dist"
APPDIR="$DIST_DIR/AppDir"

if ! command -v "$APPIMAGE_TOOL" >/dev/null 2>&1; then
  echo "appimagetool not found. Install it or set APPIMAGE_TOOL=/path/to/appimagetool."
  exit 1
fi

if [ ! -f "$DIST_DIR/speak2type" ]; then
  echo "dist/speak2type not found. Run 'make dist' or 'make dist-tray' first."
  exit 1
fi

rm -rf "$APPDIR"
mkdir -p "$APPDIR/usr/bin" \
  "$APPDIR/usr/lib" \
  "$APPDIR/usr/share/speak2type/models" \
  "$APPDIR/usr/share/applications" \
  "$APPDIR/usr/share/icons/hicolor/256x256/apps"

cp "$DIST_DIR/speak2type" "$APPDIR/usr/bin/"
cp "$DIST_DIR/lib/"*.so* "$APPDIR/usr/lib/"
cp -r "$DIST_DIR/models/"* "$APPDIR/usr/share/speak2type/models/"
cp "$ROOT_DIR/scripts/appimage/AppRun" "$APPDIR/AppRun"
cp "$ROOT_DIR/scripts/appimage/speak2type.desktop" "$APPDIR/usr/share/applications/"

if [ -f "$ROOT_DIR/scripts/appimage/speak2type.png" ]; then
  cp "$ROOT_DIR/scripts/appimage/speak2type.png" "$APPDIR/usr/share/icons/hicolor/256x256/apps/"
fi

ln -s usr/share/applications/speak2type.desktop "$APPDIR/speak2type.desktop"
if [ -f "$APPDIR/usr/share/icons/hicolor/256x256/apps/speak2type.png" ]; then
  ln -s usr/share/icons/hicolor/256x256/apps/speak2type.png "$APPDIR/speak2type.png"
fi

chmod +x "$APPDIR/AppRun"

OUT="$DIST_DIR/Speak2Type-x86_64.AppImage"
"$APPIMAGE_TOOL" "$APPDIR" "$OUT"
echo "AppImage created at $OUT"
