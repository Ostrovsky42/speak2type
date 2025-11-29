#!/bin/bash
# run_asr_demo.sh - Helper to run ASR demo with correct library paths

# Path to whisper.cpp build
BASE=$(pwd)/third_party/whisper.cpp
LIB_WHISPER=$BASE/build/src
LIB_GGML=$BASE/build/ggml/src
INCLUDE=$BASE/include:$BASE/ggml/include

# Export paths for CGO
export C_INCLUDE_PATH=$INCLUDE
export LIBRARY_PATH=$LIB_WHISPER:$LIB_GGML
export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:$LIB_WHISPER:$LIB_GGML:$(pwd)

# Run
go run -buildvcs=false cmd/asr-test/main.go
