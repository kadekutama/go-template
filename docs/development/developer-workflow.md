# Developer Workflow & Makefile Guide

This document is the definitive operational guide for software engineers and AI agents working on this repository. It provides a step-by-step walkthrough of the developer lifecycle: from initial machine bootstrapping, local dependency management, and database migrations, to day-to-day coding, formatting, linting, and local testing.

---

## 1. Overview & Tooling Architecture

This repository uses **Pixi** (`~/.pixi/bin`) to provide a hermetic, reproducible toolchain across developers and CI environments without polluting global system paths or external editor configurations (e.g. Zed, VS Code).

```
   ┌────────────────────────────────────────────────────────┐
   │                    Developer Machine                   │
   │  ┌──────────────────────────────────────────────────┐  │
   │  │   Pixi Isolated Environment (~/.pixi/bin)        │  │
   │  │   - Go 1.27.1 Toolchain                          │  │
   │  │   - Clang / Clang++ (CGO Compiler for -race)     │  │
   │  │   - GNU Make & Shellcheck                        │  │
   │  │   - Docker CLI & Docker Compose v2               │  │
   │  │   - gopls (Go Language Server)                   │  │
   │  │   - goimports, golangci-lint, mockery, etc.      │  │
   │  └──────────────────────────┬───────────────────────┘  │
   │                             │                          │
   │                             ▼                          │
   │  ┌──────────────────────────────────────────────────┐  │
   │  │   Makefile Commands (make <target>)              │  │
   │  │   - setup / deps / dev-up / migrate-up           │  │
   │  │   - fmt / fmt-check / gopls-check / lint         │  │
   │  │   - test-unit / test-race / test-integration     │  │
   │  │   - verify / build-all                           │  │
   │  └──────────────────────────────────────────────────┘  │
   └────────────────────────────────────────────────────────┘
```

---

## 2. Phase 1: Environment Setup & Toolchain Bootstrapping

Before writing code or running tests, you must initialize the local toolchains and dependencies.

### Step 1: Run the Bootstrap Script
Run the single setup command:
```bash
make setup
# Or directly: ./scripts/dev/setup.sh
```

**What this automatically does:**
1. Installs `pixi` (if not already present).
2. Installs core compilers and system binaries via Pixi (`go`, `clang`, `clang++`, `make`, `shellcheck`, `docker-cli`, `docker-compose`, `kubectl`, `k6`, `syft`, `gopls`).
3. Persistently configures CGO (`go env -w CGO_ENABLED=1 CC=clang`) so the Go race detector (`-race`) works out-of-the-box.
4. Downloads and cryptographically verifies Go modules (`go mod download && go mod verify`).
5. Installs Go CLI developer tools (`goimports`, `golangci-lint` v2.13.2, `govulncheck`, `gosec`, `go-licenses`, `mockery`, `buf`, `oapi-codegen`, `air`, `migrate`) and links them into `~/.pixi/bin`.

### Step 2: Configure Your Shell PATH
Ensure `~/.pixi/bin` is in your PATH. Add this line to your `~/.bashrc`, `~/.zshrc`, or local environment:
```bash
export PATH="$HOME/.pixi/bin:$PATH"
```

### Step 3: Inspect Status
Verify that all tools and toolchains report `[OK]`:
```bash
./scripts/dev/setup.sh --check
```

---

## 3. Phase 2: Local Services & Dependencies

The ledger system relies on several backend dependencies (PostgreSQL, Valkey, NATS JetStream, etc.).

### Start Local Dependencies
```bash
make dev-up
```
Starts the Docker Compose development stack in the background:
- **PostgreSQL 18.6**: Primary relational database.
- **Valkey 9.0.6**: L2 distributed cache and rate limiter (OSS Redis fork).
- **NATS JetStream 2.14.6**: Event broker and outbox publisher.
- **MailDev**: Local SMTP server for testing notification flows.

### Inspect Dependency Logs
```bash
make dev-logs
```
Tails the real-time logs of the running Docker containers.

### Stop Local Dependencies
```bash
make dev-down
```
Stops the Docker containers when development work is paused or finished.

---

## 4. Phase 3: Database Migrations

Schema migrations are managed with `golang-migrate` and live under `internal/infrastructure/database/migration/versions/`.

### Apply Migrations
```bash
make migrate-up
# Or via script: ./scripts/db/migrate.sh up
```
Applies all pending SQL migrations to the database specified by `DATABASE_URL`.

### Roll Back a Migration
```bash
make migrate-down
# Or via script: ./scripts/db/migrate.sh down 1
```
Rolls back the most recent migration step. All migrations must be strictly reversible.

### Create a New Migration
```bash
make migrate-create NAME=add_merchants_table
```
Generates a new timestamped migration pair:
- `internal/infrastructure/database/migration/versions/<timestamp>_add_merchants_table.up.sql`
- `internal/infrastructure/database/migration/versions/<timestamp>_add_merchants_table.down.sql`

### Seed Development Data
```bash
make db-seed
# Or via script: ./scripts/db/seed.sh
```
Runs pending migrations and applies the deterministic development plan (dev tenant `tnt-test-01`, dev ledger `ldg-test-01`, 9-account chart across USD/EUR/IDR, and 4 canonical double-entry postings). Strictly idempotent via `ON CONFLICT DO NOTHING`.

### Reset Database (Dev Only)
```bash
make db-reset
# Or via script: ./scripts/db/reset.sh
```
Drops the `public` schema, re-applies all migrations from version 1, and seeds dev data. Includes production safeguards (refuses to run against URLs containing `prod`, `amazonaws.com`, or `cloudsql` without `--force`).

### Architecture Note: Why Migrations & Seeding Are Decoupled from `main.go`
In this repository, database migrations and seeding are deliberately decoupled from application server startup binaries (`cmd/rest`, `cmd/grpc`, `cmd/cron`, etc.):
1. **Multi-Pod Concurrency**: Prevents schema lock contention, readiness probe timeouts, and `ACCESS EXCLUSIVE` table lock queuing when multiple pods autoscale concurrently.
2. **Least Privilege Security**: Runtime pods run as `app_user` (DML only: `SELECT`, `INSERT`, `UPDATE`). Administrative DDL credentials (`CREATE TABLE`, `ALTER TABLE`) are strictly restricted to isolated migration jobs in CI/CD.
3. **Accidental Seeding Protection**: Keeps dev seed logic completely out of production server binaries.
4. **Zero-Downtime Deployments**: Enables the Expand/Contract rollout pattern during blue-green and canary releases.

For deep architectural rationale on persistence, the transactional outbox pattern, and concurrency control, see [docs/infrastructure/README.md](../infrastructure/README.md).

---

## 5. Phase 4: Code Generation & Artifacts

Whenever you modify API schemas or internal port interfaces:

### Regenerate All Code Artifacts
```bash
make generate
```
Executes code generation pipelines:
- `scripts/generate/mocks.sh`: Regenerates `testify/mock` implementations using `mockery`.
- `scripts/generate/proto.sh`: Compiles Protobuf definitions using `buf`.
- `scripts/generate/gqlgen.sh`: Generates GraphQL resolvers using `gqlgen`.
- `scripts/generate/openapi.sh`: Generates Echo REST server DTOs from OpenAPI specs using `oapi-codegen`.

### Regenerate Mocks Only
```bash
make generate-mocks
```
Regenerates mocks for ports defined in `internal/domain/repository` and `internal/application/port`.

---

## 6. Phase 5: Day-to-Day Coding & Formatting

### Why We Use BOTH `gofmt` and `goimports`

A common question is: *Why do we need both `gofmt` and `goimports`?*

| Tool | Primary Capability | What It Lacks |
|---|---|---|
| **`gofmt -s`** | Official syntax formatter; performs **AST code simplification** (e.g. simplifying composite literals, slice bounds). | Does **NOT** manage imports (cannot add missing packages, remove unused ones, or group internal vs external packages). |
| **`goimports -local <module>`** | Automatically discovers and adds missing imports; removes unused imports; groups imports into 3 distinct sections (stdlib, 3rd-party, local project). | Does **NOT** perform `gofmt -s` AST code simplification. |

Because neither tool completely replaces the other, our repository combines them into a single, unified formatting pipeline:

### Format Code
```bash
make fmt
```
Executes:
1. `gofmt -s -w .` (simplifies code and applies standard indentation).
2. `goimports -local github.com/kadekutama/go-template -w .` (organizes, sorts, and groups imports cleanly).

### Check Formatting (Without Modifying)
```bash
make fmt-check
```
Used in CI and pre-commit checks. Returns exit code `0` if all files match formatting standards, or prints failing filenames and exits with code `1`.

### Build Verification
```bash
make build
```
Compiles all Go packages across the repository without emitting output binaries. Catches compile-time syntax errors quickly.

### Build Application Binaries
```bash
make build-all
```
Builds the 5 release binaries into `bin/`:
- `bin/rest-api`
- `bin/grpc-api`
- `bin/graphql-api`
- `bin/consumer`
- `bin/cron`

---

## 7. Phase 6: Static Analysis & Strict Linting

We enforce zero-warning code hygiene.

### Run Strict Linter
```bash
make lint
```
Executes the comprehensive 4-stage quality gate:
1. **Formatting Check (`fmt-check`)**: Verifies code simplification (`gofmt -s`) and import grouping (`goimports`).
2. **Compiler Diagnostics (`gopls-check`)**: Runs `gopls check` across all Go source files to detect typecheck failures, unexported field violations, and signature mismatches.
3. **Strict Static Analysis (`golangci-lint`)**: Runs `golangci-lint` with strict linters enabled:
   - `gocyclo <= 12` (cyclomatic complexity)
   - `errcheck` (unchecked errors)
   - `gosec` (security vulnerabilities)
   - `govet`, `staticcheck`, `unused`, `goconst`, `unparam`, `bodyclose`, `noctx`.
4. **Shell Script Analysis (`shellcheck`)**: Lints all bash scripts under `scripts/`.

### Run Go Vet
```bash
make vet
```
Runs standard Go compiler vet diagnostics.

---

## 8. Phase 7: The Testing Pyramid

Testing in this repository follows the strict test pyramid.

```
                  / \
                 /   \
                / E2E \          Few, slow (Chaos / k6)
               /-------\
              / Contract\        Pact contracts (make test-contract)
             /-----------\
            / Integration \      Real Testcontainers (make test-integration)
           /---------------\
          /    Unit Tests   \    Many, fast, in-memory (make test-unit / test-race)
         /-------------------\
```

### 1. Fast Unit Tests
```bash
make test-unit
```
Runs pure in-memory tests across `internal/domain`, `internal/application`, and `pkg/jsonparser`. Takes less than 2 seconds.

### 2. Race-Detector Test Suite (Mandatory Before PR)
```bash
make test-race
```
Runs the full test suite with the Go race detector enabled:
```bash
CGO_ENABLED=1 CC=clang go test -v -race ./...
```
*Note: The race detector requires CGO and a C compiler (`clang`). This is configured by `make setup`.*

### 3. Integration Tests
```bash
make test-integration
```
Runs integration tests against real containerized services using **Testcontainers** (requires Docker running).

### 4. Consumer-Driven Contract Tests
```bash
make test-contract
```
Runs Pact contract verification for external API boundaries.

### 5. Full Test Suite
```bash
make test-all
```
Runs all unit, integration, and contract tests sequentially.

---

## 9. Phase 8: Pre-Commit & Gate Verification

Before committing changes or releasing an SDD task claim:

### Single-Command Pre-Commit Verification
```bash
make verify
```
Runs:
1. `make fmt` (formats all code)
2. `make lint` (`fmt-check` + `gopls-check` + `golangci-lint` + `shellcheck`)
3. `make test-race` (zero data races across all packages)
4. SDD structural task check (`check-tasks.py`)

### Quality Gate Checks (Coverage Thresholds)
```bash
# Domain Layer Gate (Threshold: >= 90% statement coverage)
./tasks/scripts/gate-check.sh G2

# Application Layer Gate (Threshold: >= 85% statement coverage)
./tasks/scripts/gate-check.sh G3
```

### SDD Specification Validation
```bash
python3 tasks/scripts/check-tasks.py --format --graph --sdd --specs --events --codes --handlers --ports
```
Ensures task claims, packets, events, error codes, and handlers remain consistent with the architecture specification.

---

## 10. Complete Makefile Command Reference

| Command | Category | Purpose | Prerequisites |
| :--- | :--- | :--- | :--- |
| `make help` | Discovery | Lists all available targets with descriptions | None |
| `make setup` | Setup | Bootstraps Pixi, compilers, Go tools, and modules | `curl` or `wget` |
| `make deps` | Setup | Re-downloads modules and updates CLI dev tools | Pixi installed |
| `make dev-up` | Local Infra | Starts PostgreSQL, Valkey, NATS, and MailDev | Docker running |
| `make dev-down` | Local Infra | Stops local Docker dependency containers | Docker running |
| `make dev-logs` | Local Infra | Tails logs from local dependency containers | Docker running |
| `make migrate-up` | Database | Applies all pending migrations | `dev-up` / Postgres |
| `make migrate-down` | Database | Rolls back one migration step | `dev-up` / Postgres |
| `make migrate-create NAME=x`| Database | Generates a new migration SQL pair | Pixi / migrate CLI |
| `make generate` | Scaffolding | Regenerates mocks, protobuf, GraphQL, and OpenAPI | Dev tools installed |
| `make generate-mocks` | Scaffolding | Regenerates testify mocks for domain/application | `mockery` installed |
| `make fmt` | Hygiene | Formats code with `gofmt -s` and `goimports` | `goimports` on PATH |
| `make fmt-check` | Hygiene | Verifies formatting without modifying files | `goimports` on PATH |
| `make gopls-check` | Hygiene | Runs compiler diagnostics via `gopls check` | `gopls` on PATH |
| `make lint` | Hygiene | Strict lint (`fmt-check` + `gopls` + `golangci-lint` + `shellcheck`) | Linters on PATH |
| `make vet` | Hygiene | Runs standard `go vet ./...` | Go installed |
| `make build` | Build | Compiles all packages (checks build errors) | Go installed |
| `make build-all` | Build | Compiles 5 binaries into `bin/` | Go installed |
| `make test` | Testing | Alias for `make test-unit` | Go installed |
| `make test-unit` | Testing | Runs fast unit tests (domain + application) | Go installed |
| `make test-race` | Testing | Runs full test suite with `-race` detector | `CGO_ENABLED=1 CC=clang` |
| `make test-integration` | Testing | Runs tests against real Testcontainers | Docker running |
| `make test-contract` | Testing | Runs Pact contract tests | Docker / Pact |
| `make test-all` | Testing | Runs unit + integration + contract tests | Docker running |
| `make verify` | CI / Pre-Commit | Full verification (`fmt` + `lint` + `test-race` + `sdd`) | All tools ready |
| `make clean` | Cleanup | Deletes `bin/`, `coverage/`, and temporary files | None |

---

## 11. Troubleshooting & FAQs

### Q: Why does `make test-race` fail with `cgo: C compiler "cc" not found`?
**A:** The race detector requires a C compiler (`clang`). Run `make setup` to install `clang` via Pixi, and ensure `export PATH="$HOME/.pixi/bin:$PATH"` is loaded in your shell. You can also run:
```bash
go env -w CGO_ENABLED=1 CC=clang
```

### Q: Why does `make lint` report unformatted files or bad imports?
**A:** Run `make fmt` before running `make lint`. `make fmt` automatically runs both `gofmt -s` (simplifying syntax) and `goimports -local github.com/kadekutama/go-template` (organizing imports).

### Q: Does running `make setup` interfere with my system or other editors (e.g. Zed)?
**A:** No. Pixi installs packages exclusively under `~/.pixi/` and links developer binaries into `~/.pixi/bin`. It does not modify system packages, `/usr/local`, or external editor configurations.

### Q: How do I run Docker from Pixi (for Testcontainers suites)?
**A:** Pixi provides the Docker **client only** (`docker-cli`, `docker-compose` → `~/.pixi/bin/docker`, `~/.pixi/bin/docker-compose`). The **daemon** comes from the host. Every shell (including agent/non-interactive shells) needs:
```bash
export PATH="$HOME/.pixi/bin:$PATH"
docker info   # must show a Server section; client-only output means no daemon
```
If the daemon is unreachable, start one engine first: Rancher Desktop / Docker Desktop with WSL integration enabled, native `sudo systemctl start docker` (or `sudo dockerd`), or point at a remote engine via `export DOCKER_HOST=tcp://<host>:2375`. `./scripts/dev/setup.sh --check` reports client binaries and daemon reachability separately, and a failed `docker-cli`/`docker-compose` install warns instead of aborting the rest of the toolchain setup.

### Q: Why do Testcontainers tests skip (or once panicked with no daemon)?
**A:** Container-gated suites skip cleanly without a daemon via `test/testcontainers.SkipIfNoDocker` (probes `docker info`, honors `TESTCONTAINERS_SKIP=1`). Two past pitfalls are fixed in-tree: the helper no longer lets `testcontainers-go` panic on daemon detection, and PostgreSQL readiness uses SQL-level `wait.ForSQL` (TCP-accept fires while Postgres is still starting, `57P03`). Profiles: `TESTCONTAINERS_PROFILE=ci` shortens startup bounds for runners. Without a daemon, `go test -race ./...` stays green; with one, `make test-integration` proves the live paths.
