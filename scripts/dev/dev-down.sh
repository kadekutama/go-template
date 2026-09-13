#!/usr/bin/env bash
# Stop the core dependency set for local development (E00-T04).
# Usage: ./scripts/dev/dev-down.sh [-- <extra docker compose args>]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

exec docker compose \
  -f deployments/docker/docker-compose.yml \
  down "$@"
