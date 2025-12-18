#!/bin/bash
# scripts/run_session.sh
# Runs the full Session Orchestrator demo.

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Source environment
source "$PROJECT_ROOT/scripts/setup_env.sh"

echo "🚀 Running Session Orchestrator Demo..."
go run "$PROJECT_ROOT/cmd/session-test/main.go" "$@"
