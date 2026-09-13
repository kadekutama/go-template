#!/usr/bin/env bash
# Check dependency licenses (E00-T07; CI wiring arrives in E17).
# Usage: ./scripts/security/licenses.sh [-- <extra go-licenses args>]
set -euo pipefail

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: ./scripts/security/licenses.sh"
  echo "Reports the license of every module dependency."
  exit 0
fi

command -v go-licenses >/dev/null 2>&1 || { echo "missing tool: go-licenses (go install github.com/google/go-licenses@latest)" >&2; exit 1; }

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

exec go-licenses report ./... "$@"
