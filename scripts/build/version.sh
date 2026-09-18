#!/usr/bin/env bash
# Print the release version for binary stamping (E00-T07-R05).
# Usage: ./scripts/build/version.sh
# Priority: $VERSION override > `git describe --tags` > count+SHA fallback.
# Prints `dev` only when git is unavailable (plain `go build` then also
# yields "dev", which is the honest unstamped-build signal).
#
# Scheme notes:
# - Tagged commits print `git describe --tags --dirty`: v1.4.2 on the tag,
#   v1.4.2-7-gabc1234 seven commits after it, -dirty suffix on modifications.
# - Untagged history prints v0.0.0-<count>-g<sha>[-dirty]: the commit count
#   is the autoincrement, the SHA the unique identity.
# - Timestamps are deliberately NOT used: they break reproducible builds
#   (same source must yield the same version string).
# - Docker builds usually exclude .git: CI computes the version where git
#   exists and passes it in as VERSION / --build-arg (E17 wires build-args).
set -euo pipefail

if [[ -n "${VERSION:-}" ]]; then
  echo "$VERSION"
  exit 0
fi

if ! git rev-parse --git-dir >/dev/null 2>&1; then
  echo "dev"
  exit 0
fi

if described="$(git describe --tags 2>/dev/null)" && [[ -n "$described" ]]; then
  # --dirty is appended manually (not via describe --dirty) so untracked new
  # source files count too: they compile into the binary but describe --dirty
  # ignores them, which would stamp a dirty build as clean.
  if [[ -n "$(git status --porcelain 2>/dev/null)" ]]; then
    described="${described}-dirty"
  fi

  echo "$described"
  exit 0
fi

count="$(git rev-list --count HEAD 2>/dev/null || echo 0)"
sha="$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")"
dirty=""
if [[ -n "$(git status --porcelain 2>/dev/null)" ]]; then
  dirty="-dirty"
fi

echo "v0.0.0-${count}-g${sha}${dirty}"
