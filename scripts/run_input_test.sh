#!/bin/bash
# scripts/run_input_test.sh
# Usage: ./scripts/run_input_test.sh -text "Hello" -delay 3

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Source environment
source "$PROJECT_ROOT/scripts/setup_env.sh"

echo "🚀 Running Input Injection Test..."
echo "⚠️  NOTE: Will try to type into the active window after delay."
go run "$PROJECT_ROOT/cmd/input-test/main.go" "$@"
