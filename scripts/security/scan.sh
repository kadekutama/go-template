#!/usr/bin/env bash
# Vulnerability + static-security scan (E00-T07; CI wiring arrives in E17).
# Usage: ./scripts/security/scan.sh [-- <extra args>]
set -euo pipefail

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: ./scripts/security/scan.sh"
  echo "Runs govulncheck and gosec over the module."
  exit 0
fi

command -v govulncheck >/dev/null 2>&1 || { echo "missing tool: govulncheck (go install golang.org/x/vuln/cmd/govulncheck@latest)" >&2; exit 1; }
command -v gosec >/dev/null 2>&1 || { echo "missing tool: gosec (https://github.com/securego/gosec#install)" >&2; exit 1; }

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

govulncheck ./... "$@" && gosec ./... "$@"
