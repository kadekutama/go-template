#!/usr/bin/env bash
# Regenerate the OpenAPI-derived artifacts (E00-T07; consumed by E11-T08).
# Usage: ./scripts/generate/openapi.sh [-- <extra oapi-codegen args>]
set -euo pipefail

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: ./scripts/generate/openapi.sh [-- <extra oapi-codegen args>]"
  echo "Runs oapi-codegen against api/openapi/."
  exit 0
fi

command -v oapi-codegen >/dev/null 2>&1 || { echo "missing tool: oapi-codegen (go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest)" >&2; exit 1; }

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

exec oapi-codegen "$@" api/openapi/openapi.yaml
