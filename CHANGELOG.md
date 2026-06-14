# Changelog

## Unreleased
- Switched default VAD model path to Silero VAD v5 with v4 fallback.
- Pinned native dependency download scripts and whisper.cpp source sync.
- Added MIT license and GitHub Actions Linux CI workflow.
- Repository hygiene: ignore generated binaries, downloaded models, local config, IDE files, and WAV fixtures.
- Linux MVP: event-driven UX loop with tray + notifications.
- systemd user service `enable`/`disable` commands.
- AppImage packaging script and portable bundle artifact.

## 0.9.0
- Initial public release.
