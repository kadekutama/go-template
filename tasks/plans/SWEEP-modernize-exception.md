# Owner-Directed Exception: Repo-Wide Modernize Sweep (2026-10-07)

**Direction:** repo-owner, in-session ("Sweep & fix" vehicle: "Owner exception
only"; generated mocks: "Upgrade mockery to v3").
**Scope:** all 102 `modernize` findings from `make lint` (golangci-lint with
the complete analyzer set per 2026-10-07 AGENTS.md), across completed epics
and generated code.
**Why an exception:** the findings live in files owned by completed epics
(E02–E09, released claims) and in mockery-generated files. No single task
packet covers them, and the owner directed execution without new task
artifacts. This file is the durable record in place of a packet/claim.

**Rules for this sweep (binding on the implementer):**
1. Behavior-preserving mechanical edits only. Any edit that changes
   observable behavior stops and is reported; it does not land silently.
2. CORRECTION (verified 2026-10-07): the failing gate (`golangci-lint`
   modernize, 102 findings) contains ZERO generated-mock findings —
   `test/mock/**` is not flagged. The `interface{}` hits live only in the
   non-blocking `gopls check` Hint output (860 lines, exit 0). Therefore NO
   mockery upgrade, NO setup.sh change, and NO generated-file edits are
   needed or performed. `gopls` Hints stay informational per Makefile design.
3. No analyzer is disabled and no exclusion is added to silence findings.
   If a finding cannot be fixed mechanically, it is reported with the file
   list instead.
4. Verification per batch: `go build ./...`, affected package suites,
   `-race` on concurrency-touching edits, `gofmt`, and the full
   `check-tasks.py` structural gate. Final: full `go test ./...`, `make
   lint` exit 0, SDD `OK`.
5. Completed epics' evidence is re-verified, not rewritten: results are
   recorded here.

## Inventory and results

(Filled as the sweep executes; each batch appends commands, outcomes, and
the exact files changed.)

### 2026-10-07 execution (opencode-agent, under this exception)

Baseline: 102 `modernize` findings (`golangci-lint run ./...`), zero in
generated `test/mock/**` (those `interface{}` hits exist only in the
non-blocking `gopls check` Hint output, exit 0 — no mockery upgrade, no
setup.sh change, no generated-file edits needed or performed).

Batches applied (all behavior-preserving mechanical transforms):
1. `slices.Contains` (`command/report.go`); `omitempty` on nested struct
   removed (`dto/extended_reports.go`); `t.Context` (`coordination/etcd/
   watcher_internal_test.go`); `strings.SplitSeq` (`domain/service/
   isolation.go`).
2. `for i := 0; i < N; i++` → `for i := range N` / `for range N`
   (15 test files; two sites kept the index where the body uses it:
   `allocation_property_test.go` case numbers, audit `w/i` resource names,
   persistence `workerID`, issuer `strconv.Itoa(i)`).
3. `maps.Copy` for pure map clones (9 files: journal, transaction,
   gdpr_service, specification/transaction, ops_test, doubles,
   redpanda_dlq_test, regulatory_reports_test, kernel/log/context).
   Mutating loops (jwt/keys.go second loop setting KID) left intact —
   first loop converted.
4. `embedlit` promotion to Go 1.27 promoted-field form: `account.go`
   (`Actor/EventID/OccurredAt` promoted), `tenant_test.go` (10 sites),
   `transfer_service_test.go` (2 sites), testcontainers + etcd request
   literals (6 files). NOTE: the naive `Type: {…}` elision is INVALID Go
   under this toolchain (verified with a probe program); the analyzer's
   documented fix is full promotion, which is what landed.
5. Reflection iterators: `reflect.TypeFor[T]()`, `Type.Fields()`,
   `Type.Methods()` (account_test, posting_test, currency_test,
   catalog_test, config/loader_test).
6. `errors.AsType[T]` (transfer.go, service/error.go, saga.go,
   redpanda framework.go, resilience/breaker.go).
7. `wg.Go(func(){…})` closures fixed (specification_test,
   enforcer_test, persistence_test — trailing `()` removed; spec loop
   also `range workers`).
8. Self-caught breakage repaired same session: a bad `wg.Go` edit shape
   and the invalid embedlit elision broke 6 files; all repaired and
   verified before proceeding. `report.go` unreachable `return false`
   removed.

Verification: `go build ./...` 0; `go vet ./...` clean; `gofmt -s -l`
clean; full unit run `./internal/... ./pkg/... ./test/unit/...` zero
failures; `-race` on specification/rbac/coalesce/admin green;
`make lint` exit 0 with **0 issues** (was 102); SDD structural gate shows
only the 2 honest transient edges (T03/T05 completed while T02 awaits
adjudication).

INCIDENTAL (T02-claimed, not sweep scope): the sweep's full unit run
surfaced a pre-existing E09.1 codec-seam breach —
`cronui/session.go` imported `encoding/json` directly
(`TestCodecSeam` failure). Fixed under the active T02 claim by switching
to the `pkg/jsonparser` wrapper + gofmt/goimports; `jsonparser` and
`cronui` suites green, no lint findings. Recorded here so the sweep log
explains the extra diff.

### 2026-10-08 mockery v2.53.7 → v3.8.0 upgrade (owner-ordered)

Owner: "Upgrade the mockery. I don't want to use old versions."
Probe results that shaped the execution:
- v3 ships a `migrate` command; the v2 config needed it (`outpkg` key
  removed, `pkgname` instead, `filename` dropped, `with-expecter`
  dropped). All 18 package entries preserved by migration.
- v3 default output is a SINGLE `mocks_test.go` per package — but
  `_test.go` files are unimportable, so that layout breaks every
  consumer. Fixed with per-package `filename: "{{.InterfaceName}}.go"`
  (v3 supports `{{.InterfaceName}}` in filename templates), restoring the
  exact v2 file layout.
- v3 output is API-compatible with repo usage: same `NewMockX(t)`
  constructor with auto-assert cleanup, same `EXPECT()`/`Return()`/
  `Once()`/`RunAndReturn`, same `mock.Calls`/`ExpectedCalls` testify
  surface. Only change: Expecter params use `any` instead of
  `interface{}` (which also clears the gopls `interface{}` hints in
  generated code).
- No `mockery migrate` surprises beyond the above; no template override
  needed.

Repo changes: `.mockery.yaml` rewritten to migrated v3 form (+ header
comment recording the upgrade); `scripts/dev/setup.sh` pin
`vektra/mockery/v2@latest` → `vektra/mockery/v3@v3.8.0`; full regen via
`./scripts/generate/mocks.sh` (97 old v2 files deleted, 18 v3 files land
in identical paths); `gofmt -s` + `goimports -w` over `test/mock/`
(v3 template import order needs one goimports pass).

### 2026-10-13 repair round (owner-approved, T02 Reopen Round 4)

Another harness wiped tracked-file work via `git checkout` (go.mod/go.sum
casbin v3 + gobreaker v2 + env v2 pins, deps.go anchors, enforcer.go and
gobreaker.go/loader.go import swaps) and left same-path bumps (casbin
v2.135.0, gocron v1.37.0) plus a `go 1.27.2` directive. Owner approved
full repair + latest-Go ruling. All previously-verified states restored
verbatim from the approved record (verified by diff-against-record, not
memory): T06 v3.11.0 + v3 imports + /v3 anchor, T01 /v2 anchor, gobreaker
v2.4.0 + `[any]` instantiations, env v2.0.2 + Opt transform, tidy clean.
Go pins synced to 1.27.2 (go.mod was already there; SPEC.md, AGENTS.md,
CI aligned). Untracked E09.1 files, docs, and all task records were never
touched (verified present before editing). Re-verified: build 0, targeted
suites + parity race green, full unit zero failures, `make lint` 0 issues.

### Gate hardening (owner question, implemented 2026-10-07)

`make lint` could never fail on gopls hints: `gopls check` exits 0
regardless of output and nothing parsed it — so improvement warnings
were printable but unenforceable, and `tail`-truncated runs hid them
entirely. Fixed `Makefile:gopls-check` to the same fail-on-output shape
as `fmt-check`: any diagnostic is printed to stderr followed by
`gopls-check failed`, exit 1. Proven both directions: clean tree passes;
reintroduced probe diagnostic fails loudly (`make: *** Error 1`, probe
reverted afterwards — `period.go` diff is the intended sweep fix only).
CI already runs `make lint`, so enforcement propagates there with no
workflow change. Note: CI installs `gopls@latest` unpinned vs the
pixi-pinned local toolchain — a future skew source if hint sets drift;
flagging, not fixing, under this exception.

Known flake (recorded, not hidden): `TestRunAppLifecycle/
signal-driven_server_stops_cleanly` (shared/di) failed once during the
first full-suite run and passed 4/4 afterwards (3× `./internal/shared/...`
reruns + `-race`). Cause: fixed 300ms self-SIGTERM vs fx startup under
parallel-package load — a timing-sensitive test in an untouched package
(sweep touched no di/signal/startup code). No action taken; re-running
the suite is the correct response to a load flake, not editing another
epic's test timing.

### 2026-10-08 dependency upgrade round (owner-ordered, dep-audit items)

Owner: "fix the old versions to the newest ones" per the 145-module audit.
All four audit sections addressed:

1. **Same-path bumps (6 modules + lockstep):** kadm v1.11.0→v1.19.0, otel
   family v1.46.0→v1.47.0 (core + sdk + both exporters + trace/metric;
   pulled genproto patch bumps and otel/log split with it), echo
   v5.3.1→v5.4.0, go-redis v9.22.0→v9.23.0, koanf v2.3.6→v2.3.8,
   pgx v5.10.0→v5.11.0, plus stragglers moby/api→v1.56.1,
   franz-go→v1.22.1, etcd/client→v3.7.2. Verified: `go list -m -u all`
   shows zero direct deps behind afterwards.
2. **yaml fork migration:** `gopkg.in/yaml.v3` (frozen 2022) dropped from
   the graph; the 2 direct import sites (`egress/config.go`,
   `kernel/error/translator.go`, both `Unmarshal`-only) moved to
   `go.yaml.in/yaml/v3 v3.0.5`. One goimports grouping fixup.
3. **Major jumps (both single-file, API-diffed first):**
   - gobreaker v1→v2.4.0: v2 is generic
     (`NewCircuitBreaker[T]`); adapter parameterized `[any]`, zero logic
     change; resilience suites green.
   - koanf env provider v1→v2.0.2: v1 `Provider(prefix, delim, func(s))`
     vs v2 `Provider(delim, Opt{Prefix, TransformFunc func(k,v)})` —
     verified semantically identical to the old `ProviderWithValue`
     shape (prefix filter → full-name transform → empty-key omission);
     transform ported verbatim; config suites + `validate-config` green.
4. **Replace pins lifted** (both undocumented since E07.1): pgx now
   effective v5.11.0 (was v5.6.0), go-connections v0.8.1 (was v0.5.0).
   Verified: DB unit suites + persistence integration
   (`TestMigrations`, real Postgres) green. No rationale found for the
   pins, so lifting + green tests IS the documentation.
5. **SPEC.md §2 rows synced:** echo, go-redis, gobreaker/v2, otel, etcd,
   koanf.

Out of scope (audit's own verdict): indirect-only majors (backoff,
kmsg, yaml v4 RC), `golang/protobuf` deprecation (parent-controlled),
archived-repo check (0 archived — nothing to do). Probe-script CI
wiring NOT done (audit offered it as a follow-up needing an SDD packet;
awaiting owner word).

Verification: `go build` 0; full unit run zero failures; persistence
integration green; `make lint` exit reflects only the pre-existing
`default:all` style backlog (docs/LINTERS.md), zero diagnostics in this
round's files beyond unconfigured-depguard default noise.
