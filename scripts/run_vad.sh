#!/bin/bash
# scripts/run_vad.sh
# Runs the VAD demo.

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Source environment
source "$PROJECT_ROOT/scripts/setup_env.sh"

echo "🚀 Running VAD Demo..."
# Pass all arguments to the go run command
go run "$PROJECT_ROOT/cmd/vad-test/main.go" "$@"
