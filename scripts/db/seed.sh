#!/usr/bin/env bash
# Seed the dev database (E07-T04): migrate up, then apply the deterministic
# dev plan (idempotent on stable IDs).
# Usage: ./scripts/db/seed.sh
# Requires: DATABASE_URL.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: ./scripts/db/seed.sh"
  echo "Applies migrations then seeds the dev tenant (idempotent)."
  exit 0
fi

if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "missing env: DATABASE_URL" >&2
  exit 1
fi

./scripts/db/migrate.sh up

go run ./cmd/seed

echo "seed: done (idempotent on stable fixture IDs)"
