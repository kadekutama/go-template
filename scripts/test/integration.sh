#!/usr/bin/env bash
# Integration tests against testcontainers (needs a Docker daemon) (E00-T07).
# Usage: ./scripts/test/integration.sh [-- <extra go test args>]
set -euo pipefail

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: ./scripts/test/integration.sh [-- <extra go test args>]"
  echo "Runs: go test ./test/integration/... (spins up Postgres/Valkey/NATS via testcontainers)"
  exit 0
fi

command -v go >/dev/null 2>&1 || { echo "missing tool: go toolchain (https://go.dev/dl/)" >&2; exit 1; }
command -v docker >/dev/null 2>&1 || { echo "missing tool: docker (daemon required for testcontainers)" >&2; exit 1; }

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

if ! find test/integration -name "*.go" 2>/dev/null | grep -q .; then
  echo "no integration test packages in test/integration/... yet (arriving in E07+)"
  exit 0
fi

exec go test ./test/integration/... "$@"
