# Speak2Type User Guide 📖

Welcome to **Speak2Type**, a professional voice input tool for Linux, optimized for speed, precision, and the Russian language.

## 🚀 Getting Started

Speak2Type operates on a **Client-Server model**:
1.  **The Daemon** (`speak2type run --daemon`): The "Brain". Runs in the background, handles audio, VAD, and ASR.
2.  **The Clients** (`speak2type tray` or `speak2type status`): The "Controls". Communicate with the daemon via Unix Sockets.

### Quick Start
```bash
# 1. Start the brain
speak2type run --daemon

# 2. Start the UI
speak2type tray
```

---

## 🎭 Operation Profiles

Speak2Type features specialized behavior profiles to match your workflow.

### 🖋️ Dictation (Default)
Optimized for writing long texts, emails, or articles.
- **VAD**: Conservative (waits longer for you to finish).
- **Silence Timeout**: 3.0 seconds.
- **Text Stability**: High (waits for more context before "typing" words).
- **Use case**: Writing this documentation.

### ⚡ Commands
Optimized for fast system control and short snippets.
- **VAD**: Aggressive (reacts instantly to pauses).
- **Silence Timeout**: 1.5 seconds.
- **Text Stability**: Fast (types words as soon as they are likely correct).
- **Use case**: Terminal commands, IDE navigation, chat replies.

---

## 🛠️ CLI Reference

| Command | Description |
| :--- | :--- |
| `speak2type run` | Start in foreground (interactive). |
| `speak2type run --daemon` | Start as background service. |
| `speak2type stop` | Terminate the running daemon safely. |
| `speak2type status` | Check if daemon is alive & current state. |
| `speak2type tray` | Launch the Systray UI (requires Gtk/AppIndicator). |
| `speak2type doctor` | Comprehensive system health check. |
| `speak2type version` | Show version info. |
| `speak2type enable` | Install and enable systemd user service. |
| `speak2type disable` | Disable systemd user service. |

`speak2type --version` is a shortcut for the version output.

Log level (run flag):
```bash
speak2type run --log-level warn
```

Local ASR model path (for example, a local large-v3-turbo GGML model):
```bash
speak2type run --asr-provider local --asr-model models/ggml-large-v3-turbo-q5_0.bin
```

Cloud ASR providers:
```bash
OPENAI_API_KEY=... speak2type run --asr-provider openai --asr-cloud-model gpt-4o-mini-transcribe
GROQ_API_KEY=... speak2type run --asr-provider groq --asr-cloud-model whisper-large-v3-turbo
```

Useful cloud flags:
```bash
speak2type run --asr-provider groq --asr-timeout 10s --asr-prompt "Russian technical dictation"
```

---

## ⚙️ Configuration & Customization

### Config File
Speak2Type uses a JSON config file:

- `~/.config/speak2type/config.json` (Linux)

Example:
```json
{
  "session": {
    "hotkey": "f8"
  },
  "asr": {
    "provider": "local",
    "model_path": "models/ggml-base.bin",
    "language_mode": "auto",
    "model": "",
    "api_key_env": "",
    "timeout_seconds": 30
  },
  "notifications": {
    "errors": true,
    "done": false,
    "recording": false
  },
  "logging": {
    "level": "info"
  }
}
```

### Environment Variables
Speak2Type respects XDG standards. You can also override behaviors via env vars:

- `XDG_RUNTIME_DIR`: Location of the IPC socket (`/speak2type/speak2type.sock`).
- `XDG_STATE_HOME`: Location of logs (`/speak2type/speak2type.log`).
- `DISPLAY` / `XAUTHORITY`: Required for global hotkeys and text injection.
- `OPENAI_API_KEY`: Used by `asr.provider=openai` unless `asr.api_key_env` or `--asr-api-key-env` points elsewhere.
- `GROQ_API_KEY`: Used by `asr.provider=groq` unless overridden.

### Service Integration
To run Speak2Type automatically on login, use:
```bash
speak2type enable
```
This creates a systemd user unit at `~/.config/systemd/user/speak2type.service`.
To disable autostart:
```bash
speak2type disable
```

---

## 🩺 Troubleshooting

If Speak2Type isn't responding:
1.  **Run Doctor**: `speak2type doctor` will point out missing libraries or audio issues.
2.  **Check Logs**: `tail -f ~/.local/state/speak2type/speak2type.log`.
3.  **Restart**: `speak2type stop && speak2type run --daemon`.

> [!TIP]
> **Wayland Support**: In Wayland sessions, global hotkeys might be limited. We recommend X11 for the best experience with global text injection.

---
*Speak2Type — Speak, don't type.* 🎤✨
