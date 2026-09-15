# Go Implementation Conventions

**Status:** Normative implementation guidance  
**Baseline:** Go 1.27.1, pinned by `go.mod` and the CI toolchain

This document turns the architectural intent in `SPEC.md` into rules an agent
can check while implementing. It supplements, but does not replace, the Go
language specification and the task packet.

## Layer and dependency rules

- `internal/domain/` imports the standard library only. Domain code does not
  know about SQL, HTTP, JSON codecs, logging, configuration, clocks, brokers,
  or provider SDKs.
- `internal/application/` imports domain packages and its own small ports. It
  does not import infrastructure or protocol packages.
- Cross-cutting ports (clock, logger, tracer, meter, secrets, flags) belong to
  an inner/shared or application package; infrastructure adapts SDKs to them.
- `internal/infrastructure/` implements ports and owns SDK/database details.
- `cmd/`, `pkg/`, and protocol adapters translate input/output and compose
  dependencies; they do not contain accounting decisions.
- The composition root is the only place that selects concrete adapters.
  Avoid package-level mutable dependencies and `init` side effects.
- Define interfaces at the consuming boundary and keep them small. Do not add
  a method to a shared interface merely because one adapter happens to have it.
  Implementations use descriptive names (`postgresAccountRepository`), not an
  `Impl` suffix.
- A port must preserve substitutability: adapters must not add provider-specific
  behavior, hidden retries, weaker consistency, or different error meanings.

## SOLID in this codebase

SOLID is a design test, not a reason to create an interface for every struct.

| Principle | Repository test |
|---|---|
| Single responsibility | Aggregates enforce invariants; application handlers orchestrate; adapters translate/persist; protocol code maps DTOs. |
| Open/closed | Add a posting template, query projection, or provider adapter through a registry/port; do not edit a switch spread through the ledger core. |
| Liskov substitution | A fake, primary, replica, or provider adapter honors the same timeout, idempotency, consistency, and error contract as the port. |
| Interface segregation | Consumers depend on narrow read/write/authorisation ports; an entry reader cannot write entries and a cache cannot authorize spending. |
| Dependency inversion | Inner layers own ports; outer adapters depend inward. Runtime DI does not excuse compile-time imports pointing outward. |

## CQRS contract

CQRS here means a separate application command/query boundary, not mandatory
database splitting or event sourcing:

- Commands may change state, publish domain events through a port, and must be
  idempotent where the contract says so.
- Queries are read-only from the caller's perspective. They may use a cache or
  replica, must expose cursor/as-of semantics when consistency is not strong,
  and must not emit business side effects.
- A command handler never returns an infrastructure model; a query handler
  returns an application DTO/projection.
- One PostgreSQL schema is the default. Separate read models, replicas, or an
  event-driven projection require an ADR and a measured consistency contract.
- CQRS does not turn an immutable Posting into a workflow state machine. Payment
  and payout lifecycles remain separate aggregates.

## Go idioms and toolchain

- Run `gofmt` (and `goimports` where available); keep exported declarations
  documented with sentence-style comments. Use short, lower-case package names.
- Prefer concrete return types and consumer-owned interfaces. Avoid empty
  interfaces when a type parameter or a meaningful contract is possible.
- Use ordinary Go `(value, error)` returns for internal APIs. A generic
  `Result[T]`/monad is optional and requires an ADR showing a concrete benefit;
  transport envelopes are separate DTOs.
- Pass `context.Context` as the first parameter for cancellable I/O. Never store
  a context in a struct or use `context.Background()` inside request handling.
- Return and wrap errors with `%w`; use `errors.Is`/`errors.As` for inspection.
  Error strings are lower-case and do not duplicate user-facing messages.
  Never use `panic` for expected input, database, provider, or shutdown errors.
- Domain packages return typed domain errors/violations; application and
  interface layers translate them to the stable `AppError`/protocol envelope.
- Use `log/slog` for new structured logging code (Echo v5 also uses it). A
  different logger is allowed only behind the kernel port and an explicit
  benchmark/ADR; never let logging choice leak into domain code.
- Use `time.Time` in UTC at boundaries, inject a clock for deterministic tests,
  and make timeout/cancellation behavior explicit. Every goroutine has an owner,
  cancellation path, and bounded work; use `errgroup` for bounded fan-out.
- Prefer standard-library `slices`, `maps`, `cmp`, `log/slog`, and `testing`
  facilities before adding a dependency. Keep `sonic` behind the repository's
  JSON wrapper and prove compatibility/performance before using it broadly.
- CI runs `go test`, `go test -race`, `go vet`, `staticcheck`, `govulncheck`,
  and the configured linter. Generated code is reproducible and is never edited
  by hand.

## Unit testing conventions

Every test suite in this repository adheres to a strict table-driven pattern optimized for readability, test case isolation, and parallel thread safety:

### 1. Locally scoped table schema
- Declare `type testCase struct` directly inside the test function (never at package level).
- Field names must match the target function's parameter names (e.g. `amountMinor`, `rate`, `from`, `to`) and return types (`expectedResult`, `expectedResult1`, `expectedResult2`, `expectedError`). Do not use vague names like `param1`, `arg`, or `expect`.

### 2. Zero single-use variables outside `testCases`
- **Anti-pattern:** Declaring single-use variant structs (`noID`, `negAmount`, `brokenRate`, `underReview`) before `testCases := []testCase{ ... }`. This pollutes the function scope, creates long jumps when investigating test failures, and risks shared-state mutation across subtests.
- **Required pattern:** Inline all test-specific inputs directly within the test case entry. When deriving a variant from a base template, use an immediately-invoked anonymous closure:
  ```go
  {
      name: "zero amount",
      req: func() service.TransferRequest {
          r := baseReq
          r.AmountMinor = 0
          return r
      }(),
      expectedResult: service.TransferLines{},
      expectedError:  entity.NewError("INVALID_TRANSFER_AMOUNT", "transfer amount must be positive"),
  },
  ```

### 3. Multi-line formatting
- Every test case entry in `testCases` must be formatted across multiple lines using standard Go indentation. Never condense test cases into dense one-liners.

### 4. Concurrency & Determinism
- Parent tests must invoke `t.Parallel()`.
- Do NOT use `tc := tc` (obsolete in Go 1.22+) and do NOT invoke `t.Parallel()` inside subtests (`t.Run`), keeping subtests clean, sequential, and deterministic.
- Test helpers and generators must be strictly thread-safe (e.g., using `sync.Mutex` or atomics for sequential counters).
- No global state or package-level shared mutable fixtures.

### 5. Edge-case rigor
- Coverage is a secondary metric to correctness. Test cases must explicitly explore:
  - Zero, negative, and maximum boundary values (`math.MaxInt64`, arithmetic overflow cases).
  - Empty, whitespace, illegal character, and length boundary strings.
  - State machine illegal and legal transitions.
  - Expired, stale, or unverified preconditions.

### 6. Complete Unit Test Examples

#### Example A: Method on Struct / Entity Returning `error`
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

#### Example B: Function with Multiple Scalar Parameters and Multiple Returns
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

#### Example C: Context-Aware Function with Request and Response DTOs
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

### 7. Canonical Reference Test Files in Codebase

| Function Signature Pattern | Canonical Reference File | Tested Behaviors & Key Patterns |
| :--- | :--- | :--- |
| **Entity / Struct Validation** `(e Entity) Validate() error` | [`internal/domain/entity/settlement_batch_test.go`](../../internal/domain/entity/settlement_batch_test.go) | Base struct template, inline closures for field mutations, multi-line table |
| **Entity Validation with Slices & Enums** `(p PaymentLink) Validate() error` | [`internal/domain/entity/payment_link_test.go`](../../internal/domain/entity/payment_link_test.go) | Boundary strings, enum validity, slice empty/populated checks |
| **Pure Computation with Bounds** `Func(amount, bps, floor, cap) (result, error)` | [`internal/domain/service/fee_interest_test.go`](../../internal/domain/service/fee_interest_test.go) | Exact scalar parameter mapping, math overflow, inverted bounds |
| **Complex Domain Service with Maps** `ValidateRefund(req, exists, accts) (Lines, error)` | [`internal/domain/service/refund_service_test.go`](../../internal/domain/service/refund_service_test.go) | In-memory lookup maps, complex DTO structs, timestamp derivation |
| **Value Object Parser / Validator** `ValidateDescriptor(s, network) error` | [`internal/domain/valueobject/descriptor_test.go`](../../internal/domain/valueobject/descriptor_test.go) | String length limits, ASCII control characters, network-specific rules |
| **Value Object Decimal Math / Factory** `NewFXRate(base, quote, rate) (FXRate, error)` | [`internal/domain/valueobject/fx_rate_test.go`](../../internal/domain/valueobject/fx_rate_test.go) | String decimal precision parsing, invert calculation, zero rate check |
| **State Machine Transition Matrix** `CanTransitionPayment(from, to) bool` | [`internal/domain/valueobject/payment_status_test.go`](../../internal/domain/valueobject/payment_status_test.go) | Exhaustive state transition pairs, terminal state assertions |

## Ledger-specific rules

- Amounts are checked integer minor units (`int64` at the edge, wider checked
  accumulation where required); floats and implicit decimal parsing are banned.
- A Posting/Entry is immutable after commit. No `Save`/`Delete` API may be used
  as a generic journal mutation path. Corrections are new linked postings.
- Database transactions, uniqueness, lock order, balance checkpoints, and the
  outbox/inbox are correctness boundaries. Cache, Redlock, retries, and broker
  acknowledgements are not substitutes.
- Keep operation-specific ports separate: a display `Cache` cannot authorize
  spending or silently become a rate limiter; HTTP middleware consumes a narrow
  `RateLimiter` port backed by Valkey (or a deterministic test adapter).
- Public handlers never trust tenant IDs, account IDs, or authorization claims
  supplied solely in an ordinary body field. Scope comes from authenticated
  context and is checked again in the application/persistence boundary.

## Required proof

Every task packet must name the relevant checks. At minimum, implementation
changes run:

```bash
# Bootstrap toolchains and dependencies (if any tool is missing):
./scripts/dev/setup.sh  # or: make setup

# Ensure toolchain is on PATH:
export PATH="$HOME/.pixi/bin:$PATH"

# Verification pipeline:
go mod verify
go mod tidy -diff
gofmt -s -w .
make lint
go test -v -race ./...
go vet ./...
./tasks/scripts/gate-check.sh <GATE>
python3 tasks/scripts/check-tasks.py --format --graph --sdd --specs --events --codes
```

Financial or external-boundary changes also run the packet's integration,
property, fault-injection, and direct-SQL tests. A rule that cannot be checked
by a command, test, or review assertion must be recorded as an explicit risk.

## References

- [Go release history](https://go.dev/doc/devel/release)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [Effective Go](https://go.dev/doc/effective_go)
- [Echo v5 context and handler API](https://echo.labstack.com/guide/context/)

