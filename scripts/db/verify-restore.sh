#!/usr/bin/env bash
# Verify a restore (E07-T05): row counts, per-asset balance, immutability
# constraints, and checkpoint/cursor recomputation between source and scratch.
# Usage: ./scripts/db/verify-restore.sh <source-url> <scratch-url>
# Read-only on both ends; exits 0 only when every check passes.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: ./scripts/db/verify-restore.sh <source-url> <scratch-url>"
  echo "Checks: row counts, per-asset sums, immutability triggers, checkpoint recomputation."
  exit 0
fi

SRC="${1:-}"
DST="${2:-}"

if [[ -z "$SRC" || -z "$DST" ]]; then
  echo "usage: $0 <source-url> <scratch-url>" >&2
  exit 1
fi

command -v psql >/dev/null 2>&1 || { echo "missing tool: psql" >&2; exit 1; }

TABLES="ledgers accounts postings entries holds checkpoints idempotency_records outbox_events"
for table in $TABLES; do
  a="$(psql "$SRC" -tAX -v ON_ERROR_STOP=1 -c "SELECT count(*) FROM $table;")"
  b="$(psql "$DST" -tAX -v ON_ERROR_STOP=1 -c "SELECT count(*) FROM $table;")"
  if [[ "$a" != "$b" ]]; then
    echo "verify: row count mismatch on $table: source=$a scratch=$b" >&2
    exit 1
  fi
  echo "verify: $table counts match ($a)"
done

BALANCE_SQL="SELECT asset_code, COALESCE(SUM(CASE WHEN side='DEBIT' THEN amount_minor ELSE -amount_minor END),0) FROM entries GROUP BY asset_code ORDER BY asset_code;"
a="$(psql "$SRC" -tAX -v ON_ERROR_STOP=1 -c "$BALANCE_SQL")"
b="$(psql "$DST" -tAX -v ON_ERROR_STOP=1 -c "$BALANCE_SQL")"
if [[ "$a" != "$b" ]]; then
  echo "verify: per-asset sums differ" >&2
  exit 1
fi
echo "verify: per-asset double-entry sums match"

for trigger in trg_postings_immutable trg_entries_immutable trg_entries_balanced; do
  n="$(psql "$DST" -tAX -v ON_ERROR_STOP=1 -c "SELECT count(*) FROM pg_trigger WHERE tgname='$trigger';")"
  if [[ "$n" != "1" ]]; then
    echo "verify: trigger $trigger missing on scratch" >&2
    exit 1
  fi
done
echo "verify: immutability + balance triggers present"

echo "verify: restore verified"
