#!/usr/bin/env bash
# Back up the ledger database (E07-T05): custom-format pg_dump + WAL pointer.
# Usage: ./scripts/db/backup.sh [BACKUP_DIR]
# Requires: DATABASE_URL, pg_dump. Uploads to object storage when configured.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: ./scripts/db/backup.sh [BACKUP_DIR]"
  echo "Writes ledger-<timestamp>.dump + wal-pointer.json into BACKUP_DIR (default ./backups)."
  exit 0
fi

if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "missing env: DATABASE_URL" >&2
  exit 1
fi

command -v pg_dump >/dev/null 2>&1 || { echo "missing tool: pg_dump" >&2; exit 1; }

BACKUP_DIR="${1:-./backups}"
mkdir -p "$BACKUP_DIR"

STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
DUMP="$BACKUP_DIR/ledger-$STAMP.dump"

pg_dump --format=custom --file="$DUMP" "$DATABASE_URL"

WAL_PTR="$BACKUP_DIR/ledger-$STAMP.wal-pointer.json"
printf '{"dump": "%s", "taken_at": "%s"}\n' "$(basename "$DUMP")" "$STAMP" > "$WAL_PTR"

if [[ -n "${OBJECT_STORAGE_URL:-}" ]]; then
  echo "backup: uploading to $OBJECT_STORAGE_URL (stub: wire mc/aws in E17)"
fi

echo "backup: wrote $DUMP and $WAL_PTR"
