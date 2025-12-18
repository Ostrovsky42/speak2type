Speak2Type Implementation Task List
Project Goal
Port Python-based Speak2Type voice input tool to Go with mandatory Russian language support, production-ready architecture, and real-time performance.

Phase 1: Audio Foundation ✅ COMPLETE
Core Components

internal/audio/ringbuffer.go
— Circular buffer implementation
Zero-allocation Write() method
Thread-safe concurrent access (mutex-protected)
Atomic position tracking
Overflow handling
Statistics collection

internal/audio/ringbuffer_test.go
— Comprehensive test suite
Basic write/read test
Wraparound behavior test
Concurrent read/write test (race detector clean)
Zero-allocation verification test
Snapshot duration test
Empty buffer edge case test
Reset functionality test

internal/audio/service.go
— Audio capture service
malgo (miniaudio) integration
Hardware resampling to 16kHz
Zero-copy callback (unsafe.Slice)
Device enumeration
Performance metrics


pkg/config/config.go
— Configuration management
JSON persistence


Platform-specific paths
Validation
Russian language support in config

cmd/mic-test/main.go
— Demo application
Device listing
Audio capture verification
Real-time statistics display
Signal handling (Ctrl+C)
Documentation

README.md
— Project overview and quick start

docs/phase1_notes.md
— Implementation notes

walkthrough.md
— Phase 1 walkthrough
Verification
All tests passing (7/7)
Race detector clean
Binary builds successfully (3.9 MB)
Demo runs without errors
0% dropped frames verified
Status: ✅ COMPLETE (2025-11-29)

Phase 2: Voice Activity Detection ✅ COMPLETE
Core Components

internal/vad/service.go
— VAD service implementation
ONNX Runtime Go bindings setup
Embed silero_vad.onnx model (~5 MB)
LSTM state management
Chunk processing (512/1024/1536 samples)
Probability output (0.0-1.0)

internal/vad/gate.go
— Hysteresis gate logic
State machine (Silence ↔ Speech)
Configurable thresholds (start: 0.5, end: 0.3)
Minimum duration enforcement (300ms speech, 500ms silence)
Event emission (speech_start, speech_end)

internal/vad/service_test.go
— VAD tests
Accuracy test on clean speech
Keyboard noise rejection test
Hysteresis behavior test
Latency benchmark (<30ms target)
Integration

cmd/vad-test/main.go
— VAD demo
AudioService → VADService pipeline
Visual speech/silence indicator
Real-time probability display
Test on recorded samples
Dependencies
go get github.com/yalue/onnxruntime_go
Download silero_vad.onnx from Silero repo
Status: ✅ COMPLETE (2025-11-29)

Phase 3: Speech Recognition ✅ COMPLETE
Core Components

internal/asr/service.go
— Whisper.cpp integration
Architecture: Single-worker goroutine pattern
Data Types:
AudioWindow
(input),
TranscriptionChunk
(output)
Queue: Buffered channel (size 3) with drop-oldest strategy
CGO: Safe integration with official bindings (manual build)
Config: language_mode (auto/ru/en) support

internal/asr/model.go
— Model management
Model loader (ggml-base.bin)
Validation (existence, size)

internal/asr/service_test.go
— ASR tests
Integration test (Silence/English)
Queue overflow test
CGO compilation verification
Russian Language Tests

cmd/asr-test/main.go
— ASR demo
AudioService → VAD → ASR pipeline
7s sliding window logic
Real-time transcription output
Status: ✅ COMPLETE (2025-11-29)

Phase 4: Text Stabilization 🚧 NEXT
Core Components
internal/merger/service.go — Text merger service
Stability-based algorithm
LCS (Longest Common Subsequence) implementation
Committed vs Tentative buffers
Stability scoring per word
Anti-hallucination protection
internal/merger/lcs.go — LCS algorithm
Dynamic programming implementation
Overlap detection
Token-based matching
internal/merger/service_test.go — Merger tests
No word duplication test
Idempotency test
Russian phrase merging test
English phrase merging test
Property-based tests
Test Cases
Overlapping segments without gaps
Overlapping segments with model corrections
No overlap (fallback behavior)
Long continuous speech (60+ seconds)
Estimated Effort: 3-4 days

Phase 5: Session Management 📋 PLANNED
Core Components
internal/session/orchestrator.go — State machine
FSM implementation (Idle → Listening → Processing → Stabilizing)
Service coordination (Audio, VAD, ASR, Merger)
Mode support (Quick Note, Continuous Dictation)
Hotkey handling
Error recovery
internal/session/modes.go — Session modes
Quick Note implementation
Continuous Dictation implementation
Mode switching logic
internal/session/orchestrator_test.go — FSM tests
State transition tests
Error handling tests
Mode switching tests
Estimated Effort: 3-4 days

Phase 6: Input Injection & UI 📋 PLANNED
Input Injection
internal/injection/service.go — RobotGo integration
Main OS thread affinity (runtime.LockOSThread)
Command channel pattern
TypeText mode
PasteClipboard mode
Platform-specific paste (Ctrl+V / Cmd+V)
internal/injection/service_test.go — Injection tests
Basic text injection test
Clipboard preservation test
Platform detection test
User Interface
System tray integration
Icon display
Context menu
Status updates
Floating overlay (optional)
Fyne or minimal custom UI
Tentative text display
Position configuration
Settings UI (optional)
Config editing
Device selection
Hotkey customization
Estimated Effort: 4-5 days

Phase 7: Production Ready 📋 PLANNED
Optimization
CPU profiling (pprof)
Identify hot paths
Optimize allocations
Reduce GC pressure
Memory profiling
Detect leaks
Optimize buffer sizes
Verify target <500 MB
Cross-Platform Builds
Linux (amd64, arm64)
Static linking where possible
.tar.gz distribution
Desktop integration (.desktop file)
macOS (amd64, arm64)
Code signing
.app bundle
Permission handling automation
Windows (amd64)
DLL bundling
.exe installer
Registry integration
Model Distribution
Download on first run
Progress indicator
Checksum verification
Retry logic
Model cache management
Version checking
Auto-update option
Documentation
User guide
Installation instructions
Quick start tutorial
Troubleshooting guide
Developer documentation
Architecture overview
Contributing guide
API documentation
Final Verification
E2E test (full pipeline)
Performance benchmarks
Memory leak tests (24h+ run)
Cross-platform smoke tests
Estimated Effort: 5-7 days

Overall Progress
Phases
Phase 1: Audio Foundation (COMPLETE)
Phase 2: VAD (COMPLETE)
Phase 3: ASR (COMPLETE)
Phase 4: Text Merger (COMPLETE)
Phase 5: Session Management (COMPLETE)
- [x] Phase 7: Input Injection (RobotGo) @completed(2025-12-18)
- [x] Phase 7.5: VAD & Audio Debugging @completed(2025-12-18)
- [x] Phase 8: Production Readiness & Packaging @completed(2025-12-18)
6/8 phases (75%)

Key Milestones
Project structure created
Audio capture working
Zero-allocation pipeline verified
Configuration system implemented
Demo application built
VAD integration
Russian ASR verified (via demo)
Text merging stable
Hotkey functional
Production builds
1.0 Release
Critical Requirements (All Phases)
Russian Language Support
Config: primary_language: "ru" ✅
Model: Multilingual Whisper loaded
Tests: Russian phrase accuracy >88%
E2E: Mixed RU/EN conversation handling
Performance Targets
Audio callback: <1µs latency ✅
Dropped frames: <0.1% ✅
Memory (idle): <50 MB ✅
Memory (active): <500 MB
CPU (transcribing): <80% one core
Latency (first word): <250ms
Latency (updates): <3.5s
Quality Targets
Test coverage: >80% ✅ (87% Phase 1)
Race detector: clean ✅
Valgrind: no leaks
WER (Russian): <12%
WER (English): <8%
Notes
Phase 1 Complete: 2025-11-29
Phase 2/3 Complete: 2025-12-17
Phase 4/5 Complete: 2025-12-17
Phase 6.5 (Refactor) Complete: 2025-12-17
Next Review: After Phase 7 Input & UI implementation
Target 1.0 Release: TBD (after Phase 7)

Russian Language: Mandatory, first-class support enforced at all phases.

Architecture Principle: CSP (Communicating Sequential Processes) — services communicate via channels, not shared memory.