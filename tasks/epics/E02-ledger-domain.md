# Epic E02: Ledger Domain Core

**Status:** pending
**Story Points:** 30
**Phase:** 2
**Dependencies:** E01 (kernel: AppError, specs base, VOs need nothing else)
**SDD Gate:** G2 (specs executable, unit tests ≥90%, no external imports)
**Design refs:** `SPEC.md §5.1–§5.3`, `SPEC.md §13.1–§13.3`,
`docs/fintech-ledger-features.md §2`, `docs/money-flow.md §3–§4, §8`,
`docs/domain-events.md §2–§3.2`

> Why: the ledger's correctness lives here. Every money rule in
> `docs/money-flow.md §8` must be an executable spec before any adapter exists.
> Rule for this epic: **stdlib + kernel only** — an external import fails review.

## Tasks

### E02-T01: Domain event and specification primitives
**Status:** pending
**Background:** The narrow `DomainEvent` and `Specification[T]` contracts from
`SPEC.md §5.1–§5.2` that later tasks consume. Entities, value objects, and
aggregate roots remain concrete types; Go does not require universal base
interfaces for DDD terminology.
**Files:**
- Create: `internal/domain/event/base.go` (including
  `EventMetadata{tenant,ledger,causation,correlation,user,trace}` and
  aggregate-version/sequence accessors),
  `internal/domain/specification/specification.go` (stateless `Evaluate` result +
  `All/Any/Not` combinators)
**Steps:**
1. Implement the narrow event interface/envelope in `SPEC.md §5.1` plus
   `EventMetadata` from `docs/domain-events.md §2.1`; define concrete aggregate
   behavior in the later E02 tasks rather than a universal base interface.
2. `All` evaluates every child and aggregates typed violations; `Any` may
   short-circuit on first pass; `Not` has an explicit violation. No spec stores
   candidate-specific errors, so shared specs are race-safe.
3. Table/race tests for combinators and repeated concurrent evaluation.
**Acceptance Criteria:**
- [ ] `go test ./internal/domain/event ./internal/domain/specification -race -count=3` passes.
- [ ] `go list -deps ./internal/domain/... | grep -v -E "^(internal/|std)"` is empty (no external deps).
**Story Points:** 2
**Depends On:** E01-T06
**Related Docs:** `SPEC.md §5.1`, `SPEC.md §5.2`, `docs/domain-events.md §2.1`
**SDD Gate:** G2

---

### E02-T02: Minor-unit Amount, Asset/Currency Registry, and typed IDs
**Status:** pending
**Background:** Money math must never use float (`docs/fintech-ledger-features.md §2.1`,
`money-flow.md §3`). Integer-minor-unit arithmetic with ISO 4217 currency.
**Files:**
- Create: `internal/domain/valueobject/{money.go,currency.go,ids.go}` + tests
**Steps:**
1. `Money{amountMinor int64, asset AssetCode}`: checked Add/Sub/Mul/Div, Cmp,
   overflow rejection, positive-entry guards, minor-unit JSON semantics, and
   display formatting derived from the registry exponent; floats are forbidden.
2. Versioned asset/currency registry contract: code, exponent, kind, activation
   dates. Tests may seed ISO examples, but the sample list is not the authority.
3. Typed ULID IDs: `AccountID`, `PostingID` (with `TransactionID` retained only
   as a public compatibility alias), `EntryID`, `TenantID`, `UserID`, `JournalID`,
   `PeriodID` — type-safe, `String()`, `Equals()`, parse with validation.
4. Property tests for arithmetic (commutativity, associativity, no precision loss).
**Acceptance Criteria:**
- [ ] `0.1 + 0.2 == 0.3` equivalent holds in minor units (regression test).
- [ ] Cross-currency arithmetic returns `CURRENCY_MISMATCH` spec error, never panics.
- [ ] All VOs immutable (failing test if a setter is added — review check).
**Story Points:** 3
**Depends On:** E02-T01
**Related Docs:** `SPEC.md §5.1`, `SPEC.md §13.1`, `docs/fintech-ledger-features.md §2.1`, `docs/money-flow.md §3`
**SDD Gate:** G2

---

### E02-T03: Ledger and Account aggregates (classification, hierarchy, metadata)
**Status:** pending
**Background:** Core aggregate (`SPEC.md §13.1`, features §2.1, journeys §2.1).
Covers ledger ownership, chart classification, hierarchy, metadata, and account
status. Balances are projections owned by postings/checkpoints/holds, not mutable
fields on the Account aggregate.
**Files:**
- Create: `internal/domain/entity/account.go`, `internal/domain/aggregate/account.go`,
  `internal/domain/valueobject/account_type.go`, `account_status.go`
**Steps:**
1. `Ledger`: ID, TenantID, legal entity, base reporting currency, chart version.
   `Account`: ID, TenantID, LedgerID, ParentID (nullable), Number, Name, Class
   (ASSET/LIABILITY/EQUITY/REVENUE/EXPENSE), AssetCode, Status
   (ACTIVE/FROZEN/CLOSED), NormalSide, Purpose, Metadata, Version,
   timestamps. External bank/card instruments are separate workflow entities.
2. Methods: `Open`, `Freeze`, `Unfreeze`, `Close`, `Reparent`; no Deposit,
   Withdraw, balance setter, or microdeposit verification method on ledger accounts.
3. Hierarchy rule: asset code is explicit on every postable account. A roll-up
   returns one balance per asset code; unlike assets are never summed.
4. Metadata optimistic locking via `Version`; posting concurrency is repo-level
   deterministic `SELECT FOR UPDATE`/advisory locking (implemented in E07).
**Acceptance Criteria:**
- [ ] Account carries tenant+ledger+asset scope and has no mutable balance field (reflection test).
- [ ] Frozen/closed accounts reject new postings with `ACCOUNT_FROZEN`/`ACCOUNT_CLOSED`.
- [ ] Normal-side table matches `ledger-core.md §5`; both debit and credit are legal subject to a posting template.
- [ ] `go test ./internal/domain/... -race -count=3` passes.
**Story Points:** 5
**Depends On:** E02-T02
**Related Docs:** `SPEC.md §13.1`, `SPEC.md §13.2`, `docs/fintech-ledger-features.md §2.1`, `docs/money-flow.md §3–§4`, `docs/user-journeys.md §2.1`
**SDD Gate:** G2

---

### E02-T04: Immutable Posting + Entry and durable Hold aggregates
**Status:** pending
**Background:** Immutable double-entry records (`SPEC.md §13.1`, features §2.2).
**Files:**
- Create: `internal/domain/entity/{posting.go,entry.go,hold.go}`,
  `internal/domain/aggregate/{posting.go,hold.go}`
**Steps:**
1. `Posting`: ID, TenantID, LedgerID, operation+template version, references,
   EffectiveAt, server RecordedAt, ReversalOf, Metadata, Entries[]. An accepted
   posting has no pending/failed/voided state and exposes no mutator.
2. `Entry`: ID, PostingID, AccountID, Side, positive AmountMinor, AssetCode,
   AccountSequence. Signed amounts and stored `balance_after` are forbidden.
3. `Hold`: account/asset/amount/kind/state/expiry/version with idempotent capture,
   release, and expiry transitions; holds affect available balance but are not entries.
4. `ConstructPosting()` enforces positive entries, same tenant/ledger, account
   asset match, per-asset debit=credit, template rules, and reversal linkage.
**Acceptance Criteria:**
- [ ] Unbalanced debit/credit totals fail per asset code with both totals in details.
- [ ] A posting cannot be mutated; correction constructs an opposite-side linked posting.
- [ ] Cross-currency examples require independently balanced lots linked by FX trade ID.
- [ ] Hold transition table proves capture/release/expiry is idempotent.
**Story Points:** 4
**Depends On:** E02-T02, E02-T03
**Related Docs:** `SPEC.md §13.1`, `SPEC.md §13.2`, `docs/fintech-ledger-features.md §2.2`, `docs/money-flow.md §8`
**SDD Gate:** G2

---

### E02-T05: Journal, Period, and sub-ledger rules
**Status:** pending
**Background:** Missing entirely from Nemotron's breakdown (features §2.3:
Journal Entries, Period Management, Sub-Ledgers). Needed before period-close
and reconciliation work in E04/E06.
**Files:**
- Create: `internal/domain/entity/{journal.go,period.go}`,
  `internal/domain/service/period_service.go` (domain-level rules only)
**Steps:**
1. `Journal`: groups immutable postings with metadata; all postings share
   tenant, ledger, and period.
2. `Period`: ID, TenantID, LedgerID, Start/End, accounting timezone, Status
   (OPEN/CLOSED); `Close()` requires
   `PeriodOpen` + caller-verified preconditions passed in (unresolved workflow count,
   unresolved break count) — keeps domain pure.
3. Sub-ledger rule helper: `BelongsToSubLedger(entry, dimension)` for
   per-tenant/per-currency/per-entity partitioning (used by E06 queries, E07 partitions).
4. Opening balances are imported only through an approved, balanced posting
   template in an open period. The import records source/evidence, actor, and
   approval; it never writes a balance column or bypasses the normal posting
   transaction.
5. Events: `PeriodOpened`, `PeriodClosed`, `PeriodReopened`.
**Acceptance Criteria:**
- [ ] Posting into a CLOSED period fails `PERIOD_CLOSED`.
- [ ] Journal with mixed tenants/periods fails validation.
- [ ] Opening-balance import requires an open period, balances per asset, and
  retains an evidence/approval reference (test).
- [ ] Unit tests cover open → close → reopen lifecycle.
**Story Points:** 3
**Depends On:** E02-T04
**Related Docs:** `docs/fintech-ledger-features.md §2.3`, `docs/user-journeys.md §2.6`, `docs/domain-events.md §3.8`
**SDD Gate:** G2

---

### E02-T06: Domain events catalog (§3.1–§3.15)
**Status:** pending
**Background:** Every state change in E02-T03–T05 and later E03–E05 must emit a
typed event from `docs/domain-events.md §3`. Payloads must match webhook
payloads in `docs/api-contracts.md §10`.
**Files:**
- Create: `internal/domain/event/{account.go,transaction.go,transfer.go,payment.go,refund.go,payout.go,reconciliation.go,period.go,fx.go,fee.go,report.go,dispute.go,topup.go,capture.go}`
**Steps:**
1. Implement all event structs + payloads in `docs/domain-events.md §3.1–§3.15`
   (account incl. `verified`, payment incl. `requires_action`, `captured`, and
   `settled`,
   dispute opened/closed, topup succeeded/failed, plus `report.generated.v1`).
2. Domain tests verify typed fields and `EventType()` strings exactly
   (`account.created.v1`, `transfer.canceled.v1` — American spelling). Wire/event
   serialization belongs to the E08 adapter and public projections to API tasks.
3. Cross-check script: every webhook in `docs/api-contracts.md §10` maps to an
   event here except none — `tasks/scripts/check-tasks.py --events` enforces it.
**Acceptance Criteria:**
- [ ] `check-tasks.py --events` passes (webhook↔event mapping complete).
- [ ] Aggregate methods emit exactly the documented events (assert uncommitted events in tests).
- [ ] No event struct imports anything outside stdlib + kernel.
**Story Points:** 4
**Depends On:** E02-T01
**Related Docs:** `docs/domain-events.md §2–§3`, `docs/api-contracts.md §10`, `SPEC.md §5.1`
**SDD Gate:** G2

---

### E02-T07: Specifications catalog (all money-flow §8 rules)
**Status:** pending
**Background:** Every rule in `docs/money-flow.md §8` as an executable,
composable spec (`SPEC.md §5.2`). This is the executable domain-rule catalog;
delivery SDD is defined separately by `tasks/SDD.md`.
**Files:**
- Create: `internal/domain/specification/{account.go,transaction.go,transfer.go,refund.go,period.go}` + tests
**Steps:**
1. Implement: `SufficientFunds`, `ValidCurrency`, `AccountActive`,
   `EntryAmountPositive`, `PostingBalancesPerCurrency`, `PostingTemplateAllowed`,
   `RefundWindowValid`, `RefundAmountValid`, `PeriodOpen`, `OriginalExists`,
   `TransferAmountPositive`, `SameTenant`, `CaptureAmountValid`,
   `AllocationExact` (largest-remainder construction rule, money-flow §2.13).
2. Durable idempotency uniqueness/fingerprint equality is an application/database
   invariant, not a repository-aware domain specification.
3. Each spec returns stable error codes (`SPEC.md §9.1`: ACC-*/TXN-*/TRF-*/REF-*/PER-*)
   with structured details (balances, amounts, IDs).
4. Table tests: valid + each invalid dimension; combinator tests (And/Or/Not).
**Acceptance Criteria:**
- [ ] Every row of `money-flow.md §8` has a spec (audit via `check-tasks.py --specs`).
- [ ] `CONCURRENT_TRANSFER` path covered by lock-contention unit test at service level (E03).
- [ ] `go test ./internal/domain/specification/ -race -count=3` passes.
**Story Points:** 4
**Depends On:** E02-T03, E02-T04, E02-T05
**Related Docs:** `SPEC.md §5.2`, `SPEC.md §9.1`, `docs/money-flow.md §8`, `docs/fintech-ledger-features.md §2`
**SDD Gate:** G2

---

### E02-T08: Repository ports (all aggregates)
**Status:** pending
**Background:** Ports the adapters implement (`SPEC.md §3.2`, `§7.10`). Must
cover every aggregate from E02-T03–T05, not just account/transaction.
**Files:**
- Create: `internal/domain/repository/{ledger.go,account.go,posting.go,entry.go,hold.go,journal.go,period.go}`
**Steps:**
1. `AccountRepository`: Create, FindByID, FindByTenant (paginated),
   UpdateMetadata, UpdateStatus — all tenant-scoped, all return errors (no
   panics). Do not expose generic Save/Delete methods.
2. `PostingRepository`: atomic Commit request, FindByID/ExternalReference/Account
   with cursor; repository contract includes posting+entries+checkpoint+outbox.
3. `EntryReader`: FindByPosting and FindByAccount with cursor pagination. There
   is no independently callable entry write/save port; entries persist only via
   the atomic Posting/UnitOfWork commit path.
4. `HoldRepository`, `JournalRepository`, and `PeriodRepository` (FindOpen, FindByDate).
   Durable idempotency is an application-owned port in E06 and implemented in E07.
**Acceptance Criteria:**
- [ ] Every method documents its consistency expectation (strong read vs cacheable).
- [ ] Mock generation works: `make generate-mocks` produces mocks compiling against these ports.
**Story Points:** 2
**Depends On:** E02-T03, E02-T04, E02-T05
**Related Docs:** `SPEC.md §5.1`, `SPEC.md §3.2`, `SPEC.md §7.10`
**SDD Gate:** G2

---

### E02-T09: Domain test suite + external-import guard (G2 gate)
**Status:** pending
**Background:** G2 requires ≥90% coverage and a stdlib-only domain.
**Files:**
- Create: `test/unit/domain/...` (mirrors `internal/domain/...`)
**Steps:**
1. Table tests for every spec × every aggregate method; property tests for Money.
2. Add CI check: `go list -deps ./internal/domain/...` must contain no external modules.
3. Coverage report uploaded as CI artifact.
**Acceptance Criteria:**
- [ ] `go test ./internal/domain/... -race -count=3` passes.
- [ ] Coverage ≥90% (`go test -coverprofile`).
- [ ] External-import guard passes in `gate-check.sh G2`.
**Story Points:** 3
**Depends On:** E02-T02, E02-T03, E02-T04, E02-T05, E02-T06, E02-T07, E02-T08
**Related Docs:** `SPEC.md §10.2`, `SPEC.md §16`
**SDD Gate:** G2

## Acceptance Criteria

- [ ] E02-T01 … E02-T09 all `completed` (count 30 SP in `tasks/tracking/PROGRESS.md`)
- [ ] All specs executable and green; aggregates enforce every money-flow §8 rule
- [ ] Emitted events match webhooks (`check-tasks.py --events`); no external imports in domain
- [ ] Domain unit tests ≥90% with `-race -count=3`
- [ ] SDD gate G2 checks pass — `tasks/tracking/GATES.md#G2`
