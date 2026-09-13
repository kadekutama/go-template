#!/usr/bin/env bash
# Regenerate protobuf/gRPC stubs with buf (E00-T07; consumed by E12).
# Usage: ./scripts/generate/proto.sh [-- <extra buf args>]
set -euo pipefail

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: ./scripts/generate/proto.sh [-- <extra buf args>]"
  echo "Runs buf generate against api/proto/."
  exit 0
fi

command -v buf >/dev/null 2>&1 || { echo "missing tool: buf (https://buf.build/docs/installation/)" >&2; exit 1; }

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

exec buf generate "$@"
