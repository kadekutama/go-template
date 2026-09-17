SHELL := /usr/bin/env bash
GOPATH_BIN := $(shell go env GOPATH 2>/dev/null)/bin
PIXI_BIN := $(HOME)/.pixi/bin
export PATH := $(PIXI_BIN):$(GOPATH_BIN):$(PATH)

# Binaries and tools (override with `make TOOL=...` only in local shell, never committed)
GO ?= go
GOLANGCI_LINT ?= golangci-lint
GOPLS ?= gopls
GOIMPORTS ?= goimports
LOCAL_MODULE := github.com/kadekutama/go-template
BIN_DIR := bin
MIGRATE ?= migrate

# Compiler for CGO / race detector (override Make default 'cc' if clang/gcc present)
ifeq ($(origin CC),default)
  CC := $(shell command -v clang 2>/dev/null || command -v gcc 2>/dev/null || echo cc)
endif
export CC

# Script-backed targets fail loudly (never silently) while the owning task
# (E00-T04/E00-T07/E07-T05/...) has not landed its script yet.
define need-script
@test -x "$1" || (echo "missing script: $1 (not yet implemented; see tasks/epics)" >&2; exit 1)
endef

.PHONY: help
help: ## List every target with a one-line description.
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-16s %s\n", $$1, $$2}'

.PHONY: setup
setup: ## Single-command environment bootstrap: installs pixi, toolchains, tools, and modules.
	$(call need-script,./scripts/dev/setup.sh)
	./scripts/dev/setup.sh

.PHONY: deps
deps: ## Download Go modules and install developer CLI tools (scripts/dev/setup.sh).
	$(call need-script,./scripts/dev/setup.sh)
	./scripts/dev/setup.sh

.PHONY: build
build: ## Build all packages (no output binaries).
	$(GO) build ./...

.PHONY: build-all
build-all: ## Build the 5 binaries into bin/.
	./scripts/build/build-all.sh

.PHONY: test
test: test-unit ## Default test entry point (unit only; see test-all).

.PHONY: test-unit
test-unit: ## Fast unit tests (domain + application + pkg).
	./scripts/test/unit.sh

.PHONY: test-race
test-race: ## Fast unit tests with race detector enabled (requires CGO).
	CGO_ENABLED=1 CC=$(CC) $(GO) test -v -race ./...

.PHONY: test-integration
test-integration: ## Integration tests against testcontainers (needs Docker).
	$(call need-script,./scripts/test/integration.sh)
	./scripts/test/integration.sh

.PHONY: test-contract
test-contract: ## Consumer-driven contract tests (Pact).
	$(call need-script,./scripts/test/contract.sh)
	./scripts/test/contract.sh

.PHONY: test-all
test-all: ## Full suite: unit + integration + contract.
	./scripts/test/unit.sh && ./scripts/test/integration.sh && ./scripts/test/contract.sh

.PHONY: gopls-check
gopls-check: ## Run gopls diagnostics across all Go source files.
	@find . -name "*.go" -not -path "./vendor/*" | xargs $(GOPLS) check

.PHONY: fmt
fmt: ## Format all Go code (gofmt -s + goimports with local module grouping).
	gofmt -s -w .
	$(GOIMPORTS) -local $(LOCAL_MODULE) -w .

.PHONY: fmt-check
fmt-check: ## Check that all Go files are formatted with gofmt -s and goimports.
	@test -z "$$(gofmt -s -l .)" || (echo "gofmt -s check failed on files:" >&2; gofmt -s -l . >&2; exit 1)
	@test -z "$$($(GOIMPORTS) -local $(LOCAL_MODULE) -l .)" || (echo "goimports check failed on files:" >&2; $(GOIMPORTS) -local $(LOCAL_MODULE) -l . >&2; exit 1)

.PHONY: lint
lint: fmt-check gopls-check ## Strict lint (fmt check + gopls check + golangci-lint) + shellcheck on scripts.
	CGO_ENABLED=0 $(GOLANGCI_LINT) run ./... && shellcheck scripts/**/*.sh

.PHONY: verify
verify: fmt lint test-race ## Full pre-commit verification: fmt, lint, race tests, and SDD checks.
	python3 tasks/scripts/check-tasks.py --format --graph --sdd --specs --events --codes
	@echo "All pre-commit verification checks passed successfully!"

.PHONY: vet
vet: ## go vet over the tree.
	$(GO) vet ./...

.PHONY: generate
generate: ## Regenerate mocks, protobuf, GraphQL, OpenAPI artifacts.
	$(call need-script,./scripts/generate/mocks.sh)
	$(call need-script,./scripts/generate/proto.sh)
	$(call need-script,./scripts/generate/gqlgen.sh)
	$(call need-script,./scripts/generate/openapi.sh)
	./scripts/generate/mocks.sh && ./scripts/generate/proto.sh && ./scripts/generate/gqlgen.sh && ./scripts/generate/openapi.sh

.PHONY: generate-mocks
generate-mocks: ## Regenerate testify mocks for domain + application ports (E06-T12).
	$(call need-script,./scripts/generate/mocks.sh)
	./scripts/generate/mocks.sh

.PHONY: migrate-up
migrate-up: ## Apply all pending migrations (DATABASE_URL required).
	$(call need-script,./scripts/db/migrate.sh)
	./scripts/db/migrate.sh up

.PHONY: migrate-down
migrate-down: ## Roll back one migration (DATABASE_URL required).
	$(call need-script,./scripts/db/migrate.sh)
	./scripts/db/migrate.sh down 1

.PHONY: migrate-create
migrate-create: ## Create a new migration pair (usage: make migrate-create NAME=...).
	$(call need-script,./scripts/db/migrate.sh)
	./scripts/db/migrate.sh create $(NAME)

.PHONY: db-seed
db-seed: ## Migrate up and apply the deterministic dev seed (DATABASE_URL required).
	$(call need-script,./scripts/db/seed.sh)
	./scripts/db/seed.sh

.PHONY: db-reset
db-reset: ## Drop the public schema, migrate up, and reseed (dev only, DATABASE_URL required).
	$(call need-script,./scripts/db/reset.sh)
	./scripts/db/reset.sh

.PHONY: dev-up
dev-up: ## Start the core dependency set (compose).
	$(call need-script,./scripts/dev/dev-up.sh)
	./scripts/dev/dev-up.sh

.PHONY: dev-down
dev-down: ## Stop the core dependency set.
	$(call need-script,./scripts/dev/dev-down.sh)
	./scripts/dev/dev-down.sh

.PHONY: dev-logs
dev-logs: ## Tail dependency service logs.
	$(call need-script,./scripts/dev/dev-logs.sh)
	./scripts/dev/dev-logs.sh

.PHONY: docker-build
docker-build: ## Build all service images (multi-stage Dockerfiles, E17).
	docker build -f deployments/docker/Dockerfile.rest-api -t go-template/rest-api:dev .

.PHONY: validate-config
validate-config: ## Validate config YAMLs against the loader + JSON schema (E01-T03).
	$(call need-script,./scripts/config/validate.sh)
	./scripts/config/validate.sh

.PHONY: clean
clean: ## Remove build output and caches (never touches tracked files).
	rm -rf $(BIN_DIR) coverage/ tmp/
