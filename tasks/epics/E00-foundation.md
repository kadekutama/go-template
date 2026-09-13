# Epic E00: Repository Bootstrap & Tooling

**Status:** pending
**Story Points:** 14
**Phase:** 0
**Dependencies:** — (none; this epic is first)
**SDD Gate:** G1
**Design refs:** `SPEC.md §4` (directory structure), `SPEC.md §2` (Go 1.27.1 target pin), `SPEC.md §11.1` (pipeline stages), `SPEC.md §12` (compose variants), `tasks/SDD-INTEROP.md`

> Why this epic is first: every later task assumes the folder layout, the Makefile
> targets, and a minimal CI gate (lint + build). Without it, agents invent
> divergent layouts and unreviewed code lands without checks.

## Tasks

### E00-T00: Establish repository boundary and shared Git baseline
**Status:** completed
**Background:** The repository owner approved `go-template/` as a standalone
repository with `main` as its default branch on 2026-09-13 and supplied the
shared remote `https://github.com/kadekutama/go-template`. Baseline commits
`6eca386`/`b51d5c9` exist and origin is configured; remaining completion steps
are push authentication plus the remote-`main` reconciliation decision (the two
histories share no merge base).
**Files:**
- Create/maintain after the boundary decision: `.github/CODEOWNERS`,
  `docs/development/repository-governance.md`; configure Git metadata outside
  tracked files without importing unrelated sibling content
**Steps:**
1. Record the owner-approved standalone `go-template` boundary and `main`
   default branch; agents must not create nested `.git` metadata or absorb
   sibling projects.
2. Create a clean initial baseline commit containing the audited planning/SDD
   artifacts and configure the shared `origin`/default branch used for claims.
3. Document branch/worktree naming, PR/merge serialization, CODEOWNERS, and how
   claim conflicts are resolved across harnesses.
4. Verify a fresh clone/worktree reproduces the same files and validator result.
**Acceptance Criteria:**
- [ ] `git rev-parse --show-toplevel`, `git rev-parse --verify HEAD`, and
  `git remote get-url origin` return the owner-approved boundary, commit, and remote.
- [ ] `go-template` planning files are tracked and unrelated sibling projects are excluded.
- [ ] A fresh checkout passes `python3 tasks/scripts/check-tasks.py --format --graph --sdd`.
- [ ] CODEOWNERS/governance define who may merge claims, migrations, generated contracts, and ADRs.
**Story Points:** 2
**Depends On:** —
**Related Docs:** `tasks/SDD.md §3, §6`, `AGENTS.md`
**SDD Gate:** G1

---

### E00-T01: Initialize Go module and folder structure
**Status:** completed
**Background:** Create the module and the exact tree from `SPEC.md §4` so all
later epics have stable import paths (`internal/...`, `pkg/...`, `cmd/...`).
Without this, every agent invents its own layout and imports break.
**Files:**
- Create: `go.mod`, `.gitignore`, five minimal `cmd/*/main.go` stubs; `go.sum`
  appears only when a dependency requires it
- Create dirs: `cmd/{rest-api,grpc-api,graphql-api,cron,consumer}`, `internal/{domain,application,interface/{rest,grpc,cron,consumer},infrastructure,shared}`, `pkg/{httpserver,grpcserver,graphql,scheduler,jsonparser}`, `api/{proto,openapi,graphql}`, `config/`, `deployments/{docker,k8s}`, `docs/{architecture,domain,application,infrastructure,api,development}`, `scripts/{build,dev,test,db,generate,release,security}`, `test/{unit,integration,performance,chaos,contract,fixtures,mock,testcontainers}`, `.github/workflows/`, `.opencode/{agents,skills,permissions}/`
**Steps:**
1. `go mod init example.com/go-template` with `go 1.27.1` directive. The reserved
   `example.com` path is the canonical reference-template identity; the E18
   scaffolder replaces it for consumers. Do not invent an organization path.
2. Create the directory tree above (empty `.gitkeep` where needed).
3. Write `.gitignore` covering `bin/`, `coverage/`, `*.log`, `.env*`, and
   `.air.toml` local overrides while explicitly unignoring committed
   `.env.example`/`.env.*.example` templates.
4. Add deterministic no-op mains for all five binaries; run `go mod download`
   and `go build ./...`.
**Acceptance Criteria:**
- [ ] `go build ./...` succeeds on the empty tree.
- [ ] Tree matches `SPEC.md §4` (verify with `tasks/scripts/check-tasks.py --dirs`).
- [ ] `.gitignore` covers `bin/`, `coverage/`, `*.log`, and local `.env*` files
  while keeping committed `.env.example` templates trackable.
**Story Points:** 1
**Depends On:** E00-T00
**Related Docs:** `SPEC.md §4`, `SPEC.md §2`
**SDD Gate:** G1

---

### E00-T02: Makefile with standard targets
**Status:** completed
**Background:** One entry point for build/test/lint/dev workflows used by every
later epic and by CI. Standardizes what "done" means for verification steps.
**Files:**
- Create: `Makefile`
**Steps:**
1. Add targets: `build`, `build-all` (5 binaries → `bin/`), `test`, `test-unit`,
   `test-integration`, `test-contract`, `test-all`, `lint`, `fmt`, `vet`,
   `generate`, `migrate-up`, `migrate-down`, `migrate-create`, `dev-up`,
   `dev-down`, `dev-logs`, `docker-build`, `clean`, `help`.
2. `make help` must list every target with a one-line description.
**Acceptance Criteria:**
- [ ] `make help` lists all targets above.
- [ ] `make build-all` produces 5 binaries in `bin/` (stub mains acceptable at this stage).
- [ ] `make lint`, `make fmt`, `make vet` run without error on the empty tree.
**Story Points:** 1
**Depends On:** E00-T01
**Related Docs:** `SPEC.md §15`, `SPEC.md §12`
**SDD Gate:** G1

---

### E00-T03: Strict golangci-lint configuration
**Status:** completed
**Background:** Code-quality gate enforced in CI from day one (`SPEC.md §16`).
Catches `errcheck`, `gosec`, races in config, and style drift before they spread;
the implementation conventions are recorded in `docs/development/go-conventions.md`.
**Files:**
- Create: `.golangci.yml`
**Steps:**
1. Enable at minimum: `errcheck`, `govet`, `staticcheck`, `gosec`, `bodyclose`,
   `noctx`, `rowserrcheck`, `sqlclosecheck`, `goconst`, `gocritic`, `gocyclo`,
   `gofmt`, `goimports`, `ineffassign`, `misspell`, `unconvert`, `unparam`, `unused`.
2. Set severity to error; configure `gocyclo min-complexity: 12`.
3. Run `golangci-lint run ./...` on the empty tree.
**Acceptance Criteria:**
- [ ] `golangci-lint run ./...` passes on the empty tree.
- [ ] Config file is referenced by `make lint` and `.github/workflows/ci.yml` (E00-T05).
**Story Points:** 1
**Depends On:** E00-T01
**Related Docs:** `SPEC.md §2`, `SPEC.md §16`
**SDD Gate:** G1

---

### E00-T04: Hot-reload and local dev scripts
**Status:** pending
**Background:** Developer velocity for `make dev-up` loops used from E01 onward.
**Files:**
- Create: `.air.toml`, `scripts/dev/dev-up.sh`, `scripts/dev/dev-down.sh`, `scripts/dev/dev-logs.sh`
**Steps:**
1. Configure air: watch `**/*.go`, `config/*.yaml`; exclude `bin/`, `vendor/`, `test/`, `docs/`, `.git/`; build `./cmd/rest-api`; 500ms delay.
2. `dev-up.sh` starts the core compose set; `dev-down.sh` stops it; `dev-logs.sh` tails service logs.
**Acceptance Criteria:**
- [ ] Editing a `cmd/rest-api` stub rebuilds and restarts via air.
- [ ] `scripts/dev/*.sh` are executable and wired into Makefile targets.
**Story Points:** 1
**Depends On:** E00-T01, E00-T02
**Related Docs:** `SPEC.md §15`, `README.md` (Quick Start)
**SDD Gate:** G1

---

### E00-T05: Minimal CI gate (lint + build) + dependency automation
**Status:** pending
**Background:** The full pipeline comes in E17, but lint+build must gate PRs
from the first code change. Dependency automation keeps the reproducible target
pins current and reviewable from day one.
**Files:**
- Create: `.github/workflows/ci.yml` (lint + build jobs only — full matrix in E17),
  `renovate.json`, `.github/dependabot.yml`
**Steps:**
1. `ci.yml`: on PR, run `go mod verify`, `go mod tidy -diff`, `make lint`,
   then `make build-all`. Fail-fast.
2. `renovate.json`: weekly schedule, group non-major updates, no auto-merge yet.
3. `dependabot.yml`: weekly Go-modu updates.
4. Document in `ci.yml` header that E17 extends this file (do not create a second pipeline).
**Acceptance Criteria:**
- [ ] Opening a test PR triggers lint+build and reports status.
- [ ] CI fails on unverified or non-tidy module files (`go mod verify` /
  `go mod tidy -diff`).
- [ ] Renovate + Dependabot configs validate (`renovate-config-validator`, dependabot schema).
- [ ] No duplicate pipeline files; E17 will extend this one.
**Story Points:** 2
**Depends On:** E00-T02, E00-T03
**Related Docs:** `SPEC.md §11.1`, `SPEC.md §2`
**SDD Gate:** G1

---

### E00-T06: Seed the docs skeleton for later epics
**Status:** pending
**Background:** E18 fills content, but directories + index files must exist now
so code tasks can link ADRs and layer docs as they go.
**Files:**
- Create: `docs/architecture/README.md` (ADR index table, empty),
  `docs/{domain,application,infrastructure,api,development}/README.md` (one-line scope each)
**Steps:**
1. Write the ADR index with columns `ID | Title | Status` and a "how to add an ADR" note.
2. Each layer README states its scope in one paragraph (copy from `SPEC.md §3`).
**Acceptance Criteria:**
- [ ] All six README files exist and link back to `SPEC.md §3`.
- [ ] `tasks/scripts/check-tasks.py --dirs` passes.
**Story Points:** 1
**Depends On:** E00-T01
**Related Docs:** `SPEC.md §3`, `SPEC.md §4`
**SDD Gate:** G1

---

### E00-T07: scripts/ automation package (build, test, generate, release, security)
**Status:** pending
**Background:** The Makefile (E00-T02) references scripts that must actually
exist; several epics assume them (db scripts in E07-T05, openapi in E11-T08,
mocks in E06-T06). Centralizing here prevents each epic inventing its own.
**Files:**
- Create: `scripts/build/build-all.sh`, `scripts/test/{unit,integration,contract,performance,chaos}.sh`,
  `scripts/generate/{mocks,gqlgen,proto,openapi}.sh`, `scripts/release/{changelog,tag,docker-push}.sh`,
  `scripts/security/{scan,sbom,licenses}.sh` (db/ + dev/ already covered by E07-T05 and E00-T04)
**Steps:**
1. Each script: `set -euo pipefail`, usage text, passthrough args; no hardcoded paths (repo-root relative).
2. Wire every script into the matching Makefile target from E00-T02.
3. `shellcheck` clean on all scripts (add to `make lint`).
**Acceptance Criteria:**
- [ ] Every Makefile target that references a script resolves to an existing executable file (test).
- [ ] `shellcheck scripts/**/*.sh` passes.
**Story Points:** 2
**Depends On:** E00-T02
**Related Docs:** `SPEC.md §11`, `SPEC.md §15`
**SDD Gate:** G1

---

### E00-T08: Enforce the harness-neutral SDD control plane
**Status:** pending
**Background:** Multiple AI harnesses must be able to specify, claim, verify,
hand off, and take over work using repository state alone. Chat transcripts and
harness-specific memory are not portable coordination mechanisms.
**Files:**
- Maintain: `tasks/SDD.md`, `tasks/{specs,claims,evidence,handoffs}/_TEMPLATE.md`,
  `tasks/scripts/check-tasks.py`, `AGENTS.md`, harness-specific agent adapters
**Steps:**
1. Keep the task packet, claim, evidence, completion, and takeover protocols
   harness-neutral; harness profiles only point to the normative protocol.
2. Make `check-tasks.py --graph --sdd` reject dependency cycles, active work
   without ready packets/claims, progress drift, and completion without evidence/handoff.
3. Wire `--format --graph --sdd` into the minimal CI workflow from E00-T05.
4. Create a fixture task in a temporary test directory proving valid and invalid
   lifecycle states without changing the real progress dashboard.
**Acceptance Criteria:**
- [ ] A fresh agent can identify the first runnable task and required context
  using only repository files.
- [ ] Validator tests cover a dependency cycle, missing packet, duplicate/missing
  claim, progress mismatch, and completed task without evidence.
- [ ] `python3 tasks/scripts/check-tasks.py --format --graph --sdd` passes.
- [ ] Every harness-specific instruction links to `tasks/SDD.md` and does not
  redefine status, evidence, or takeover rules.
**Story Points:** 3
**Depends On:** E00-T00, E00-T01, E00-T02, E00-T05
**Related Docs:** `tasks/SDD.md`, `AGENTS.md`, `tasks/README.md`
**SDD Gate:** G1

## Acceptance Criteria

- [ ] E00-T00 … E00-T08 all `completed` (count 14 SP in `tasks/tracking/PROGRESS.md`)
- [ ] `make build-all` produces 5 binaries; `make lint` is clean
- [ ] CI lint+build is green on a test PR
- [ ] SDD gate G1 checks pass — `tasks/tracking/GATES.md#G1`
