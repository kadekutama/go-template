#!/usr/bin/env bash
# Regenerate repository mocks with mockery (E00-T07; consumed by E06-T06).
# Usage: ./scripts/generate/mocks.sh [-- <extra mockery args>]
set -euo pipefail

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: ./scripts/generate/mocks.sh [-- <extra mockery args>]"
  echo "Regenerates test/mock/ from domain/application ports (.mockery.yaml when added)."
  exit 0
fi

command -v mockery >/dev/null 2>&1 || { echo "missing tool: mockery (https://vektra.github.io/mockery/installation/)" >&2; exit 1; }

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

exec mockery "$@"
