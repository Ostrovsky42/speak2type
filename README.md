# Speak2Type

Modular, highly concurrent pipeline for voice input (VAD -> ASR -> Text Stabilization -> Processing) in Go.
Designed for Linux (X11-first) and macOS with platform-specific input backends.

## Features

- **Robust VAD**: Silero VAD v5 (ONNX) with v4 fallback and state preservation.
- **Hybrid ASR**: Local whisper.cpp by default, with optional OpenAI and Groq cloud transcription providers.
- **Text Stability**: LCS-based merging for flicker-free streaming.
- **Reliable Injection**: Clipboard-based text insertion with target-window capture and focus restore.
- **Diagnostics**: Built-in `doctor` command for environment verification.

## Docs

- **Install**: `INSTALL.md`
- **Usage**: `USAGE.md`
- **Release**: `RELEASE.md`
- **Contributing**: `CONTRIBUTING.md`
- **Changelog**: `CHANGELOG.md`
- **Architecture notes**: `docs/architecture.md`
- **License**: `LICENSE`
- **Third-party licenses**: `THIRD_PARTY_LICENSES.md`


## Models

Model files and native libraries are intentionally not committed. After cloning, run:

```bash
make deps
```

This downloads checksum-pinned Silero VAD, Whisper GGML, ONNX Runtime, and whisper.cpp artifacts for local ASR. `speak2type run` expects the default VAD model at `models/silero_vad.onnx`; if it is missing, run `make deps`. Cloud ASR providers still use local VAD/audio capture, but do not need the local Whisper GGML model at runtime. Linux and macOS dependency downloads are supported; macOS currently builds the nohook path in CI.

## ⚡ Quick Start

### 1. Prerequisites

- **Linux**: X11 is recommended for reliable text injection. Wayland key simulation remains experimental and is disabled by default unless `--force-wayland-inject` is used.
- **macOS**: Microphone and Accessibility permissions are required for capture and text injection.

**Install System Dependencies (Ubuntu/Debian):**
```bash
sudo apt-get update
sudo apt-get install -y build-essential cmake pkg-config libasound2-dev \
    libx11-dev libxtst-dev libpng-dev
```

### 2. Setup & Build

We use `make` for a unified workflow.

```bash
# 1. Download pinned libraries (ONNX Runtime, Whisper.cpp) and models (Silero V5/V4, GGML Base)
make deps

# 2. Check your environment (Critical!)
make doctor
# Output should show "System check PASSED"

# 3. Build binaries
make build
```

Binaries will be placed in `./bin/`.

### 3. Usage

**✨ Blessed Path (Local ASR):**
```bash
./bin/speak2type run -device-index 0 -lang ru
```

In tray builds, `run` starts the background daemon and opens the tray UI. Running the binary without arguments prints CLI usage/info.

**Cloud ASR Providers:**
```bash
OPENAI_API_KEY=... ./bin/speak2type run --asr-provider openai --asr-cloud-model gpt-4o-mini-transcribe
GROQ_API_KEY=... ./bin/speak2type run --asr-provider groq --asr-cloud-model whisper-large-v3-turbo
```
The tray menu can also save provider settings plus separate OpenAI/Groq API keys under `ASR Provider`; changes are reloaded by the daemon immediately.

**Verify System:**
```bash
./bin/speak2type doctor
```

**Verify Text Injection (Sniper Mode):**
```bash
./bin/speak2type inject-test -text "Привет!" -delay-ms 3000
# Focus a text field. The tool will paste text and restore your clipboard.
```

**Enable Autostart (systemd user service):**
```bash
./bin/speak2type enable
```

### ⌨️ Global Hotkeys
- **F8**: Toggle Recording (Start/Stop)
- **Enter** (in terminal): Toggle Recording

### ⚠️ Limitations & Risks

1. **Wayland**: Text injection is **experimental** on Wayland and disabled by default. Use X11 for Linux reliability.
2. **macOS Permissions**: Grant Microphone and Accessibility permissions before using capture or injection.
3. **Clipboard Injection**:
   - We use paste shortcut simulation (`Ctrl+V` / `Cmd+V`). Apps that block paste will not work.
   - **Race Conditions**: If you copy something exactly when Speak2Type pastes, clipboard restore may capture inconsistent state.
   - **Focus Guard**: The app captures the active target window at recording start and restores focus before paste. If the target cannot be restored, injection is blocked.
   - **Privacy**: The clipboard is briefly used. We attempt to restore it, but crashes might leave the clipboard with the injected text.

**Debug VAD (Silence Issue?):**
```bash
./scripts/run_vad.sh -device-index 0 -debug-rms
# VAD v4 is the default and should trigger on speech.
```

---

## 🛠️ Troubleshooting

### Input Injection Fails / Types Gibberish
- **Cause**: Direct key-code typing is unreliable across layouts and OSes.
- **Fix**: Speak2Type uses clipboard paste by default through the platform input backend.
- **Focus Guard**: Start recording from the target application with F8. If recording is started from tray, the tray/menu may be captured instead of the editor.
- **Wayland**: If `make doctor` shows "Session Type: wayland", injection is experimental. Switch to X11 for reliability.
- **macOS**: Confirm Accessibility permission in System Settings if paste does not happen.

### Cloud ASR Fails
- **OpenAI**: set `OPENAI_API_KEY` or override with `--asr-api-key-env`.
- **Groq**: set `GROQ_API_KEY` or override with `--asr-api-key-env`.
- Use `--asr-timeout 10s` to tune network timeout for short command dictation.

### `libonnxruntime.so` not found
- Run `make doctor`. It will tell you exactly where it expects the library.
- Ensure you ran `make deps`.

### `VAD always SILENCE`
- We default to Silero VAD v5 (`models/silero_vad.onnx`) and keep v4 (`models/silero_vad_v4.onnx`) as a fallback.
- Provide more gain: `./scripts/run_vad.sh -gain 5.0`

---

## 🏗️ Development

**Run Tests:**
```bash
make test
```

**Manual Build:**
If you need to bypass make:
```bash
source scripts/setup_env.sh
go build ./cmd/...
```
