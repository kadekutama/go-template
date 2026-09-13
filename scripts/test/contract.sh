#!/usr/bin/env bash
# Consumer-driven contract tests (Pact) (E00-T07; broker wiring arrives in E16).
# Usage: ./scripts/test/contract.sh [-- <extra go test args>]
set -euo pipefail

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: ./scripts/test/contract.sh [-- <extra go test args>]"
  echo "Runs: go test ./test/contract/..."
  exit 0
fi

command -v go >/dev/null 2>&1 || { echo "missing tool: go toolchain (https://go.dev/dl/)" >&2; exit 1; }

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

exec go test ./test/contract/... "$@"
