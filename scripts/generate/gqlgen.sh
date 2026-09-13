#!/usr/bin/env bash
# Regenerate the GraphQL resolver layer with gqlgen (E00-T07; consumed by E13).
# Usage: ./scripts/generate/gqlgen.sh [-- <extra gqlgen args>]
set -euo pipefail

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: ./scripts/generate/gqlgen.sh [-- <extra gqlgen args>]"
  echo "Runs gqlgen generate against api/graphql/ schemas."
  exit 0
fi

command -v go >/dev/null 2>&1 || { echo "missing tool: go toolchain (https://go.dev/dl/)" >&2; exit 1; }

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

exec go run github.com/99designs/gqlgen generate "$@"
