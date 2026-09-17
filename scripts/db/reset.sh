#!/usr/bin/env bash
# Reset the dev database (E07-T04): drop the public schema, migrate up, seed.
# Usage: ./scripts/db/reset.sh
# Requires: DATABASE_URL. Refuses production-looking hosts without --force.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: ./scripts/db/reset.sh [--force]"
  echo "Drops the public schema, migrates up, and seeds (dev only)."
  exit 0
fi

if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "missing env: DATABASE_URL" >&2
  exit 1
fi

case "$DATABASE_URL" in
  *prod*|*amazonaws.com*|*cloudsql*)
    if [[ "${1:-}" != "--force" ]]; then
      echo "refusing to reset a production-looking database (pass --force to override)" >&2
      exit 1
    fi
    ;;
esac

command -v psql >/dev/null 2>&1 || { echo "missing tool: psql" >&2; exit 1; }

psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
./scripts/db/seed.sh
