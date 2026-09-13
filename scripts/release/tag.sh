#!/usr/bin/env bash
# Create an annotated release tag after validating semver (E00-T07).
# Usage: ./scripts/release/tag.sh <vMAJOR.MINOR.PATCH>
set -euo pipefail

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: ./scripts/release/tag.sh <vMAJOR.MINOR.PATCH>"
  echo "Creates an annotated tag; refuses non-semver input and existing tags."
  exit 0
fi

command -v git >/dev/null 2>&1 || { echo "missing tool: git" >&2; exit 1; }

VERSION="${1:?version argument required, e.g. v0.1.0}"
shift
[[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "invalid semver tag: $VERSION (want vMAJOR.MINOR.PATCH)" >&2; exit 1; }

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

git rev-parse -q --verify "refs/tags/$VERSION" >/dev/null && { echo "tag already exists: $VERSION" >&2; exit 1; }
exec git tag -a "$VERSION" -m "release $VERSION" "$@"
