#!/usr/bin/env bash
# k6 performance suites (E00-T07; suites arrive in E16, CI wiring in E17).
# Usage: ./scripts/test/performance.sh [-- <extra k6 args>]
set -euo pipefail

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: ./scripts/test/performance.sh [-- <extra k6 args>]"
  echo "Runs k6 suites under test/performance/."
  exit 0
fi

command -v k6 >/dev/null 2>&1 || { echo "missing tool: k6 (https://k6.io/docs/get-started/installation/)" >&2; exit 1; }

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

exec k6 run test/performance/baseline.js "$@"
