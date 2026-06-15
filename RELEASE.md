# Speak2Type: Professional Voice Input (Release 0.9.0)

Speak2Type is a production-ready voice-to-text utility for Linux and macOS, optimized for the Russian language and desktop text injection.

### 🚀 Quick Start
For detailed instructions on configuration, modes, and CLI, see: **[USAGE.md](USAGE.md)**

- **Tray build**: `./speak2type run` starts the daemon and tray UI.
- **Daemon only**: `./speak2type run --daemon` starts the background service without tray.
- **CLI info**: running `./speak2type` without arguments prints command help.

## 🏷️ Versioning

- Project version is stored in `VERSION`.
- Builds embed the version via `-ldflags "-X github.com/Ostrovsky42/speak2type/internal/version.Version=..."`.
- `speak2type version` prints the embedded version.
- Changes are tracked in `CHANGELOG.md`.

## 📦 Release Artifacts

### Linux
- `Speak2Type-x86_64.AppImage` (recommended desktop artifact, tray-enabled)
- `speak2type-<version>-linux-<arch>.tar.gz` (portable bundle: binary + libs + models)

### macOS
- `speak2type-<version>-darwin-amd64.tar.gz`
- `speak2type-<version>-darwin-arm64.tar.gz`

macOS archives are portable CLI bundles with bundled native libraries and models. They require Microphone and Accessibility permissions at runtime. Tray/AppImage packaging is Linux-only.

## 🔁 Updating

- **Linux AppImage**: download the new AppImage and replace the old one.
- **Linux tarball**: unpack over the previous directory and restart the service:
  ```bash
  systemctl --user restart speak2type.service
  ```
- **macOS tarball**: unpack the archive, replace the previous binary directory, then restart any running `speak2type` process.

## 📜 Verification Contract
The system is considered **production-ready** and healthy if and only if:

1.  **Diagnostic**: `./speak2type doctor` returns exit code `0`.
2.  **Recognition**: `./speak2type run --dry-run` produces coherent Russian text in the console.
3.  **Integration**: `./speak2type inject-test` inserts text into the captured target window.
4.  **Background**: `./speak2type run --daemon` detaches successfully, and `./speak2type status` shows `RUNNING`.
5.  **Binary dependencies**: `ldd ./speak2type` on Linux or `otool -L ./speak2type` on macOS shows all shared libraries resolved.
6.  **Cloud ASR reload**: changing provider/API key from tray applies through `reload_config` without restarting the daemon.

## 🛠 Installation (The Blessed Path)

### Linux
To install Speak2Type as a persistent background service:

1.  Deploy the binary and libraries to a stable path (e.g., `~/bin/speak2type`).
2.  Enable autostart:
    ```bash
    ./speak2type enable
    ```

### macOS
Unpack the macOS tarball to a stable directory, grant Microphone and Accessibility permissions when prompted, then run:

```bash
./speak2type run
```

Systemd service installation is Linux-only.

## 🧾 License Attribution

Release artifacts that include native libraries or model files must also include `LICENSE` and `THIRD_PARTY_LICENSES.md`.

## 🧰 Build Artifacts (Maintainers)

```bash
make deps
make dist
make release
```

On Linux, `make release` builds the tray distribution, AppImage, and tarball; `appimagetool` must be installed for AppImages. On macOS, `make release` builds the portable `dist/` bundle and tarball with `.dylib` dependencies.

## ⚠️ Known Failure Modes & Limitations
*   **Clipboard Racing**: If you are manually copying/pasting while Speak2Type is injecting, the `restore-clipboard` feature may capture and restore inconsistent state.
*   **Focused Paste**: Speak2Type captures the active target window at recording start and tries to restore focus before paste. If the captured window is unavailable, focus guard blocks injection instead of pasting into the wrong app.
*   **Tray-start Targeting**: Starting recording from the tray menu may capture the tray/menu focus instead of the editor. The hotkey flow is recommended for reliable target-window capture.
*   **Wayland Support**: Text injection is disabled by default on Wayland for stability. Use `--force-wayland-inject` at your own risk.
*   **macOS Permissions**: Microphone and Accessibility permissions are required for capture and injection.
*   **GUI Session**: The daemon MUST be started within an active user GUI session to access the display and keyboard hooks.

## 📊 Support Matrix
| OS / Distro | Status | Method | Notes |
| :--- | :--- | :--- | :--- |
| Ubuntu 22.04+ | **Verified** | X11 / robotgo paste | Recommended Linux platform. |
| Debian 12 | **Verified** | X11 / robotgo paste | Requires standard dev libs for build. |
| Fedora (Wayland) | **Best Effort** | Experimental injection | Use `--force-wayland-inject` only if needed. |
| macOS Intel | **Supported** | CoreAudio + robotgo paste | Requires Microphone and Accessibility permissions. |
| macOS Apple Silicon | **Supported** | CoreAudio + robotgo paste | Requires Microphone and Accessibility permissions. |

---
*For technical support or issues, check logs at `~/.local/state/speak2type/speak2type.log`.*
