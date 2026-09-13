#!/usr/bin/env bash
# Tail dependency service logs for local development (E00-T04).
# Usage: ./scripts/dev/dev-logs.sh [service...] [-- <extra compose logs args>]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

exec docker compose \
  -f deployments/docker/docker-compose.yml \
  logs -f "$@"
