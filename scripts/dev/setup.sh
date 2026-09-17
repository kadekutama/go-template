#!/usr/bin/env bash
# Single setup script: installs pixi, toolchains, system tools, Go modules, and dev CLI tools.
# Usage: ./scripts/dev/setup.sh [--check|--pixi-only|--tools-only|--modules-only]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

show_help() {
  cat <<'HELP'
Usage: ./scripts/dev/setup.sh [OPTIONS]

Single-command developer environment and dependency setup script.
Installs and verifies pixi, core toolchains (Go, Clang, Make, Docker, Shellcheck, k6, syft),
Go modules, and repository CLI tools (goimports, golangci-lint, mockery, buf, oapi-codegen, etc.).

Options:
  --check         Inspect and report status of all required dependencies without installing
  --pixi-only     Only check/install pixi and global pixi packages
  --tools-only    Only install Go development CLI tools (go install ...)
  --modules-only  Only download and verify Go modules (go mod download/verify)
  -h, --help      Show this help message and exit
HELP
}

CHECK_ONLY=false
PIXI_ONLY=false
TOOLS_ONLY=false
MODULES_ONLY=false
FAILED_PIXI_PKGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --check)
      CHECK_ONLY=true
      shift
      ;;
    --pixi-only)
      PIXI_ONLY=true
      shift
      ;;
    --tools-only)
      TOOLS_ONLY=true
      shift
      ;;
    --modules-only)
      MODULES_ONLY=true
      shift
      ;;
    -h|--help)
      show_help
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      show_help >&2
      exit 1
      ;;
  esac
done

PIXI_BIN="$HOME/.pixi/bin"

# 1. Check or install Pixi
ensure_pixi() {
  if ! command -v pixi >/dev/null 2>&1; then
    if [[ -x "$PIXI_BIN/pixi" ]]; then
      export PATH="$PIXI_BIN:$PATH"
    else
      echo "==> Pixi not found. Installing pixi via official installer..."
      if command -v curl >/dev/null 2>&1; then
        curl -fsSL https://pixi.sh/install.sh | bash
      elif command -v wget >/dev/null 2>&1; then
        wget -qO- https://pixi.sh/install.sh | bash
      else
        echo "Error: Neither curl nor wget is available to download pixi." >&2
        exit 1
      fi
      export PATH="$PIXI_BIN:$PATH"
    fi
  fi

  # Ensure PIXI_BIN is on PATH for the remainder of this script
  if [[ ":$PATH:" != *":$PIXI_BIN:"* ]]; then
    export PATH="$PIXI_BIN:$PATH"
  fi
  echo "==> Pixi available: $(command -v pixi) ($(pixi --version 2>/dev/null || echo 'unknown'))"
}

# Core system toolchains managed via pixi global
PIXI_PACKAGES=(
  "go:Go toolchain"
  "clang:C compiler for CGO and race detector"
  "clangxx:C++ compiler"
  "make:GNU Make build tool"
  "shellcheck:Shell script linter"
  "docker-cli:Docker CLI"
  "docker-compose:Docker Compose v2"
  "kubernetes-client:Kubectl CLI for chaos tests"
  "k6:Load testing tool for performance tests"
  "syft:SPDX SBOM generator for releases"
  "gopls:Go language server"
)

install_pixi_packages() {
  echo "==> Ensuring Pixi global packages are installed..."
  local installed_list
  installed_list="$(pixi global list 2>/dev/null || true)"

  for item in "${PIXI_PACKAGES[@]}"; do
    pkg="${item%%:*}"
    desc="${item#*:}"
    if echo "$installed_list" | grep -qE "^${pkg} "; then
      echo "  [OK] pixi: $pkg ($desc)"
    else
      echo "  -> Installing pixi package: $pkg ($desc)..."
      # Docker CLI/Compose are hard requirements for integration tests, but a
      # failed install must not abort the whole toolchain setup: record it and
      # report actionable guidance at the end (see check_docker).
      if ! pixi global install "$pkg"; then
        echo "  [WARN] pixi install failed for: $pkg ($desc)" >&2
        FAILED_PIXI_PKGS+=("$pkg")
      fi
    fi
  done
  echo "==> Pixi global packages ready."
}

# check_docker verifies the Docker *client* binaries and, separately, daemon
# reachability. Pixi only ships the client (docker-cli, docker-compose); the
# daemon comes from the host (Rancher Desktop, Docker Desktop, or dockerd).
# Never fatal: unit tests and linting work without a daemon, and
# container-gated tests skip cleanly (see test/testcontainers SkipIfNoDocker).
check_docker() {
  echo "--- Docker Client & Daemon ---"
  local ok=true

  for b in docker docker-compose; do
    if command -v "$b" >/dev/null 2>&1; then
      echo "  [OK]      $b ($(command -v "$b"))"
    elif [[ -x "$PIXI_BIN/$b" ]]; then
      echo "  [OK]      $b ($PIXI_BIN/$b)"
    else
      echo "  [MISSING] $b (run ./scripts/dev/setup.sh to install via pixi)"
      ok=false
    fi
  done

  if [[ "$ok" == "true" ]] && docker info >/dev/null 2>&1; then
    echo "  [OK]      docker daemon ($(docker version --format '{{.Server.Version}}' 2>/dev/null || echo 'reachable'))"
  else
    echo "  [WARN]    docker daemon unreachable."
    echo "            Pixi provides the client only; start an engine first:"
    echo "            - Rancher Desktop (Windows): launch the app with WSL integration enabled, or"
    echo "            - Native Linux: sudo systemctl start docker (or sudo dockerd), or"
    echo "            - Remote: export DOCKER_HOST=tcp://<host>:2375"
    echo "            Without a daemon, Testcontainers suites skip (TESTCONTAINERS_SKIP=1 forces skip)."
  fi
  echo ""
}

configure_go_env() {
  if command -v go >/dev/null 2>&1; then
    echo "==> Configuring Go environment (CGO_ENABLED=1, CC=clang)..."
    go env -w CGO_ENABLED=1 CC=clang
  fi
}

install_modules() {
  echo "==> Downloading and verifying Go modules..."
  go mod download
  go mod verify
  echo "==> Go modules verified successfully."
}

# Go developer tools: "binary_name:package_path"
GO_TOOLS=(
  "goimports:golang.org/x/tools/cmd/goimports@latest"
  "golangci-lint:github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2"
  "govulncheck:golang.org/x/vuln/cmd/govulncheck@latest"
  "gosec:github.com/securego/gosec/v2/cmd/gosec@latest"
  "go-licenses:github.com/google/go-licenses@latest"
  "mockery:github.com/vektra/mockery/v2@latest"
  "buf:github.com/bufbuild/buf/cmd/buf@latest"
  "oapi-codegen:github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest"
  "air:github.com/air-verse/air@latest"
  "migrate:github.com/golang-migrate/migrate/v4/cmd/migrate@latest"
)

get_gobin() {
  local bin
  bin="$(go env GOBIN 2>/dev/null || true)"
  if [[ -z "$bin" ]]; then
    local gpath
    gpath="$(go env GOPATH 2>/dev/null || true)"
    bin="${gpath:-$HOME/go}/bin"
  fi
  echo "$bin"
}

install_go_tools() {
  local gobin
  gobin="$(get_gobin)"
  echo "==> Installing Go CLI tools into $gobin..."
  mkdir -p "$gobin"

  for item in "${GO_TOOLS[@]}"; do
    bin="${item%%:*}"
    pkg="${item#*:}"
    echo "  -> Installing $bin ($pkg)..."
    if [[ "$bin" == "migrate" ]]; then
      go install -tags 'postgres' "$pkg"
    else
      go install "$pkg"
    fi

    if [[ -d "$PIXI_BIN" && -w "$PIXI_BIN" && -f "$gobin/$bin" ]]; then
      ln -sf "$gobin/$bin" "$PIXI_BIN/$bin"
    fi
  done
  echo "==> Go CLI tools installed successfully."
}

check_status() {
  echo "=== Environment & Toolchain Status Report ==="
  echo ""

  echo "--- Pixi Package Manager ---"
  if command -v pixi >/dev/null 2>&1; then
    echo "  [OK]      pixi ($(command -v pixi), $(pixi --version 2>/dev/null || echo 'installed'))"
  elif [[ -x "$PIXI_BIN/pixi" ]]; then
    echo "  [OK]      pixi ($PIXI_BIN/pixi)"
  else
    echo "  [MISSING] pixi (run ./scripts/dev/setup.sh to install)"
  fi
  echo ""

  echo "--- Toolchains & System Dependencies (Pixi-managed) ---"
  local check_bins=("go" "clang" "clang++" "make" "shellcheck" "kubectl" "k6" "syft" "gopls")
  for b in "${check_bins[@]}"; do
    if command -v "$b" >/dev/null 2>&1; then
      loc="$(command -v "$b")"
      echo "  [OK]      $b ($loc)"
    elif [[ -x "$PIXI_BIN/$b" ]]; then
      echo "  [OK]      $b ($PIXI_BIN/$b)"
    else
      echo "  [MISSING] $b"
    fi
  done
  echo ""

  check_docker

  echo "--- Go Developer CLI Tools ---"
  local gobin
  gobin="$(get_gobin)"
  for item in "${GO_TOOLS[@]}"; do
    bin="${item%%:*}"
    pkg="${item#*:}"
    if command -v "$bin" >/dev/null 2>&1; then
      loc="$(command -v "$bin")"
      echo "  [OK]      $bin ($loc)"
    elif [[ -x "$gobin/$bin" ]]; then
      echo "  [OK]      $bin ($gobin/$bin)"
    else
      echo "  [MISSING] $bin (install with: go install $pkg)"
    fi
  done
  echo ""
}

if [[ "$CHECK_ONLY" == "true" ]]; then
  check_status
  exit 0
fi

# Step 1: Ensure Pixi
ensure_pixi

# Step 2: Install Pixi Packages
if [[ "$TOOLS_ONLY" == "false" && "$MODULES_ONLY" == "false" ]]; then
  install_pixi_packages
  configure_go_env
fi

if [[ "$PIXI_ONLY" == "true" ]]; then
  echo ""
  check_status
  echo "==> Pixi setup completed successfully."
  exit 0
fi

# Step 3: Go Modules
if [[ "$TOOLS_ONLY" == "false" ]]; then
  install_modules
fi

# Step 4: Go Developer Tools
if [[ "$MODULES_ONLY" == "false" ]]; then
  install_go_tools
fi

echo ""
check_status
if [[ "${#FAILED_PIXI_PKGS[@]}" -gt 0 ]]; then
  echo "==> Setup completed WITH WARNINGS: pixi install failed for: ${FAILED_PIXI_PKGS[*]}" >&2
  echo "    Re-run ./scripts/dev/setup.sh once the network/registry issue is resolved." >&2
else
  echo "==> Full environment & dependency setup completed successfully."
fi
