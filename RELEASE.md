# Speak2Type: Professional Voice Input (Release 0.9.x)

Speak2Type is a production-ready voice-to-text utility for Linux, optimized for the Russian language and seamless desktop integration.

### 🚀 Quick Start
For detailed instructions on configuration, modes, and CLI, see: **[USAGE.md](USAGE.md)**

1.  **Run Brain**: `./speak2type run --daemon`
2.  **Run UI**: `./speak2type tray` (requires `libayatana-appindicator3-dev`)

## 📜 Verification Contract
The system is considered **production-ready** and healthy if and only if:

1.  **Diagnostic**: `./speak2type doctor` returns exit code `0`.
2.  **Recognition**: `./speak2type run --dry-run` produces coherent Russian text in the console.
3.  **Integration**: `./speak2type inject-test` successfully inserts text into an active X11 window.
4.  **Background**: `./speak2type run --daemon` detaches successfully, and `./speak2type status` shows `RUNNING` with valid X11 environment variables.
5.  **Binary**: `ldd ./speak2type` shows all shared library dependencies resolved.

## 🛠 Installation (The Blessed Path)
To install Speak2Type as a persistent background service:

1.  Deploy the binary and libraries to a stable path (e.g., `~/bin/speak2type`).
2.  Configure the `scripts/speak2type-user.service` template with the absolute path to your binary.
3.  Install and start:
    ```bash
    mkdir -p ~/.config/systemd/user/
    cp scripts/speak2type-user.service ~/.config/systemd/user/speak2type.service
    systemctl --user daemon-reload
    systemctl --user enable --now speak2type.service
    ```

## ⚠️ Known Failure Modes & Limitations
*   **Clipboard Racing**: If you are manually copying/pasting while Speak2Type is injecting, the `restore-clipboard` feature may capture and restore inconsistent state.
*   **Locked Focus**: The **Focus Guard** will block injection if you switch windows *after* starting a session. Always switch to your target application **before** pressing F8.
*   **Wayland Support**: Text injection (simulated keys) is disabled by default on Wayland for stability. Use `--force-wayland-inject` at your own risk.
*   **GUI Session**: The daemon MUST be started within an active X11/Wayland user session to access the display and keyboard hooks.

## 📊 Support Matrix
| OS / Distro | Status | Method | Notes |
| :--- | :--- | :--- | :--- |
| Ubuntu 22.04+ | **Verified** | X11 / Clipboard | Recommended platform. |
| Debian 12 | **Verified** | X11 / Clipboard | Requires standard dev libs for build. |
| Fedora (Wayland) | **Best Effort** | Clipboard Only | Injection via `--force-wayland-inject`. |

---
*For technical support or issues, check logs at `~/.local/state/speak2type/speak2type.log`.*
