#!/usr/bin/env bash
# Restore a backup into a scratch database (E07-T05). Never touches the source.
# Usage: ./scripts/db/restore.sh <dump-file> <scratch-database-url>
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: ./scripts/db/restore.sh <dump-file> <scratch-database-url>"
  echo "Restores into the scratch database only; the source is never written."
  exit 0
fi

DUMP="${1:-}"
SCRATCH="${2:-}"

if [[ -z "$DUMP" || -z "$SCRATCH" ]]; then
  echo "usage: $0 <dump-file> <scratch-database-url>" >&2
  exit 1
fi

if [[ ! -f "$DUMP" ]]; then
  echo "missing dump file: $DUMP" >&2
  exit 1
fi

command -v pg_restore >/dev/null 2>&1 || { echo "missing tool: pg_restore" >&2; exit 1; }

pg_restore --clean --if-exists --dbname="$SCRATCH" "$DUMP"
echo "restore: $DUMP restored into scratch database"
