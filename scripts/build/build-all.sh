#!/usr/bin/env bash
# Build the 5 service binaries into bin/ (E00-T07).
# Usage: ./scripts/build/build-all.sh
set -euo pipefail

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: ./scripts/build/build-all.sh [-- <extra go build args>]"
  echo "Builds rest-api, grpc-api, graphql-api, cron, consumer into bin/."
  exit 0
fi

command -v go >/dev/null 2>&1 || { echo "missing tool: go toolchain (https://go.dev/dl/)" >&2; exit 1; }

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

mkdir -p bin
for b in rest-api grpc-api graphql-api cron consumer; do
  go build "$@" -o "bin/$b" "./cmd/$b"
done
