#!/usr/bin/env bash
# Render release notes from conventional commits (E00-T07).
# Usage: ./scripts/release/changelog.sh [<since-ref> [-- <extra git log args>]]
set -euo pipefail

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: ./scripts/release/changelog.sh [<since-ref>]"
  echo "Prints conventional-commit subject lines since <since-ref> (default: latest tag)."
  exit 0
fi

command -v git >/dev/null 2>&1 || { echo "missing tool: git" >&2; exit 1; }

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

SINCE="${1:-$(git describe --tags --abbrev=0 2>/dev/null || git rev-list --max-parents=0 HEAD)}"
if [ $# -gt 0 ]; then shift; fi
exec git log --oneline "${SINCE}..HEAD" "$@"
