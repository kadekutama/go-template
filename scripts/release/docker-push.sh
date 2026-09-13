#!/usr/bin/env bash
# Build and push service images (E00-T07; Dockerfiles arrive in E17).
# Usage: ./scripts/release/docker-push.sh [<tag>] [-- <extra docker buildx args>]
set -euo pipefail

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: ./scripts/release/docker-push.sh [<tag>]"
  echo "Builds and pushes service images (default tag: dev)."
  exit 0
fi

command -v docker >/dev/null 2>&1 || { echo "missing tool: docker" >&2; exit 1; }

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

TAG="${1:-dev}"
if [ $# -gt 0 ]; then shift; fi
echo "image publish for tag $TAG is wired in E17; Dockerfiles not yet present (extra args reserved: $*)."
