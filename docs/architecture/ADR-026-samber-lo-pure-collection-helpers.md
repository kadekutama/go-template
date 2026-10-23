# ADR-026: Pure Generic Collection Helpers (`samber/lo` v1.53.0) & Static Safety (`nilaway`, `fxtest`)

**Status:** Accepted
**Date:** 2026-10-16
**Accepted:** 2026-10-18 (repository owner, via `tasks/specs/E09.2-T01.md` approval)
**Owner:** E09.2-T01, E09.2-T02, E09.2-T03
**Related tasks:** `tasks/specs/E09.2-T01.md`, `tasks/specs/E09.2-T02.md`, `tasks/specs/E09.2-T03.md`

## Context

Even with Go 1.27's standard-library `slices`, `maps`, and `cmp` packages, common collection operations across our domain services, application workflows, and persistence adapters still require repetitive multi-line loops:

1. **Slice-to-Map Indexing & Grouping**: Building lookup indices (`map[K]V` from `[]V`) in `internal/application/workflow/saga.go` (`byName`) and `internal/infrastructure/database/postgres/repositories/posting.go` (`byID`), or grouping reconciliation items by external reference in `internal/domain/service/reconciliation_service.go` (`groupLedger`, `groupExternal`).
2. **Set Operations & Deduplicated Projections**: Computing set unions (`unionRefs` in `reconciliation_service.go`), deduplicating extracted foreign keys (`ids` in `posting.go`), and mapping infallible row slices to domain entities (`search.go`, `account.go`, `entry.go`, `hold.go`).
3. **Architecture Boundary Question**: Historically, `.golangci.yml` (`depguard`) enforced a strict "Go standard library only" rule in `internal/domain/` as a proxy for the Clean Architecture Dependency Rule. However, in Clean Architecture (Robert C. Martin) and DDD, the Dependency Rule prohibits *outward architectural coupling* (I/O, databases, transport protocols, frameworks, global state, clocks, or side effects)—not pure, stateless, zero-dependency generic combinators (`func[T, R]([]T, func(T) R) []R`).
4. **Concurrency, Mutation & Nil-Safety Guardrails**: Conversely, `samber/lo` includes subpackages that contradict the pure-combinator contract—`github.com/samber/lo/parallel` spawns raw goroutines without `internal/shared/kernel/safe` panic containment, and `github.com/samber/lo/mutable` mutates the shared backing array of a slice in place, breaking value-object immutability—plus root retry/concurrency helpers (`lo.Attempt*`, `lo.Async*`, `lo.Transaction*`, `lo.NewDebounce*`, `lo.Synchronize*`, `lo.WaitFor*`) that would bypass `internal/infrastructure/resilience`. Meanwhile, static interprocedural nil-dereference analysis (`go.uber.org/nilaway`) and DI container graph validation (`fx.ValidateApp` + `go.uber.org/fx/fxtest`) strengthen compile/test-time safety without runtime overhead.

## Decision

1. **Adopt `github.com/samber/lo v1.53.0` as an approved pure generic utility library across all layers (`domain`, `application`, `infrastructure`, `interface`, `shared`)**:
   - Allow exactly the pure root package `github.com/samber/lo` in the `.golangci.yml` `depguard` rules for `domain`, `application`, and `composition-roots`; the `interface`, `infrastructure`, and `shared` rules stay `list-mode: lax` and rely on the deny list. The former `lo/mutable` allow entries were a drafting slip corrected by the repository owner on 2026-10-18 (C01): `lo/mutable` mutates shared slice backing arrays in place and is banned like `lo/parallel`.
   - Prefer `lo` when it replaces boilerplate loops with clear, infallible data expressions: `lo.Map`, `lo.Filter`, `lo.FilterMap`, `lo.Uniq`, `lo.UniqBy`, `lo.UniqMap`, `lo.KeyBy`, `lo.Keyify`, `lo.Associate`, `lo.GroupBy`, `lo.PartitionBy`, `lo.Chunk`, `lo.Find`, `lo.ContainsBy`, `lo.Union`, `lo.Intersect`, `lo.Difference`, `lo.Keys`, `lo.Values`, `lo.ToPtr`, `lo.FromPtr`, `lo.CoalesceOrEmpty`, and `lo.Ternary`.
   - Keep standard `slices`, `maps`, and `cmp` where the Go 1.27 standard library already provides an exact 1-call equivalent (`slices.Contains`, `slices.ContainsFunc`, `slices.Sort`, `slices.SortFunc`, `slices.Clone`, `maps.Clone`, `cmp.Or`).
   - Keep explicit `for _, item := range items` loops whenever the loop body performs fallible validation/parsing returning `(T, error)` with early `return nil, err`, checked integer overflow arithmetic (`docs/ledger-core.md`), or cancellable `ctx.Context` I/O.
2. **Ban `lo/parallel`, `lo/mutable`, and `lo` concurrency/retry/panic helpers via linter enforcement**:
   - Configure `depguard` in `.golangci.yml` to explicitly **deny** `github.com/samber/lo/parallel`, `github.com/samber/lo/mutable`, and the unapproved `github.com/samber/lo/it` iterator package (whose channel adapters spawn unmanaged goroutines) in all six rule sets (`domain`, `application`, `interface`, `infrastructure`, `shared`, `composition-roots`), directing engineers to `internal/shared/kernel/safe`, `golang.org/x/sync/errgroup`, and newly allocated slices.
   - Configure `forbidigo` in `.golangci.yml` to ban `lo.Attempt*`, `lo.Async*`, `lo.Transaction*` (including the `lo.NewTransaction` constructor), `lo.NewDebounce*`, `lo.Synchronize*`, `lo.WaitFor*`, the channel/goroutine helpers (`lo.ChannelDispatcher`, `lo.SliceToChannel`, `lo.ChannelToSlice`, `lo.Generator`, `lo.Buffer*`, `lo.FanIn`, `lo.FanOut`), and `lo.Must*` (per the 2026-10-18 owner decision C02), preserving `internal/infrastructure/resilience`, `internal/application/workflow`, and `internal/shared/kernel/safe` as the sole retry, saga, and goroutine primitives and keeping panic-free error returns in production code.
3. **Reject `github.com/sourcegraph/conc`**:
   - Go 1.25+ provides `sync.WaitGroup.Go`, and the repository already standardizes panic-safe goroutine execution in `internal/shared/kernel/safe` (`SPEC.md §9.7`) and `golang.org/x/sync/errgroup`. Adding a second concurrency wrapper would split panic-recovery semantics.
4. **Standardize `fx.ValidateApp` & `go.uber.org/fx/fxtest` (`v1.24.0`) across all DI modules (`E09.2-T02`)**:
   - Every `module.go` package must have a companion `module_test.go` that validates graph completeness via `fx.ValidateApp` (zero side effects) and tests `OnStart`/`OnStop` lifecycle hooks via `fxtest.New(t, ...)`. Raw `fx.New` in unit tests is prohibited.
5. **Enforce interprocedural nil-safety via `go.uber.org/nilaway` (`@latest`, aligned with the other developer CLI tools) (`E09.2-T03`)**:
   - Install `nilaway` in `scripts/dev/setup.sh`, expose `make nilaway` in `Makefile`, and require zero `nilaway` findings across `./internal/... ./pkg/... ./cmd/...`.

## Consequences

- Domain services, saga orchestrators, and SQL repositories eliminate repetitive slice/map indexing loops while preserving 100% pure, deterministic domain semantics.
- Linter guardrails (`depguard` + `forbidigo`) mechanically prevent accidental misuse of `lo/parallel`, `lo/mutable`, or `lo.Attempt*`.
- Every `fx.Module` is statically verified against missing providers/cycles (`fx.ValidateApp`) and lifecycle leaks (`fxtest`), and interprocedural nil flows are caught before runtime (`nilaway`).

## Rollback

Revert `.golangci.yml` `depguard`/`forbidigo` rules, replace `lo.*` call sites with explicit `for` loops, and remove `github.com/samber/lo` from `go.mod`.

## Resolution Notes (2026-10-18, `E09.2-T01`)

- **C01 — `lo/mutable` depguard allow vs. documented ban (corrected by the repository owner, 2026-10-18).** The original epic, packet, and this ADR drafted an allow entry for `github.com/samber/lo/mutable` in `domain`, `application`, and `composition-roots`, contradicting `AGENTS.md`, `docs/development/go-conventions.md` item 4, `docs/architecture/clean-architecture-guide.md §4`, and `SPEC.md §2`. The owner ruled the allow entry a drafting slip: `github.com/samber/lo/mutable` mutates shared slice backing arrays in place and therefore breaks value-object immutability; only the pure root combinators of `github.com/samber/lo` are approved. The allow entries were removed, `github.com/samber/lo/mutable` is now explicitly denied in all six `depguard` rule sets alongside `github.com/samber/lo/parallel`, and the G2 domain-import check, the hermetic domain-import guard, `SPEC.md §2`, the packet's R02, and the epic's E09.2-T01 steps and acceptance criteria were reconciled.
- **C02 — `lo.Must*` ban.** The owner approved adding `lo.Must`, `lo.Must0`…`lo.Must6` to the `forbidigo` ban even though the epic's regex omitted them; `forbidigo` has no per-file scoping, so the ban also covers test code, matching the repository's `require.NoError` test policy.
- **R1 — review round 1 hardening (2026-10-18).** Adversarial review `tasks/reviews/E09.2-T01.md` (CR-001/CR-002) showed that `lo.NewTransaction`, `lo.SliceToChannel`, `lo.ChannelToSlice`, `lo.Generator`, and the unapproved `github.com/samber/lo/it` subpackage could bypass the ADR-026 guardrails; the `forbidigo` list now covers those helpers and `depguard` denies `lo/it` in all six rule sets. CR-004's pre-indexed alias set is implemented with `lo.Keyify`, now listed among the approved root combinators. CR-003 (aliasing/dot-import evasion) was contested with probe evidence: with `analyze-types: true`, `forbidigo` resolves the canonical package name, so aliased and dot-imported calls are rejected. A hermetic guard (`test/unit/domain/guard_test.go#TestImplementationTreeAvoidsBannedLoHelpers`) mirrors the ban list independently of `golangci-lint`, catching aliased calls, dot imports, and `lo` subpackage imports even when `issues.uniq-by-line` hides a linter finding on a line that already reports another issue.
- **Verification.** The guardrails were proven with temporary probe files importing `github.com/samber/lo/parallel` / `github.com/samber/lo/mutable` / `github.com/samber/lo/it` and calling `lo.Attempt`, `lo.NewTransaction`, and `lo.SliceToChannel`: `golangci-lint run` reported the `depguard` import rejections and the `forbidigo` call rejections, and the probes were deleted before handoff (`tasks/evidence/E09.2-T01.md`).
