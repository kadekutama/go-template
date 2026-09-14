# AGENTS.md - Harness-Neutral Agent Instructions

This file provides instructions for AI agents working on this Go Clean Architecture template project.

## Project Context
- **Project**: Go Clean Architecture Template (SDD + DDD)
- **Location**: `/home/kadekutama/workspace/oc-free-models-test/go-template`
- **Specification entry point**: `SPEC.md`; task-specific authority is defined by `tasks/SDD.md`
- **Target**: Production-ready template for Fintech Ledger, SaaS, Event-Driven systems

## Mandatory Delivery Protocol

Every harness must follow `tasks/SDD.md`. Before implementation, the assigned
task needs a ready packet under `tasks/specs/` and an active claim. Evidence and
handoff files are durable state; conversation history is not. Harness-specific
profiles may add operating instructions but cannot override the task packet,
accepted ADRs, or `docs/ledger-core.md`.

**Multi-Harness Governance & Collision Avoidance:**
- **Shared filesystem is NEVER permission to edit:** Access to the local workspace
  or repository clone does not grant permission to modify files without an active
  claim in `tasks/claims/<TASK-ID>.md` (`Status: active`) that matches the agent's
  harness identifier.
- **No out-of-band edits:** When all task claims in an epic/branch are released,
  the working tree is frozen. Post-release polish or review fixes require either
  reopening the claim (transitioning to `active` with amendment rationale) or
  recording a formal takeover claim under `tasks/SDD.md §4`.
- **Parallel Work Isolation:** For parallel tasks (e.g. Phase 3 / E03+), harnesses
  MUST operate in separate Git worktrees (`git worktree add ../go-template-<TASK-ID>`)
  and must not declare overlapping change surfaces.

The phrase **Specification-Driven Delivery** refers to this repository workflow.
The **Specification pattern** refers only to executable domain business rules.

## Agent Roles & Responsibilities

### 1. Domain Expert (`domain-expert`)
- **Focus**: `internal/domain/` - Pure domain logic, no external dependencies
- **Tasks**: Entities, Value Objects, Aggregates, Domain Events, domain Specifications, Domain Services
- **Rules**: 
  - Zero external dependencies (stdlib only)
  - All business rules as executable Specifications
  - Domain Events record facts; application handlers dispatch side effects
  - Aggregate roots enforce invariants

### 2. API Designer (`api-designer`)
- **Focus**: `api/`, `cmd/*/`, `internal/interface/{rest,grpc}/`, `pkg/httpserver/`, `pkg/grpcserver/`, `pkg/graphql/`
- **Tasks**: REST (Echo), gRPC, GraphQL (gqlgen), WebSocket, API versioning, OpenAPI docs
- **Rules**:
  - Schema-first (Protobuf, GraphQL schemas)
  - Request/Response DTOs in application layer
  - Standardized error envelope
  - Correlation ID propagation

### 3. Database Engineer (`db-engineer`)
- **Focus**: `internal/infrastructure/database/`, `internal/infrastructure/cache/`
- **Tasks**: PostgreSQL (GORM), Migrations (golang-migrate), Valkey, Ristretto, Hybrid Cache
- **Rules**:
  - Repository interfaces in domain, implementations in infrastructure
  - Migrations embedded, Up/Down reversible
  - Connection pooling configured
  - Cache-aside pattern with L1/L2 (Ristretto → Valkey)

### 4. DevOps Engineer (`devops-engineer`)
- **Focus**: `deployments/`, `.github/workflows/`, `scripts/`, Docker, K8s
- **Tasks**: Docker Compose (all variants), CI/CD, Traefik, Observability Stack, GitOps
- **Rules**:
  - All dependencies dockerized for testability
  - Multi-stage Dockerfiles
  - Testcontainers for integration tests
  - SBOM, security scanning in CI

### 5. Test Engineer (`test-engineer`)
- **Focus**: `test/`, `scripts/test/`
- **Tasks**: Unit, Integration (Testcontainers), Performance (k6), Contract (Pact), Chaos
- **Rules**:
  - Test pyramid: many unit, some integration, few E2E
  - Domain: pure function tests
  - Application: mock ports (mockery)
  - Infrastructure: real containers
  - 100% dockerized test dependencies

## General Rules for All Agents

### Normative Ledger Read Order

Before any fintech task, read `docs/ledger-core.md`, the assigned epic, and only
then its linked narrative docs. `docs/ledger-core.md` overrides older examples
for accounting semantics, idempotency, concurrency, balances, and event delivery.
The repository owner approved that precedence and the audited ADR-002/003/009
decisions on 2026-09-13; their permanent records live under
`docs/architecture/`. ADR-011 remains proposed until its benchmark evidence is
reviewed and accepted.
The audit and open risks are recorded in `docs/repository-audit.md`.

### Epic Ownership Coverage

- Domain Expert: E02–E05
- API Designer: E06 and E11–E14
- Database Engineer: E07–E10
- DevOps Engineer: E00–E01, E15, E17, E19
- Test Engineer: E16 and verification support in every epic
- E18 documentation is cross-functional; the coordinating agent owns the index
  and each specialist owns docs/ADRs for its changes.

### Code Standards
- **Go Version**: 1.27.1 (current stable patch at the audit date; pin in `go.mod` and CI)
- **Linting**: `golangci-lint` strict mode (`.golangci.yml`)
- **Formatting**: `gofmt` / `goimports`
- **Dependencies**: Reproducible target pins in `SPEC.md`; verify/update through Renovate/Dependabot and evidence
- **Go idioms**: Follow [`docs/development/go-conventions.md`](docs/development/go-conventions.md); new structured logging uses `log/slog`
- **JSON**: import the repository's `pkg/jsonparser` wrapper; it is
  encoding/json-compatible and may use Sonic only when benchmark evidence
  justifies it (never import jsoniter or a codec directly from domain code)
- **Cache**: Valkey 9.0.6 (OSS fork of Redis) instead of Redis
- **SMTP Testing**: maildev instead of MailHog

### Architecture Compliance
- **Dependency Direction**: Interface → Application → Domain ← Infrastructure
- **No circular imports** between layers
- **Domain layer**: Zero external imports
- **fx Modules**: Each layer registers its own fx.Option

### Testing Requirements
- Unit tests for all business logic
- Integration tests for all infrastructure adapters
- Contract tests for API boundaries
- Run: `make test-all` before committing

### Documentation
- Update relevant `.md` files in `docs/` with changes
- Record ADRs in `docs/architecture/` for architectural decisions
- Keep `SPEC.md` synchronized with implementation

### Git Workflow
- Feature branches from `main`
- Conventional commits: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`
- PR required for all changes
- CI must pass before merge

## Phase Tracking
Current Phase: **Phase 2 Complete / Phase 3 Start** (E00, E01, E02 completed; next: `tasks/epics/E03-money-movement.md`, `tasks/epics/E04-compliance.md`, `tasks/epics/E05-tenancy.md`)

Agents should:
1. Follow `tasks/SDD.md` and select one dependency-ready task (`python3 tasks/scripts/check-tasks.py --ready`).
2. Read its approved task packet and exact normative references.
3. Verify that no other harness is claiming the task, and create an active claim in `tasks/claims/<TASK-ID>.md` before touching code.
4. If working in parallel with another harness, isolate the workspace using a dedicated Git worktree.
5. Implement and record requirement-level evidence in `tasks/evidence/<TASK-ID>.md`.
6. Maintain a repository-visible handoff in `tasks/handoffs/<TASK-ID>.md` so another harness can resume.
7. Create an ADR for architectural deviations before implementing them.

## Key Files to Reference
- `SPEC.md` - Master specification (source of truth)
- `docs/development/go-conventions.md` - Go/SOLID/CQRS/ledger coding contract
- `tasks/SDD-INTEROP.md` - OpenSpec/Spec Kit interoperability mapping
- `Makefile` - Build/test/deploy commands
- `.golangci.yml` - Linting rules
- `config/schemas/config.json` - Config validation schema
- `deployments/docker/*.yml` - Dependency definitions

## Communication
- Agents coordinate through task claims, packets, handoffs, evidence, and Git/PR state.
- Questions that change behavior remain in the packet; it cannot be `ready` until resolved.
- Breaking or cross-cutting architectural changes require an ADR.
