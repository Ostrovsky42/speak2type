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
export CGO_LDFLAGS := -L$(LIB_DIR) -lwhisper -lggml -lggml-base -lggml-cpu -lonnxruntime
export LD_LIBRARY_PATH := $(LIB_DIR):$(LD_LIBRARY_PATH)

# Auto-detect nohook if X11 headers are missing on Linux
HAS_X11 := $(shell test -f /usr/include/X11/Xlib-xcb.h && echo 1 || echo 0)
BUILD_TAGS ?= $(if $(filter 0,$(HAS_X11)),nohook,)

all: build

help:
	@echo "Speak2Type Build System"
	@echo "  make deps     - Download models and libraries"
	@echo "  make build    - Compile binaries (fails if libs missing)"
	@echo "  make dist     - Create portable dist/ folder"
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
	@./bin/speak2type doctor

build: check-env
	@echo "🔨 Building Speak2Type with tags: $(BUILD_TAGS)..."
	@mkdir -p bin
	go build -tags "$(BUILD_TAGS)" -ldflags "-r $(LIB_DIR)" -o bin/speak2type cmd/speak2type/main.go
	@echo "✨ Build complete. Binary: ./bin/speak2type"

dist: check-env
	@echo "📦 Packaging for distribution..."
	@rm -rf dist && mkdir -p dist/lib dist/models
	# 1. Build with $ORIGIN/lib rpath
	go build -tags "$(BUILD_TAGS)" -ldflags "-r \$$ORIGIN/lib" -o dist/speak2type cmd/speak2type/main.go
	# 2. Copy Libs
	cp $(LIB_DIR)/*.so* dist/lib/
	# 3. Copy Models
	cp $(MODELS_DIR)/* dist/models/
	# 4. Copy README subset or license
	@echo "Speak2Type Portable" > dist/README.txt
	@echo "Run ./speak2type run" >> dist/README.txt
	@echo "✨ Distribution ready in ./dist"

test: check-env
	@echo "🧪 Running tests..."
	go test -v ./internal/...

clean:
	@echo "🧹 Cleaning up..."
	rm -rf bin/
