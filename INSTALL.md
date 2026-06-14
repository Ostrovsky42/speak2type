# Install

This guide covers building and running Speak2Type on Linux and macOS.

## Release Install (AppImage)

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

## Prerequisites

- **Linux**: X11 session for reliable text injection. Wayland is experimental.
- **macOS**: Accessibility permissions for input simulation.

### System Dependencies (Ubuntu/Debian)
```bash
sudo apt-get update
sudo apt-get install -y build-essential cmake pkg-config libasound2-dev portaudio19-dev \
    libx11-dev libxtst-dev libpng-dev xclip
```

## Build

```bash
# 1. Download libraries and models
make deps

# 2. Verify environment
make doctor

# 3. Build binaries
make build
```

The binary is created at `./bin/speak2type`.

## Run

```bash
./bin/speak2type run -device-index 0 -lang ru
```

## Install as a User Service (systemd)

```bash
speak2type enable
```

## Troubleshooting

- Run `./bin/speak2type doctor` to validate dependencies and environment.
- Logs: `~/.local/state/speak2type/speak2type.log`
