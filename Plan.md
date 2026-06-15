# Speak2Type Plan and Roadmap

Last updated: 2026-06-15

## Status

The original implementation plan is complete, and the project has moved beyond the initial Python-to-Go port scope.

Current state:

- Core dictation pipeline is implemented: audio capture -> VAD -> ASR -> text stabilization -> focused paste injection.
- Linux and macOS are documented runtime targets.
- Local and cloud ASR providers are supported.
- Tray UX covers provider/key selection, hot reload, daemon/tray lifecycle, language/profile selection, logs, doctor, and restart.
- Release/install/user documentation has been refreshed for Linux and macOS.

## Completed Original Phases

### Phase 1: Audio Foundation

Completed.

Implemented:

- `internal/audio` capture service and ring buffer.
- Device enumeration and audio stats.
- Config persistence in `pkg/config`.
- `cmd/mic-test` verification tool.
- Documentation and phase notes.

### Phase 2: Voice Activity Detection

Completed.

Implemented:

- Silero VAD ONNX runtime path.
- VAD gate/hysteresis.
- VAD state preservation.
- Primary Silero VAD v5 model path with v4 fallback noted in architecture docs.
- VAD tests and diagnostics.

### Phase 3: Speech Recognition

Completed and extended.

Implemented:

- ASR service with worker queue and drop-oldest behavior.
- Local `whisper.cpp` provider.
- Provider interface in `internal/asr`.
- OpenAI and Groq cloud transcription providers.
- Provider-specific API keys:
  - `asr.openai_api_key`
  - `asr.groq_api_key`
  - legacy `asr.api_key` fallback retained.
- Runtime ASR provider hot reload through `reload_config` without daemon restart.

### Phase 4: Text Stabilization

Completed.

Implemented:

- LCS-based text merger.
- Stable vs tentative text handling.
- Final flush behavior.
- Console display of recognized text.

### Phase 5: Session Management

Completed and extended.

Implemented:

- Session orchestrator.
- Hotkey toggle flow.
- Processing state.
- ASR completion tracking.
- Full-session fallback audio submission when VAD misses speech on manual stop.
- Raw ASR logging and explicit empty-ASR logging.
- Processing indicator:
  - foreground: live `Processing... <elapsed> pending=N` line;
  - daemon: periodic processing log.

### Phase 6: Input Injection and UI

Completed and extended.

Implemented:

- Clipboard paste injection through platform input backend.
- Target-window capture at recording start.
- Focus restore before paste.
- Focus guard to avoid pasting into the wrong window.
- Clear `inject_unavailable` vs `focus_guard` error classification.
- Tray UI with status, language, profile, ASR provider/key settings, doctor/log helpers, restart, and unified `Stop Speak2Type` action.

### Phase 7: Production Readiness and Packaging

Completed and extended.

Implemented:

- `make deps`, `make build`, `make dist`, `make dist-tray`, `make appimage`, `make release` workflows.
- Linux AppImage/tarball release path.
- macOS portable tarball release documentation.
- Linux/macOS dependency download support for ONNX Runtime.
- Release, install, usage, and README documentation refresh.
- Third-party attribution docs.

## Completed Beyond Original Plan

### Hybrid ASR Providers

Delivered:

- `asr.Provider` interface.
- Local whisper.cpp provider.
- OpenAI API provider.
- Groq API provider.
- Provider-specific config/API key fields.
- Tray provider/key management.
- Hot reload without daemon restart.

### Tray-First UX

Delivered:

- In tray builds, `speak2type run` starts daemon + tray.
- Running `speak2type` without args prints CLI help/info.
- `speak2type run --daemon` remains daemon-only.
- Unified tray stop action stops daemon and tray.
- Restart action restarts daemon and tray.

### Focused Paste UX

Delivered:

- Capture target window when recording starts.
- Restore focus before paste.
- Block injection if the captured target cannot be restored.
- Documented tray-start limitation: tray/menu focus may be captured, so F8 from the target app is the recommended flow.

### macOS Documentation

Delivered:

- macOS build/release notes.
- macOS runtime permissions: Microphone + Accessibility.
- macOS config path.
- macOS release artifacts in `RELEASE.md`.
- `otool -L` verification contract.

## Verification Commands

Current verification baseline:

```bash
go test -tags no_whisper ./...
go test -tags "tray no_whisper" ./cmd/speak2type ./internal/cli ./internal/cli/tray
git diff --check -- README.md USAGE.md RELEASE.md INSTALL.md
```

Standard local build verification:

```bash
make deps
make build
make doctor
./bin/speak2type run
```

## Current Constraints and Known Limitations

- X11 remains the recommended Linux session for reliable hotkeys and paste injection.
- Wayland injection remains experimental and disabled by default unless `--force-wayland-inject` is used.
- Starting recording from tray may capture the tray/menu as the target window; F8 from the target app is the reliable flow.
- Cloud ASR requires API keys from environment or config.
- Local ASR performance depends heavily on model size and CPU.
- macOS runtime injection requires Accessibility permission.

## SDD Direction

The next process improvement is to move from an implementation task list to spec-driven development.

Recommended approach:

- Adopt OpenSpec as the primary SDD workflow for this brownfield codebase.
- Borrow Kiro's three-artifact structure inside each change:
  - `requirements.md`
  - `design.md`
  - `tasks.md`
- Write requirements with EARS-style statements.
- Keep specs in-repo and review spec deltas before code changes.
- Use GitHub Spec Kit only if a heavier `constitution -> specify -> plan -> tasks -> implement` flow is desired.

Important note: some external research notes reference dates after the current project date, 2026-06-15. Treat those claims as research backlog items and verify current upstream status before implementation.

### Proposed OpenSpec Layout

```text
openspec/
  project.md
  specs/
    core-dictation/
      requirements.md
      design.md
      tasks.md
    input-injection/
      requirements.md
      design.md
      tasks.md
    asr-providers/
      requirements.md
      design.md
      tasks.md
  changes/
    <change-id>/
      proposal.md
      requirements.md
      design.md
      tasks.md
```

### EARS Requirement Template

Examples for future specs:

```text
REQ-001: The system shall capture microphone audio at 16 kHz mono.
REQ-010: When the user presses F8, the system shall toggle recording state.
REQ-020: While transcription is processing, the system shall display processing progress.
REQ-030: If injection is unavailable, then the system shall surface an actionable error and avoid pasting text into the wrong target.
REQ-040: Where cloud ASR is selected, the system shall load the provider-specific API key without requiring daemon restart.
```

## Track 1 Backlog: Existing Dictation Pipeline

Near-term candidates:

1. Initialize OpenSpec and write `project.md` constraints.
2. Reverse-spec the current dictation pipeline before more code changes.
3. Add specs for ASR provider hot reload and focused paste.
4. Evaluate Silero VAD v6/v6.2 ONNX compatibility against `internal/vad` before changing defaults.
5. Profile local whisper.cpp large-v3-turbo latency and identify CGO callback overhead.
6. Improve Wayland injection strategy:
   - evaluate ydotool as an explicit optional backend;
   - evaluate XDG RemoteDesktop/libei where available;
   - update `doctor` to detect and report actionable backend setup.
7. Improve tray-start targeting UX, for example a delayed "record next focused window" mode.
8. Add macOS smoke verification docs/scripts for permissions and injection.

## Track 2 Backlog: Multi-Speaker Diarization to Sub-Agent Pipelines

This is a new feature track, not part of the completed dictation MVP.

Recommended first architecture:

```text
AudioService -> VAD -> diarization boundary -> ASR per utterance
             -> speaker identity -> filter chain -> MCP sub-agent router
```

Build vs reuse:

- Reuse existing AudioService, VAD, ASR service, event bus, and config patterns.
- Reuse a diarization engine; do not train a diarizer.
- Build the orchestration boundary, utterance event model, routing/filter chain, and MCP dispatch.

Candidate implementation path:

1. Prototype in-process offline/chunked diarization with sherpa-onnx Go bindings.
2. Add an utterance event model:
   - speaker cluster ID;
   - optional enrolled speaker identity;
   - timestamps;
   - transcript;
   - confidence/route metadata.
3. Add speaker enrollment/identification using embeddings and cosine similarity.
4. Add a Go filter chain:
   - validate;
   - normalize;
   - classify;
   - route.
5. Add MCP sub-agent boundary using the official Go MCP SDK.
6. Keep Python sidecars as optional later paths for pyannote/NeMo if accuracy or streaming needs justify them.

Complexity warning:

- Offline/chunked diarization is realistic for a Go-first prototype.
- True low-latency streaming diarization with stable labels is still complex.
- Stable speaker identity should rely on enrollment embeddings rather than ephemeral diarization cluster IDs.

## Track 2 Decision Gates

Use these thresholds to choose architecture:

- If sub-second speaker labels are required, evaluate GPU-backed streaming diarization before committing to a Go-only architecture.
- If offline or 1-2s delayed labels are acceptable, start with sherpa-onnx in-process.
- If accuracy beats deployment simplicity, consider a Python sidecar behind gRPC or MCP.
- If routed downstream work can be long-running, model sub-agents as MCP servers and use task handles once the target MCP spec/API is stable enough.

## Definition of Done Going Forward

A new feature is done when:

- The behavior is captured in a spec or change-level proposal.
- Requirements include acceptance criteria.
- The implementation is covered by focused tests or a documented manual verification flow.
- `go test -tags no_whisper ./...` passes.
- Tray-specific changes also pass `go test -tags "tray no_whisper" ./cmd/speak2type ./internal/cli ./internal/cli/tray`.
- User-facing behavior is reflected in `README.md`, `USAGE.md`, `INSTALL.md`, or `RELEASE.md` when applicable.

## Historical Notes

The original milestones were completed and superseded:

- Phase 1: Audio Foundation.
- Phase 2: VAD.
- Phase 3: ASR.
- Phase 4: Text Merger.
- Phase 5: Session Management.
- Phase 6: Input Injection and UI.
- Phase 7: Production readiness and packaging.