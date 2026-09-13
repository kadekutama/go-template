#!/usr/bin/env bash
# Emit an SPDX SBOM for the release (E00-T07; CI wiring arrives in E17).
# Usage: ./scripts/security/sbom.sh [-- <extra syft args>]
set -euo pipefail

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: ./scripts/security/sbom.sh"
  echo "Writes sbom.spdx.json via syft."
  exit 0
fi

command -v syft >/dev/null 2>&1 || { echo "missing tool: syft (https://github.com/anchore/syft#installation)" >&2; exit 1; }

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

exec syft . -o spdx-json=sbom.spdx.json "$@"
