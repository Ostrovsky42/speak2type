#!/bin/bash
# scripts/run_asr.sh
# Runs the ASR demo.

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Source environment
source "$PROJECT_ROOT/scripts/setup_env.sh"

echo "🚀 Running ASR Demo..."
go run "$PROJECT_ROOT/cmd/asr-test/main.go" "$@"
