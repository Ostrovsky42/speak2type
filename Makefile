# Speak2Type Makefile (Strict Production)

.PHONY: all build clean deps test aid doctor check-env

# Paths
PROJECT_ROOT := $(shell pwd)
LIB_DIR := $(PROJECT_ROOT)/third_party/lib
WHISPER_DIR := $(PROJECT_ROOT)/third_party/whisper.cpp
MODELS_DIR := $(PROJECT_ROOT)/models

# Strict Environment Setup
# We define them here to ensure they are used, but we also check if they are valid.
export CGO_CFLAGS := -I$(WHISPER_DIR)/include -I$(WHISPER_DIR)/ggml/include
export CGO_LDFLAGS := -L$(WHISPER_DIR)/build/src -L$(WHISPER_DIR)/build/ggml/src -L$(LIB_DIR) -lwhisper -lonnxruntime
export LD_LIBRARY_PATH := $(WHISPER_DIR)/build/src:$(WHISPER_DIR)/build/ggml/src:$(LIB_DIR):$(LD_LIBRARY_PATH)

all: build

help:
	@echo "Speak2Type Build System"
	@echo "  make deps     - Download models and libraries"
	@echo "  make build    - Compile binaries (fails if libs missing)"
	@echo "  make doctor   - Run strict diagnostics"
	@echo "  make test     - Run tests"
	@echo "  make clean    - Remove artifacts"

deps:
	@echo "⬇️  Downloading dependencies..."
	./scripts/download_libs.sh
	./scripts/download_models.sh

check-env:
	@echo "🔍 Checking Build Environment..."
	@test -f $(LIB_DIR)/libonnxruntime.so || (echo "❌ libonnxruntime.so missing. Run 'make deps'"; exit 1)
	@test -d $(WHISPER_DIR)/include || (echo "❌ whisper.cpp headers missing. Run 'make deps' (ensure submodule init)"; exit 1)
	@echo "✅ Environment OK"

doctor:
	@go run cmd/doctor/main.go

build: check-env
	@echo "🔨 Building Speak2Type..."
	@mkdir -p bin
	go build -o bin/session-test cmd/session-test/main.go
	go build -o bin/doctor cmd/doctor/main.go
	go build -o bin/inject-test cmd/inject-test/main.go
	@echo "✨ Build complete. Binaries in ./bin"

test: check-env
	@echo "🧪 Running tests..."
	go test -v ./internal/...

clean:
	@echo "🧹 Cleaning up..."
	rm -rf bin/
