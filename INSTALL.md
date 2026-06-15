# Install

This guide covers building and running Speak2Type on Linux and macOS.

## Release Install

### Linux AppImage
```bash
chmod +x Speak2Type-x86_64.AppImage
./Speak2Type-x86_64.AppImage run
```

To enable autostart, place the AppImage in a stable path and run:
```bash
./Speak2Type-x86_64.AppImage enable
```
To disable autostart:
```bash
./Speak2Type-x86_64.AppImage disable
```

### macOS tarball
```bash
tar -xzf speak2type-<version>-darwin-<arch>.tar.gz
cd speak2type-<version>-darwin-<arch>
./speak2type run
```

Grant Microphone and Accessibility permissions when macOS prompts for them. Systemd autostart is Linux-only.

## Prerequisites

- **Linux**: X11 is recommended for reliable text injection. Wayland key simulation remains experimental and is disabled by default unless `--force-wayland-inject` is used.
- **macOS**: Microphone and Accessibility permissions are required for capture and text injection.

### System Dependencies (Ubuntu/Debian)
```bash
sudo apt-get update
sudo apt-get install -y build-essential cmake pkg-config libasound2-dev \
    libx11-dev libxtst-dev libpng-dev
```

## Build

```bash
# 1. Download checksum-pinned libraries and models
make deps

# 2. Verify environment
make doctor

# 3. Build binaries
make build
```

The binary is created at `./bin/speak2type`. Models are not stored in Git; `make deps` must complete before the default local-ASR `run` command can start. OpenAI/Groq ASR providers do not need the local Whisper GGML model, but they do require API keys in the environment or saved in config from the tray menu.

## Run

```bash
./bin/speak2type run -device-index 0 -lang ru
```

## Cloud ASR Providers

```bash
OPENAI_API_KEY=... ./bin/speak2type run --asr-provider openai --asr-cloud-model gpt-4o-mini-transcribe
GROQ_API_KEY=... ./bin/speak2type run --asr-provider groq --asr-cloud-model whisper-large-v3-turbo
```

Use `--asr-api-key-env CUSTOM_ENV` if your key is stored under a different environment variable. In tray builds, provider and API key changes under `ASR Provider` are reloaded by the daemon immediately.

## Install as a User Service (systemd)

```bash
speak2type enable
```

## Troubleshooting

- Run `./bin/speak2type doctor` to validate dependencies and environment.
- Logs: `~/.local/state/speak2type/speak2type.log`


## macOS Build Notes

The macOS build path is supported for development and CI. Use `nohook` when building in headless environments or without global hotkey support:

```bash
brew install cmake pkg-config
make deps
make build BUILD_TAGS=nohook
```

Runtime audio capture requires Microphone permission. Runtime input simulation requires Accessibility permission and is not part of the headless CI contract.
