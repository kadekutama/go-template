#!/usr/bin/env bash
# Chaos scenarios for K8s (E00-T07; scenarios arrive in E16, wiring in E17).
# Usage: ./scripts/test/chaos.sh [-- <extra args>]
set -euo pipefail

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: ./scripts/test/chaos.sh [-- <extra args>]"
  echo "Applies chaos scenarios under test/chaos/ (needs a K8s cluster)."
  exit 0
fi

command -v kubectl >/dev/null 2>&1 || { echo "missing tool: kubectl (chaos runs target a K8s cluster)" >&2; exit 1; }

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

echo "chaos scenarios are defined in E16; nothing to apply yet (extra args reserved: $*)."
