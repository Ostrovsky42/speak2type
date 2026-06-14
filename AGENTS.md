# Repository Guidelines

## Project Structure and Module Organization
- `cmd/`: CLI entry points (e.g., `cmd/speak2type`).
- `internal/`: Core services (audio, VAD, ASR, session orchestration, tray UI, IPC).
- `pkg/`: Public config types and helpers.
- `scripts/`: Build, packaging, and service templates.
- `models/`, `third_party/`: model files and native deps.
- `docs/`, `README.md`, `USAGE.md`, `RELEASE.md`: documentation.
- Tests live alongside packages (e.g., `internal/e2e`, `internal/event`).

## Build, Test, and Development Commands
- `make deps`: download models and native libraries.
- `make doctor`: verify system dependencies and environment.
- `make build`: build `./bin/speak2type`.
- `make dist`: produce portable `dist/` bundle.
- `make dist-tray`: build `dist/` with `tray` build tag.
- `make appimage`: package AppImage (requires `appimagetool`).
- `make release`: build AppImage + tar.gz bundle.
- `./bin/speak2type run --daemon`: run in background.
- `./bin/speak2type tray`: tray UI (build with `-tags tray`).

## Coding Style and Naming Conventions
- Go code is formatted with `gofmt` (tabs, standard Go style).
- Packages use lower-case names; exported types use CamelCase.
- Prefer clear, specific error messages and avoid silent failures.
- Build tags: `tray` enables systray UI; `nohook` disables hotkeys.

## Testing Guidelines
- Tests use Go’s standard `testing` package.
- Run `make test` (currently `go test -v ./internal/...`).
- Unit tests are preferred for new utilities (e.g., event bus).

## Commit and Pull Request Guidelines
- Recent commits follow a lightweight convention: `type: summary` (e.g., `feat: ...`, `refactor: ...`).
- PRs should include: a short summary, testing notes, and a linked issue if applicable.
- Include screenshots only when changing UI/tray behavior.

## Configuration and Runtime Notes
- Config file: `~/.config/speak2type/config.json`.
- Logs: `~/.local/state/speak2type/speak2type.log`.
- X11 is required for reliable injection; Wayland is experimental.
