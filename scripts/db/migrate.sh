#!/usr/bin/env bash
# Ledger migration runner (E07-T01). Applies embedded versioned SQL pairs.
# Usage: ./scripts/db/migrate.sh up|down|version|force|create NAME=...
# Requires: DATABASE_URL (postgres DSN). Uses the `migrate` CLI when present,
# otherwise the embedded Go runner.
set -euo pipefail

usage() {
  echo "Usage: $0 up|down [N]|version|force VERSION|create NAME=..."
  echo "Requires DATABASE_URL (e.g. postgres://ledger:pw@localhost:5432/ledger?sslmode=disable)"
}

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

MIGRATIONS_DIR="$ROOT/internal/infrastructure/database/migration/versions"

cmd="${1:-}"
case "$cmd" in
  up|down|version|force)
    if [[ -z "${DATABASE_URL:-}" ]]; then
      echo "missing env: DATABASE_URL" >&2
      exit 1
    fi
    if command -v migrate >/dev/null 2>&1; then
      case "$cmd" in
        up) exec migrate -path "$MIGRATIONS_DIR" -database "$DATABASE_URL" up ;;
        down)
          steps="${2:-1}"
          exec migrate -path "$MIGRATIONS_DIR" -database "$DATABASE_URL" down "$steps" ;;
        version) exec migrate -path "$MIGRATIONS_DIR" -database "$DATABASE_URL" version ;;
        force)
          ver="${2:-}"
          if [[ -z "$ver" ]]; then echo "force needs VERSION" >&2; exit 1; fi
          exec migrate -path "$MIGRATIONS_DIR" -database "$DATABASE_URL" force "$ver" ;;
      esac
    fi
    echo "migrate CLI not found; install golang-migrate v4.19.1 (https://github.com/golang-migrate/migrate) and retry" >&2
    exit 1
    ;;
  create)
    name="${NAME:-${2:-}}"
    if [[ -z "$name" ]]; then echo "create needs NAME=... (e.g. make migrate-create NAME=add_index)" >&2; exit 1; fi
    last="$(find "$MIGRATIONS_DIR" -name '*.up.sql' | sort | tail -n 1)"
    if [[ -z "$last" ]]; then next="000001"; else
      num="$(basename "$last" | cut -d_ -f1)"
      next="$(printf '%06d' "$((10#$num + 1))")"
    fi
    up="$MIGRATIONS_DIR/${next}_${name}.up.sql"
    down="$MIGRATIONS_DIR/${next}_${name}.down.sql"
    printf -- '-- %s (up)\n' "$name" > "$up"
    printf -- '-- %s (down)\n' "$name" > "$down"
    echo "created $up and $down"
    ;;
  -h|--help|"") usage ;;
  *) echo "unknown command: $cmd" >&2; usage; exit 1 ;;
esac
