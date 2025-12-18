#!/bin/bash
# scripts/print_env.sh
# Prints the environment variables in a format suitable for IDEs (or .env files).

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Source the setup script to get the values
source "$PROJECT_ROOT/scripts/setup_env.sh" > /dev/null

if [ "$1" == "--json" ]; then
    # JSON output
    echo "{"
    echo "  \"CGO_CFLAGS\": \"$CGO_CFLAGS\","
    echo "  \"CGO_LDFLAGS\": \"$CGO_LDFLAGS\","
    echo "  \"LD_LIBRARY_PATH\": \"$LD_LIBRARY_PATH\","
    echo "  \"DYLD_LIBRARY_PATH\": \"$DYLD_LIBRARY_PATH\""
    echo "}"
else
    # Standard Key=Value output
    echo "CGO_CFLAGS=$CGO_CFLAGS"
    echo "CGO_LDFLAGS=$CGO_LDFLAGS"
    echo "LD_LIBRARY_PATH=$LD_LIBRARY_PATH"
    echo "DYLD_LIBRARY_PATH=$DYLD_LIBRARY_PATH"
fi
