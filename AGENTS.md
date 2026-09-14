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

### Unit Test Structure and Scoping Policy

All unit tests across this repository must follow our strict table-driven pattern. Any AI model, subagent, or human engineer writing unit tests must adhere to these conventions:

#### Core Rules

1. **Locally Scoped `type testCase struct`**: Declare it inside the test function body, never at package scope.
2. **Exact Parameter & Return Field Names**: Field names in `testCase` must match the exact signature of the tested function (e.g. `amountMinor`, `bps`, `floorMinor`, `capMinor`, `expectedResult`, `expectedError`). Never use vague field names like `param`, `arg`, `input`, or `want`.
3. **No Single-Use Variables Declared Outside `testCases`**:
   - **Anti-pattern**: Declaring single-use variants (`noID`, `negAmount`, `brokenRate`, `lost`, `under`) before `testCases`.
   - **Mandatory pattern**: Inline all test-specific inputs directly inside each `testCase`. If deriving a variant from a base template, use an immediately-invoked anonymous closure:
     ```go
     {
         name: "zero amount",
         dispute: func() entity.Dispute {
             d := base
             d.AmountMinor = 0
             return d
         }(),
         expectedError: entity.NewError("INVALID_AMOUNT", "amount must be positive"),
     },
     ```
4. **Multi-Line Formatting**: Every test case entry in `testCases` must span multiple lines with standard indentation. Never write dense one-liners.
5. **Parallel Safety & Modern Loop Scoping**: Invoke `t.Parallel()` at the top of parent test functions. Do NOT include `tc := tc` (obsolete in Go 1.22+) and do NOT invoke `t.Parallel()` inside subtest loops (`t.Run`), keeping subtests clean, sequential, and deterministic.
6. **Edge-Case Rigor**: Thoroughly cover zero, negative, maximum boundary values (`math.MaxInt64`, arithmetic overflow), empty/whitespace strings, and illegal state transitions.

#### Complete Unit Test Examples

Below are complete, canonical examples covering different function signatures:

##### Example 1: Entity / Method on Struct Returning `error`
Tested signature: `func (b SettlementBatch) Validate() error`

```go
package entity_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
)

func TestSettlementBatchValidate(t *testing.T) {
	t.Parallel()

	baseBatch := entity.SettlementBatch{
		BatchID:         "b-1",
		ProviderBatchID: "pb-1",
		ProviderTraceID: "pt-1",
		AssetCode:       "USD",
		GrossMinor:      10000,
		FeeMinor:        290,
		NetMinor:        9710,
		CoverageStart:   "2026-09-01T00:00:00Z",
		CoverageEnd:     "2026-09-02T00:00:00Z",
		Items: []entity.SettlementItem{
			{PaymentID: "p-1", Status: entity.SettleItemSettled, AmountMinor: 5000},
			{PaymentID: "p-2", Status: entity.SettleItemPending, AmountMinor: 4710},
		},
	}

	type testCase struct {
		name          string
		batch         entity.SettlementBatch
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid batch",
			batch:         baseBatch,
			expectedError: nil,
		},
		{
			name: "missing batch id",
			batch: func() entity.SettlementBatch {
				b := baseBatch
				b.BatchID = ""
				return b
			}(),
			expectedError: entity.NewError("BATCH_ID_REQUIRED", "settlement batch requires batch and provider batch ids"),
		},
		{
			name: "negative gross",
			batch: func() entity.SettlementBatch {
				b := baseBatch
				b.GrossMinor = -1
				return b
			}(),
			expectedError: entity.NewError("INVALID_ENTRY_AMOUNT", "gross amount must be non-negative"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.batch.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
```

##### Example 2: Domain Function with Multiple Scalar Parameters and Multiple Returns
Tested signature: `func AssessTransactionFee(amountMinor, bps, floorMinor, capMinor int64) (int64, error)`

```go
package service_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
)

func TestAssessTransactionFee(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		amountMinor    int64
		bps            int64
		floorMinor     int64
		capMinor       int64
		expectedResult int64
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "within bounds",
			amountMinor:    10000,
			bps:            290,
			floorMinor:     30,
			capMinor:       500,
			expectedResult: int64(290),
			expectedError:  nil,
		},
		{
			name:           "floor applied",
			amountMinor:    100,
			bps:            290,
			floorMinor:     30,
			capMinor:       500,
			expectedResult: int64(30),
			expectedError:  nil,
		},
		{
			name:           "zero amount error",
			amountMinor:    0,
			bps:            290,
			floorMinor:     30,
			capMinor:       500,
			expectedResult: int64(0),
			expectedError:  &entity.Error{Code: "INVALID_FEE_AMOUNT", Message: "fee amount must be positive"},
		},
		{
			name:           "arithmetic overflow",
			amountMinor:    math.MaxInt64,
			bps:            2,
			floorMinor:     0,
			capMinor:       0,
			expectedResult: int64(0),
			expectedError:  &entity.Error{Code: "FEE_OVERFLOW", Message: "fee computation overflowed"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.AssessTransactionFee(tc.amountMinor, tc.bps, tc.floorMinor, tc.capMinor)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
```

##### Example 3: Context-Aware Function with Request and Response DTOs
Tested signature: `func (s *PaymentService) Authorize(ctx context.Context, req AuthorizeRequest) (AuthorizeResponse, error)`

```go
func TestPaymentServiceAuthorize(t *testing.T) {
	t.Parallel()

	baseReq := AuthorizeRequest{
		AccountID:   "acct-123",
		AmountMinor: 5000,
		Currency:    "USD",
	}

	type testCase struct {
		name           string
		ctx            context.Context
		req            AuthorizeRequest
		expectedResult AuthorizeResponse
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "successful authorization",
			ctx:  context.Background(),
			req:  baseReq,
			expectedResult: AuthorizeResponse{
				Status: "AUTHORIZED",
			},
			expectedError: nil,
		},
		{
			name: "canceled context",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			req:            baseReq,
			expectedResult: AuthorizeResponse{},
			expectedError:  context.Canceled,
		},
		{
			name: "invalid amount",
			ctx:  context.Background(),
			req: func() AuthorizeRequest {
				r := baseReq
				r.AmountMinor = -100
				return r
			}(),
			expectedResult: AuthorizeResponse{},
			expectedError:  entity.NewError("INVALID_AMOUNT", "amount must be positive"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewPaymentService()
			res, err := svc.Authorize(tc.ctx, tc.req)
			assert.Equal(t, tc.expectedResult, res)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
```

#### Canonical Reference Test Files

When implementing tests for different function signatures, inspect these reference files in the repository:

| Function Signature Pattern | Canonical Reference File | Tested Behaviors & Key Patterns |
| :--- | :--- | :--- |
| **Entity / Struct Validation** `(e Entity) Validate() error` | [`internal/domain/entity/settlement_batch_test.go`](internal/domain/entity/settlement_batch_test.go) | Base struct template, inline closures for field mutations, multi-line table |
| **Entity Validation with Slices & Enums** `(p PaymentLink) Validate() error` | [`internal/domain/entity/payment_link_test.go`](internal/domain/entity/payment_link_test.go) | Boundary strings, enum validity, slice empty/populated checks |
| **Pure Computation with Bounds** `Func(amount, bps, floor, cap) (result, error)` | [`internal/domain/service/fee_interest_test.go`](internal/domain/service/fee_interest_test.go) | Exact scalar parameter mapping, math overflow, inverted bounds |
| **Complex Domain Service with Maps** `ValidateRefund(req, exists, accts) (Lines, error)` | [`internal/domain/service/refund_service_test.go`](internal/domain/service/refund_service_test.go) | In-memory lookup maps, complex DTO structs, timestamp derivation |
| **Value Object Parser / Validator** `ValidateDescriptor(s, network) error` | [`internal/domain/valueobject/descriptor_test.go`](internal/domain/valueobject/descriptor_test.go) | String length limits, ASCII control characters, network-specific rules |
| **Value Object Decimal Math / Factory** `NewFXRate(base, quote, rate) (FXRate, error)` | [`internal/domain/valueobject/fx_rate_test.go`](internal/domain/valueobject/fx_rate_test.go) | String decimal precision parsing, invert calculation, zero rate check |
| **State Machine Transition Matrix** `CanTransitionPayment(from, to) bool` | [`internal/domain/valueobject/payment_status_test.go`](internal/domain/valueobject/payment_status_test.go) | Exhaustive state transition pairs, terminal state assertions |


### Mandatory Pre-Commit & Verification Protocol

No task claim may be released (`Status: released`) and no work may be considered complete without running and passing the full verification pipeline:

1. **Environment Setup (Pixi & CGO)**:
   - Tool binaries (`clang`, `golangci-lint`, `pixi`) reside in `/home/kadekutama/.pixi/bin`.
   - Ensure PATH is exported: `export PATH="$HOME/.pixi/bin:$PATH"`.
   - The Go race detector (`-race`) requires a C compiler and CGO: always export `export CGO_ENABLED=1 CC=clang`.
2. **Formatting**:
   - Run `gofmt -s -w .` and `goimports -l -w .`.
3. **Strict Linting**:
   - Run `make lint` (executes `golangci-lint run ./...` with strict rules: `gocyclo <= 12`, `goconst`, `unparam`, `revive`, etc., and `shellcheck`).
   - Must exit with **0 issues**.
4. **Race-Detector Test Suite**:
   - Run `CGO_ENABLED=1 CC=clang go test -v -race ./...` (or for domain: `CGO_ENABLED=1 CC=clang go test -v -race ./internal/domain/...`).
   - Must pass with **zero data races**.
5. **Gate Check**:
   - Run `./tasks/scripts/gate-check.sh <GATE>` (e.g., `G2` for domain layer).
   - Statement coverage must meet or exceed the gate threshold (Domain >= 90%).
6. **SDD Specification Validation**:
   - Run `python3 tasks/scripts/check-tasks.py --format --graph --sdd --specs --events --codes`.
   - Must report `OK: structural checks passed`.
7. **Durable Evidence**:
   - The terminal output and exit status of each command above must be documented in `tasks/evidence/<TASK-ID>.md`.

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
