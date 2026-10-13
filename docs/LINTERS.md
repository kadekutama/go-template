# Linter Census — Owner Policy (2026-10-13, remediated to zero)

## Provenance

- **Toolchain:** golangci-lint 2.14.0; Go 1.27.2; Linux/amd64.
- **Checkout:** `feat/E09.1-T01-gocron-v2`, base commit
  `28b01f791040d0b92aed2ee663c461ea69c040a7`, with the existing dirty shared
  working tree. This describes that working-tree snapshot, not a clean commit.
- **Configuration validation:** `golangci-lint config verify` passed after
  updating the v2 output-format and goimports-prefix schema.
- **Inventory:** `golangci-lint linters` confirms the policy below; all other
  available non-deprecated linters remain enabled, including `modernize`,
  `paralleltest`, `tparallel`, `depguard`, `ireturn`, and `testpackage`.
- **Scan:** `PATH="$HOME/.pixi/bin:$PATH" CGO_ENABLED=0 golangci-lint run ./...`
  exited **0** with **0 issues**. `PATH="$HOME/.pixi/bin:$PATH" make lint`
  (fmt-check + gopls Hint diagnostics + golangci-lint + shellcheck) also
  exits **0**. The final outputs were captured at
  `/tmp/opencode/make-lint-final-20261013.log` and the full race suite at
  `/tmp/opencode/go-test-race-final2-20261013.log` (exit 0, zero failures,
  zero data races).

An exit status of 1 is expected for this census: the strict lint gate reports
all remaining findings. No Go source or test findings were fixed as part of
this configuration/report task.

## Configuration decisions

`default: all` remains in effect. The configured non-deprecated disables are
exactly the owner-selected set:

- `lll`, `godot`, `funlen`, `varnamelen`, `cyclop`, `dupl`,
  `exhaustruct_v5`, `noinlineerr`, `wrapcheck`, `containedctx`,
  `gochecknoinits`.

The three deprecated names remain disabled: `exhaustruct`, `gomodguard`, and
`wsl`. The available replacements are handled separately: `gomodguard_v2` and
`wsl_v5` remain enabled; `exhaustruct_v5` is disabled by the explicit owner
request above.

Additional settings:

- `testifylint` disables only `require-error`; its other checks remain active.
- `forcetypeassert` stays enabled, with findings excluded only from `_test.go`
  files. The final scan reports no `forcetypeassert` findings.
- `tagliatelle` applies `snake` case to JSON and YAML tags, matching the
  database's snake_case contract (the on-disk `max_wait` key stays as is).
- `paralleltest.ignore-missing-subtests: false` keeps both parent- and
  subtest-level omissions visible for a safety-reviewed sweep.
- `ireturn` remains enabled. Since an explicit `allow` list replaces its
  implicit defaults, the config preserves `empty`, `anon`, `error`, and
  `stdlib`, allows generic type-parameter returns, and adds only 20 exact,
  package-qualified named-interface patterns from the audited baseline.
- Targeted exclusions (each with an inline justification comment; no
  `//nolint` in Go sources): `gosec` G115 for the guarded Otter saturating
  cast and G101 for ephemeral testcontainer credentials; `spancheck` for
  the OTel `Tracer.Start` adapter; `contextcheck` for the singleflight
  shared call, the lock-guard release path, and the NATS fx hook;
  `recvcheck` for the GORM models; `testpackage` for white-box suites;
  one combined `paralleltest`+`tparallel` text rule covering the seven
  `t.Setenv` suites (exact names; the runtime forbids `t.Parallel` with
  `t.Setenv` and siblings share process env); one `paralleltest`+
  `tparallel` path rule for `test/integration/` (ordered shared-fixture
  lifecycles); one combined `gochecknoglobals` path+identifier rule for
  the six load-bearing bindings. (The lock-guard `contextcheck` scope was
  removed after a real fix: `releaseLease` now takes the caller context
  with cancellation detached via `WithoutCancel`, preserving the
  must-release-on-cancel guarantee.)
- Output and formatter settings use the validated golangci-lint v2 schema.

## Findings by enabled linter

| Linter | Findings | Linter | Findings |
|---|---:|---|---:|
| *(all enabled linters)* | *0* | | |
| **Total** | **0** | **Finding-producing linters** | **0** |

No linter reports findings under the current configuration. The historical
baselines this remediation started from were 7,970 findings (35 linters,
all-linter census) and, after the first auto-fixable and mechanical passes,
238 findings (20 linters, captured at
`/tmp/opencode/lint-remaining-20261013.log`). Linters that report zero
because of a configured rule below are named there, not omitted silently.

## Remediation record (238 → 0)

The 238-finding baseline above was remediated without `//nolint` comments
(`grep -rn nolint --include=*.go` is empty outside the linter's own
`nolintlint` self-checks). Fixes are behavior-preserving except where noted;
every batch was verified with `go vet` and the affected `go test` packages,
and the final state passes the full `go test -race ./...` suite with zero
failures and zero data races.

- `contextcheck` (7): threaded caller contexts through the port fake, the
  zerolog test callback, and the cache integration defer; hoisted the
  Redpanda correlation out of the traced closure; derived the shutdown
  drain timeout from the run context via `WithoutCancel` (drain must
  outlive cancellation, so a bare derived context would break shutdown).
  Three sites keep their current shape by documented design and are
  scoped in config: the singleflight shared call (must outlive any
  single waiter), the lock-guard release (must run even when the caller
  cancels, else keys stall to TTL), and the NATS fx `OnStop` hook
  (`nats.go` `Close` takes no context).
- `errname` (2): renamed `spendFailure` → `spendError` and the `safe.Panic`
  error type → `safe.PanicError` (the `Logger.Panic` severity method is
  unchanged; an early bulk rename that touched it was reverted).
- `exhaustive` (11): added the missing enum cases with the same outcome
  the `default` arm already produced (terminal states → `false`/the same
  typed error), verified against ledger-core transition comments.
- `fatcontext` (1): covered by the Redpanda correlation hoist above.
- `gochecknoglobals` (27): converted read-only tables, route lists, and
  test time fixtures to functions (fresh values per call; `time.Time`
  is immutable); inlined the Sonic profile at its four call sites. Four
  load-bearing singletons stay and are scoped in config with justification:
  the atomic error-code registry (init-seeded, runtime `Register`),
  the zerolog shape `OnceFunc` (must run exactly once), `sync.Map`
  mock-attached tracker state in tests (testify mocks cannot carry
  fields), and the atomic test-code counter (uniqueness needs sharing).
- `godoclint` (9): one package doc per package (duplicates demoted to
  section comments with content preserved); symbol-leading doc comments.
- `maintidx` (13): split oversized table tests by scenario area (golden /
  validation / per-domain slices) and extracted shared fixtures into
  builders; tables, assertions, and coverage are unchanged.
- `mnd` (58): named constants per package (`mnd_consts.go`), values
  preserved; financial divisors, length bounds, timeouts, and capacity
  hints keep domain-meaningful names.
- `modernize` (2): `slices.Contains` loop rewrite; dropped ineffective
  `omitempty` on a nested `time.Time` JSON field.
- `nilerr`/`nilnil` (2+2): mock responder returns the parse error; the
  bootstrap denial path was extracted so the handler no longer checks
  an error and returns nil; disabled admin config now returns an
  explicit `ErrAdminDisabled` sentinel (tests updated to the contract).
- `nosprintfhostport` (6): `net.JoinHostPort` for the host portion of
  etcd/Postgres DSNs (identical output for test hosts; correct for IPv6).
- `prealloc` (7): pre-sized fingerprint parts and mock-observation slices.
- `recvcheck` (4): `Registry.Lookup` to pointer receiver (the mutating
  `Register` requires it; the single by-value caller still compiles).
  GORM models keep mixed receivers by necessity (hooks mutate, so they
  must stay pointer; `TableName` is called on composite literals, so it
  must stay value) and are scoped in config.
- `revive` (51): package docs, symbol-leading comments, justification
  comments for side-effect blank imports, `max` shadowing renames, ctx-first
  test helpers, and stutter renames (`CommandHandler`→`Handler`,
  `EventMetadata`→`Metadata`, `*Params`→`Params`, etc., with mock file move
  and doc updates; build-verified).
- `spancheck` (2): scoped in config — `Tracer.Start` must hand the live
  span to the caller; ending it inside `Start` would record nothing.
- `staticcheck` (11): omitted inferable types; merged duplicate NATS
  test imports under the `edgenats` alias.
- `tagliatelle` (1): `yaml: snake` configured to match the JSON/database
  snake_case contract instead of renaming the on-disk `max_wait` key.
- `testifylint` (1): dropped the no-op blank `testify/assert` import.
- `unused` (21): wired the generated `mnd` constants into their call
  sites (remaining dead ones deleted).
- `paralleltest`/`tparallel`/`thelper`/`testpackage`/`wsl_v5`/`nlreturn`/
  `whitespace`/`gofmt`/`nonamedreturns`/`funcorder`/`gosec`/`err113`:
  zero via the earlier passes (safe subtest parallelism with per-subtest
  fixtures where mocks are shared; `t.Setenv` tests and ordered
  integration lifecycles documented as sequential in config; duplicate
  `t.Parallel()` calls removed).

## Owner questions and dispositions
## Owner questions and dispositions

### `depguard`: what architecture boundaries does it enforce?

Without project rules, `depguard` defaults to allowing only standard-library
imports. Its old `Main` label was a rule name, not Go's `internal` visibility
check: Go permits this module's `cmd/**` packages to import its own
`internal/...` packages.

The current configuration replaces that default with production-layer rules:

- `internal/domain/**`: strict allowlist for stdlib and domain packages.
- `internal/application/**`: strict allowlist for stdlib, application/domain,
  and the documented `pkg/jsonparser` and `internal/shared/kernel/error`
  seams.
- `internal/interface/**`: deny imports from infrastructure, DI, and `cmd`.
- `internal/infrastructure/**`: deny imports from interface, DI, and `cmd`.
- Non-DI `internal/shared/**`: deny imports from domain, application,
  infrastructure, interface, and `cmd`.
- `cmd/**` and `internal/shared/di/**`: explicit composition-root exceptions;
  they may select and wire concrete adapters.

These rules exclude Go test files. Config verification and
`golangci-lint run --enable-only depguard ./...` pass with zero issues. The
full census likewise has zero depguard findings. This checks import boundaries,
not all SOLID/clean-code properties: depguard cannot establish single
responsibility, substitutability, or good API design. The prior default-only
census emitted 1,555 depguard findings because no project rules existed; valid
composition-root imports should not be removed to satisfy that default.

References: [golangci-lint depguard settings](https://golangci-lint.run/docs/linters/configuration/#depguard)
and [Go internal packages](https://go.dev/doc/go1.4#internalpackages).

### `tagliatelle`: is JSON case snake case by default?

No. Its default JSON rule expects camel case. The config explicitly sets JSON
tags to `snake` for consistency across JSON/API fields and compatibility with
database naming. The single remaining result is a YAML tag (`yaml(camel)`:
`max_wait`, expected `maxWait`), not a JSON tag; it remains visible for a
separate YAML naming decision.
Reference: [golangci-lint tagliatelle settings](https://golangci-lint.run/docs/linters/configuration/#tagliatelle).

### `paralleltest`: where should `t.Parallel()` go?

The [upstream `paralleltest` README](https://github.com/kunwardeep/paralleltest/blob/main/README.md)
shows `t.Parallel()` both in the parent test and inside each `t.Run` callback
for independent cases. This is the repository's guidance now: parallelize only
after checking mutable fixtures and external resources for isolation; keep a
case sequential when order or shared state requires it, and record that concrete
reason. Use `t.Cleanup` when a parent resource must outlive parallel children,
put the case name in `t.Run(tc.name, ...)`, and do not add `tc := tc` under the
repository's Go 1.27 range-variable semantics. The upstream README also shows
`tc := tc`; that reinitialization workaround is unnecessary with this toolchain.

The earlier all-linter baselines reported 722 `paralleltest` findings (723
in a focused-only run, including one analyzer-variance finding at
`internal/infrastructure/cache/hybrid/engine_test.go:51`) and 623 `tparallel`
findings. Both are zero now: every independent case parallelizes parent and
subtest; `t.Setenv` tests (which the Go runtime forbids parallelizing),
ordered integration lifecycles (which share one container/DB per parent),
and the JWT shared-mock suites (reworked to per-subtest fixtures after the
race detector caught the sharing) are the documented sequential exceptions,
scoped in config per test name and path rather than silenced inline. A prior
bulk pass that parallelized indiscriminately also left 36 duplicate
`t.Parallel()` calls (which panic at runtime); all were removed.

`tparallel` remains enabled; do not suppress its findings or apply parallelism
without checking concurrency safety.

### `ireturn`: which interface returns are intentional?

Yes. The pre-exception baseline had 58 findings, but they were not all the same
design case. The current config keeps `ireturn` enabled and reports zero findings
for that baseline using 20 anchored package-qualified named-interface patterns,
the built-in `empty`/`anon`/`error`/`stdlib` allowances, and the `generic`
type-parameter allowance. These are type-level exceptions: every return of one
of the named interfaces is allowed, not only the audited function locations.
No package-prefix or catch-all exception is configured. The named types are:

- `internal/application/port`: `EventOutbox`, `IdempotencyStore`, `Lock`
- `internal/domain/event`: `DomainEvent`
- `internal/domain/repository`: `HoldRepository`, `PostingRepository`
- `internal/infrastructure/cache/hybrid`: `Codec`
- `internal/infrastructure/coordination/etcd`: `LeaderElector`, `Watcher`
- `internal/interface/cron/admin`: `SessionAuthenticator`
- `internal/shared/kernel`: `Clock`, `IDGenerator`
- `internal/shared/kernel/log`: `Logger`
- `internal/shared/kernel/trace`: `Span`, `Tracer`
- `github.com/pressly/goose/v3`: `Logger`
- `github.com/testcontainers/testcontainers-go`: `Container`
- `github.com/twmb/franz-go/pkg/sasl`: `Mechanism`
- `go.opentelemetry.io/otel/propagation`: `TextMapPropagator`
- `go.uber.org/fx`: `Option`

Consider the lock port in `internal/application/port/lock.go` and its Valkey
adapter in `internal/infrastructure/lock/redlock.go`:

```go
// Application-owned contract:
type Lock interface {
    Release(ctx context.Context) error
    Refresh(ctx context.Context, ttl time.Duration) error
}

type DistributedLock interface {
    Acquire(ctx context.Context, tenant valueobject.TenantID, key string, ttl time.Duration) (Lock, error)
}

// Infrastructure implementation (simplified):
func (r *Redlock) Acquire(ctx context.Context, tenant valueobject.TenantID, key string, ttl time.Duration) (appport.Lock, error) {
    // Validate, acquire the Valkey lease, then:
    return &redlockLease{client: r.client, key: leaseKey, token: token, logger: r.logger}, nil
}
```

A strict concrete change such as

```go
func (r *Redlock) Acquire(
	ctx context.Context,
	tenant valueobject.TenantID,
	key string,
	ttl time.Duration,
) (*redlockLease, error)
```

would make `Redlock` stop implementing `appport.DistributedLock`: Go requires
interface method signatures to match exactly and has no covariant return types.
Changing the application port to return `*redlockLease` would make the inner
application layer depend on an infrastructure implementation and would prevent
substituting another lock adapter without changing the port; the current
`redlockLease` type is also unexported and cannot be named from the application
package. A wrapper could restore the port but adds indirection without improving
the contract.

Heterogeneous domain events are a different case. In
`internal/application/command/account.go`, `firstEvent` returns the first item
from `aggregate.Account.UncommittedEvents()`, a `[]event.DomainEvent` that can
hold different concrete event types (created, frozen, closed, and others):

```go
func firstEvent(account aggregate.Account) event.DomainEvent {
    events := account.UncommittedEvents()
    if len(events) == 0 {
        return nil
    }
    return events[0]
}
```

There is no single concrete event type that preserves all those results. A
blanket concrete rule would force type switches, a tagged wrapper, or `any`,
making the result less direct and potentially narrowing behavior. Conversely,
`buildPostingPostedEvent` always constructs a
`TypedEvent[TransactionPostedPayload]`; its private return signature may be a
reasonable concrete-return candidate if callers benefit. This illustrates why
the decision belongs at each API, not in an all-or-nothing rule. For example,
the concrete candidate would be:

```go
func buildPostingPostedEvent(
	p PostingParams,
	entries []entity.Entry,
) (event.TypedEvent[event.TransactionPostedPayload], error)
```

That is unlike `firstEvent`: its result type is genuinely heterogeneous.

So, could every function be changed to return concrete types? A project could
choose that rule, but it would break contracts like `DistributedLock`, expose
adapter details, and complicate heterogeneous results. Concrete returns are
appropriate when the implementation is the intended API and the result has one
stable type. Keep `ireturn` enabled as a review signal, then decide API by API;
do not mass-refactor interface ports merely to silence the linter. The current
task changes documentation only, not these Go signatures.

### `testpackage`: black-box tests by default

Prefer external `package name_test` tests so behavior is verified through the
public API. The former 23 same-package hits are zero under a test-only
exclusion because the remaining white-box suites genuinely need unexported
access (per-instance logger exit stubs, in-package clock fakes); reviewers
should still challenge new same-package tests that could assert through the
public API instead.

## Targeted analyzer results

All targeted analyzers report zero findings:

- `testifylint` (only `require-error` disabled), `forcetypeassert`
  (test-only exclusion), `tagliatelle` (JSON+YAML snake case),
  `paralleltest`/`tparallel` (safe parent/subtest parallelism; `t.Setenv`
  suites and ordered integration lifecycles are scoped sequential),
  `testpackage` (white-box suites scoped), `ireturn` (narrow type
  allowlist), `spancheck`/`contextcheck`/`recvcheck` (documented
  design-conflict scopes), `gochecknoglobals` (globals converted to
  functions except four load-bearing scopes), `mnd` (named constants),
  `maintidx` (scenario-split suites), `funcorder`/`nonamedreturns`
  (reordered/unnamed), `gosec`/`err113` (sentinels and scoped rules),
  and `unused` (constants wired in).

## Scope and follow-up

This census records results and policy. The 238→0 remediation above was
performed under the owner-directed broad-remediation exception on the active
E18-T14 claim (same branch, no worktree); the exception, its verification,
and the one E09.1 work-preservation incident it caused are recorded in
`tasks/evidence/E18-T14.md` and `tasks/handoffs/E18-T14.md`. Outside such an
explicit exception, fix findings only under an active task claim that owns
the affected paths. The configured depguard boundaries cover imports only;
other SOLID/code-quality expectations remain in architecture guidance and
human review. Future regressions fail `make lint` (which runs before merge
per CI) at the first new finding.
