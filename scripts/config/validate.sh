#!/usr/bin/env bash
# Validate committed config YAMLs against the loader + JSON schema alignment (E01-T03).
# Usage: ./scripts/config/validate.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

exec go test ./internal/infrastructure/config/ "$@"
