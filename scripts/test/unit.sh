#!/usr/bin/env bash
# Fast unit tests: domain + application + pkg (E00-T07).
# Usage: ./scripts/test/unit.sh [-- <extra go test args>]
set -euo pipefail

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: ./scripts/test/unit.sh [-- <extra go test args>]"
  echo "Runs: go test ./internal/... ./pkg/..."
  exit 0
fi

command -v go >/dev/null 2>&1 || { echo "missing tool: go toolchain (https://go.dev/dl/)" >&2; exit 1; }

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

if ! find internal pkg -name "*.go" 2>/dev/null | grep -q .; then
  echo "no test packages in internal/... or pkg/... yet (arriving in E01+)"
  exit 0
fi

exec go test ./internal/... ./pkg/... "$@"
