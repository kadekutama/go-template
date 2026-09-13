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
go mod verify
go mod tidy -diff
gofmt -w <changed-go-files>
go test ./...
go test -race ./...
go vet ./...
python3 tasks/scripts/check-tasks.py --format --graph --sdd
```

Financial or external-boundary changes also run the packet's integration,
property, fault-injection, and direct-SQL tests. A rule that cannot be checked
by a command, test, or review assertion must be recorded as an explicit risk.

## References

- [Go release history](https://go.dev/doc/devel/release)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [Effective Go](https://go.dev/doc/effective_go)
- [Echo v5 context and handler API](https://echo.labstack.com/guide/context/)
