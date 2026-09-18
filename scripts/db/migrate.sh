#!/usr/bin/env bash
# Ledger migration runner (E07.1-T01, ADR-018). Drives Pressly Goose v3 for
# in-process-compatible CLI execution and Ariga Atlas for safety linting and
# checksum validation.
# Usage: ./scripts/db/migrate.sh up|down|status|version|reset|create NAME=...|lint
# Requires: DATABASE_URL (postgres DSN) for live commands; Atlas dev-database
# URL defaults to a dockerized PostgreSQL 18 for lint.
set -euo pipefail

usage() {
  echo "Usage: $0 up|down [N]|status|version|reset|create NAME=...|lint"
  echo "Requires DATABASE_URL (e.g. postgres://ledger:pw@localhost:5432/ledger?sslmode=disable)"
}

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

MIGRATIONS_DIR="$ROOT/internal/infrastructure/database/migration/versions"
ATLAS_DIR_URL="file://$MIGRATIONS_DIR?format=goose"

need_goose() {
  command -v goose >/dev/null 2>&1 || {
    echo "goose CLI not found; install v3.28.0 (go install github.com/pressly/goose/v3/cmd/goose@v3.28.0) or run ./scripts/dev/setup.sh" >&2
    exit 1
  }
}

need_atlas() {
  command -v atlas >/dev/null 2>&1 || {
    echo "atlas CLI not found; install v1.3.0 (https://atlasgo.io) or run ./scripts/dev/setup.sh" >&2
    exit 1
  }
}

rehash() {
  need_atlas
  atlas migrate hash --dir "$ATLAS_DIR_URL"
}

cmd="${1:-}"
case "$cmd" in
  up)
    [[ -z "${DATABASE_URL:-}" ]] && { echo "missing env: DATABASE_URL" >&2; exit 1; }
    need_goose
    goose -dir "$MIGRATIONS_DIR" postgres "$DATABASE_URL" up
    ;;
  down)
    [[ -z "${DATABASE_URL:-}" ]] && { echo "missing env: DATABASE_URL" >&2; exit 1; }
    need_goose
    steps="${2:-1}"
    for ((i = 0; i < steps; i++)); do
      goose -dir "$MIGRATIONS_DIR" postgres "$DATABASE_URL" down
    done
    ;;
  status)
    [[ -z "${DATABASE_URL:-}" ]] && { echo "missing env: DATABASE_URL" >&2; exit 1; }
    need_goose
    goose -dir "$MIGRATIONS_DIR" postgres "$DATABASE_URL" status
    ;;
  version)
    [[ -z "${DATABASE_URL:-}" ]] && { echo "missing env: DATABASE_URL" >&2; exit 1; }
    need_goose
    goose -dir "$MIGRATIONS_DIR" postgres "$DATABASE_URL" version
    ;;
  reset)
    [[ -z "${DATABASE_URL:-}" ]] && { echo "missing env: DATABASE_URL" >&2; exit 1; }
    need_goose
    goose -dir "$MIGRATIONS_DIR" postgres "$DATABASE_URL" reset
    ;;
  create)
    name="${NAME:-${2:-}}"
    [[ -z "$name" ]] && { echo "create needs NAME=... (e.g. make migrate-create NAME=add_index)" >&2; exit 1; }
    stamp="$(date -u +%Y%m%d%H%M%S)"
    file="$MIGRATIONS_DIR/${stamp}_${name}.sql"
    cat > "$file" <<EOF
-- ${stamp}_${name}: describe the change here.

-- +goose Up
-- TODO: forward DDL.

-- +goose Down
-- TODO: reverse DDL.
EOF
    echo "created $file"
    rehash
    ;;
  lint)
    need_atlas
    atlas migrate validate --dir "$ATLAS_DIR_URL"
    atlas migrate lint --dir "$ATLAS_DIR_URL" --dev-url "${ATLAS_DEV_URL:-docker://postgres/18/dev}" --latest 1
    ;;
  -h|--help|"") usage ;;
  *) echo "unknown command: $cmd" >&2; usage; exit 1 ;;
esac
