#!/usr/bin/env bash
# Fast unit tests: domain + application + pkg (E00-T07) + mirror suites (E02-T09).
# Usage: ./scripts/test/unit.sh [-- <extra go test args>]
set -euo pipefail

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: ./scripts/test/unit.sh [-- <extra go test args>]"
  echo "Runs: go test ./internal/... ./pkg/... ./test/unit/..."
  exit 0
fi

command -v go >/dev/null 2>&1 || { echo "missing tool: go toolchain (https://go.dev/dl/)" >&2; exit 1; }

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

# Ensure Pixi toolchain and compiler are on PATH if available
if [[ -d "${HOME}/.pixi/bin" && ":${PATH}:" != *":${HOME}/.pixi/bin:"* ]]; then
  export PATH="${HOME}/.pixi/bin:${PATH}"
fi

if [[ -z "${CC:-}" ]]; then
  if command -v clang >/dev/null 2>&1; then
    export CC=clang
  elif command -v gcc >/dev/null 2>&1; then
    export CC=gcc
  fi
fi

if [ -z "$(find internal pkg -name "*.go" -print -quit 2>/dev/null)" ]; then
  echo "no test packages in internal/... or pkg/... yet (arriving in E01+)"
  exit 0
fi

exec go test ./internal/... ./pkg/... ./test/unit/... "$@"
