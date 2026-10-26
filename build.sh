#!/bin/sh
set -eu

cd -- "$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
if ! command -v go >/dev/null 2>&1; then
    echo "Error: Go 1.23 or newer must be installed and available on PATH." >&2
    exit 1
fi
exec go run ./scripts/build.go
