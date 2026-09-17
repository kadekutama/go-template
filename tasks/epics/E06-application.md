# Epic E06: Application Layer (Commands, Queries, Ports, Sagas)

**Status:** completed
**Story Points:** 40
**Phase:** 4
**Dependencies:** Task-level E02–E05 contracts; the ledger pilot depends only on its E02 subset
**SDD Gate:** G3 (handlers + sagas tested ≥85% with mocked ports)
**Design refs:** `SPEC.md §6.1–§6.4`, `docs/data-flow.md §2` (handler flows),
`docs/api-contracts.md §7` (every endpoint needs a handler),
`docs/domain-events.md §4.2` (outbox)

> Why this epic is separate: handlers orchestrate approved domain contracts.
> Core ledger ports/use cases can proceed as a vertical slice; each extended
> context waits only for its explicit E03–E05 task dependencies.

## Tasks

### E06-T01: Shared handler plumbing (envelope, idempotency, outbox, translation hook)
**Status:** completed
**Background:** Every command handler repeats: validate → idempotency reserve →
load → execute → persist + outbox → publish → translate errors. Build once
(data-flow §2 command flow).
**Files:**
- Create: `internal/application/{command/handler.go,query/handler.go}`,
  `internal/application/middleware/{idempotency.go,translation.go,authz.go}`
**Steps:**
1. Define generic `CommandHandler[C, R]` / `QueryHandler[Q, R]` interfaces (or
   concrete consumer-owned interfaces where a generic adds no value); use
   ordinary Go `(R, error)` returns. A Result/monad wrapper is not part of the
   baseline contract.
2. Idempotency middleware computes a canonical request fingerprint and calls the
   durable `IdempotencyStore`: reserve/lease, replay the stored response on the
   same hash, conflict on a different hash. Valkey may be a read-through hint only.
3. Unit-of-work port atomically bundles posting/state, idempotency result, balance
   checkpoint, and outbox write (E07 honors it).
4. Error translation hook: code → go-i18n message using context locale (data-flow §2.1 step 8).
5. AuthZ check helper (Casbin port) applied before execution.
**Acceptance Criteria:**
- [ ] A sample handler (CreateAccount) uses all five middlewares (test with fakes).
- [ ] Double-submit of same idempotency key returns the original result, executes once (test).
**Story Points:** 3
**Depends On:** E06-T06
**Related Docs:** `SPEC.md §6.1–§6.2`, `docs/data-flow.md §2`, `docs/domain-events.md §4.2`, `SPEC.md §9.1–§9.3`
**SDD Gate:** G3

---

### E06-T02: Account + tenant commands/queries
**Status:** completed
**Background:** Backs api-contracts account endpoints + tenant provisioning (journeys §2.1).
**Files:**
- Create: `internal/application/command/{account.go,tenant.go}`,
  `internal/application/query/{account.go,tenant.go}`,
  `internal/application/dto/{account.go,tenant.go}`
**Steps:**
1. Commands: Open/Update/Freeze/Unfreeze/CloseAccount, ProvisionTenant, UpdateTenantSettings.
2. Queries: Get/ListAccounts (cursor+offset, filters), GetBalance (all 5 types), GetEntries (cursor), GetStatement, GetTenant.
3. DTOs map 1:1 to api-contracts §7 shapes (accounts + tenants groups).
**Acceptance Criteria:**
- [ ] Every account/tenant endpoint in api-contracts §7 has a handler (audit via `check-tasks.py --handlers`).
- [ ] Handlers tested with mocked ports (valid/invalid/boundary).
**Story Points:** 3
**Depends On:** E06-T01, E06-T12, E05-T01
**Related Docs:** `docs/api-contracts.md §7` (accounts, tenants), `docs/user-journeys.md §2.1`, `tasks/epics/E02-ledger-domain.md#E02-T03`
**SDD Gate:** G3

---

### E06-T03: Transaction + transfer commands/queries (incl. scheduled/bulk)
**Status:** completed
**Background:** Backs transfers §7.5 incl. new `execute_at`/`recurrence` fields
and batch endpoints added during verification.
**Files:**
- Create: `internal/application/command/{transaction.go,transfer.go,batch_transfer.go}`,
  `internal/application/query/{transaction.go,transfer.go}`,
  `internal/application/dto/{transaction.go,transfer.go,batch.go}`
**Steps:**
1. PostTransaction, ReverseTransaction, CreateTransfer (immediate + scheduled fields),
   CancelTransfer, CreateBatchTransfer, GetBatchStatus.
2. Batch: intake validation → persist batch + items → emit `transfer.batch.received.v1`;
   item execution reuses single-transfer handler with `{batch}:{index}` keys.
3. Queries: Get/List with filters (account, dates, status, type, amount range), cursor pagination.
**Acceptance Criteria:**
- [ ] Batch intake rejects >1000 items and mixed-tenant items (tests).
- [ ] Scheduled transfer persists PENDING without funds check; execution path enforces it (tests).
- [ ] Protocol-parity note honored: DTOs carry everything gRPC/GraphQL need (review).
**Story Points:** 4
**Depends On:** E06-T01, E06-T12, E03-T01
**Related Docs:** `docs/api-contracts.md §7` (transactions, transfers), `docs/money-flow.md §2.3, §2.8, §2.9`, `docs/domain-events.md §3.2–§3.3`
**SDD Gate:** G3

---

### E06-T04: Payment, refund, payout commands/queries
**Status:** completed
**Background:** Backs api-contracts payments/refunds/payouts; drives payment-processor
and settlement integrations (E10).
**Files:**
- Create: `internal/application/command/{payment.go,refund.go,payout.go}`,
  `internal/application/query/{payment.go,refund.go,payout.go}`,
  `internal/application/dto/{payment.go,refund.go,payout.go}`
**Steps:**
1. Payment intents: create/confirm/cancel; confirm delegates charging to the
   payment-processor port (E10 implements) then posts ledger entries on success event.
   SCA challenges surface `requires_action` (402 `card_error` mapping at the edge);
   capture calls E03-T08 rules (full/partial/expiry).
2. Refunds: create with window/amount validation; links reversal to original.
3. Payouts: create/cancel; two-stage settlement tracked; overdue rule surfaced;
   schedule/minimum/instant honored from E03-T03 and E03-T11 policy, with
   `PAYOUT_BLOCKED` reason details and no provider submission when ineligible.
4. Top-ups: create/cancel/status via E03-T10; unverified accounts rejected.
5. Queries + filters per api-contracts §7 (original_txn, dates, status, method).
**Acceptance Criteria:**
- [ ] Processor failure leaves intent PENDING with retry-safe idempotency (test with fake processor).
- [ ] Webhook payloads for `payment_intent.*`, `refund.*`, `payout.*` match api-contracts §10 (test fixtures).
**Story Points:** 4
**Depends On:** E06-T01, E06-T12, E03-T02, E03-T03, E03-T06, E03-T11
**Related Docs:** `docs/api-contracts.md §7` (payments, refunds, payouts), `docs/money-flow.md §2.1–§2.2, §2.4`, `docs/user-journeys.md §2.4–§2.5`
**SDD Gate:** G3

---

### E06-T05: Reconciliation, period, report, compliance commands/queries
**Status:** completed
**Background:** Backs ops endpoints + compliance workflows (features §4, §6).
**Files:**
- Create: `internal/application/command/{reconciliation.go,period.go,report.go,compliance.go}`,
  `internal/application/query/{reconciliation.go,period.go,report.go,compliance.go}`
**Steps:**
1. Reconciliation: trigger run, resolve/acknowledge break (with SoD check passthrough).
2. Period: open/close/reopen; close runs E04-T03 validation and returns ALL failures.
3. Reports: generate (template+params+delivery → signed URL + `report.generated.v1`),
   status, templates list. Covers all 10 report types in features §6 —
   financial: trial balance, balance sheet, income statement, cash flow statement,
   general ledger detail, account statements; operational: transaction volume,
   settlement, fee revenue, exception reports.
4. Compliance: run screening review, record decision, export regulatory report fields.
**Acceptance Criteria:**
- [ ] All 10 features-§6 report types generatable (parameterized test).
- [ ] Period close returns every failing check, not first-only (test).
- [ ] Break resolve enforces resolver≠approver above threshold (test with fakes).
**Story Points:** 4
**Depends On:** E06-T01, E06-T12, E04-T01, E04-T02, E04-T03, E04-T05
**Related Docs:** `docs/api-contracts.md §7` (reconciliation, periods, reports), `docs/fintech-ledger-features.md §4, §6`, `docs/user-journeys.md §2.3, §2.6`
**SDD Gate:** G3

---

### E06-T06: Core ledger integrity ports
**Status:** completed
**Background:** Define the smallest stable contracts needed to post and read a
ledger without waiting for every payment, compliance, identity, and provider port.
**Files:**
- Create: `internal/application/port/{ledger.go,unit_of_work.go,idempotency.go,event.go,clock.go,ids.go,authz.go}`
**Steps:**
1. Define inbound `PostLedgerPosting`, `GetPosting`, `GetBalance`, and `ListEntries`.
2. Define `UnitOfWork` so posting, entries, checkpoints, durable idempotency result,
   and outbox commit atomically; define strong-read and cursor expectations.
3. Define durable `IdempotencyStore`, event writer/publisher boundary, `Clock`,
   `IDGenerator`, and command-level `Authorizer` without adapter types.
4. Document retry, unknown commit outcome, tenant/ledger scope, and transaction ownership.
**Acceptance Criteria:**
- [ ] Core port mocks compile without importing infrastructure packages.
- [ ] One contract test proves the UnitOfWork boundary cannot persist a partial posting.
**Story Points:** 2
**Depends On:** E02-T08
**Related Docs:** `SPEC.md §6`, `SPEC.md §7.10`, `docs/data-flow.md §2`, `docs/ledger-core.md §3, §8`
**SDD Gate:** G3

---

### E06-T07: Sagas (transfer, refund, payout-settlement, reconciliation, period-close, batch)
**Status:** completed
**Background:** Long-running orchestrations with compensation (`SPEC.md §6.4`,
money-flow §10 failure table).
**Files:**
- Create: `internal/application/workflow/{transfer.go,refund.go,payout_settlement.go,reconciliation.go,period_close.go,batch.go}` + `compensate.go`
**Steps:**
1. Each saga: steps with idempotent actions, per-step timeouts, retry policies,
   compensation (e.g., payout-settlement failure → hold + notify, never partial ledger write).
2. State persisted (saga record) so cron/workers can resume after crash.
3. Failure table from money-flow §10 encoded as tests (DB crash mid-txn, NATS down, processor timeout, stale FX, duplicate submit).
**Acceptance Criteria:**
- [ ] Crash-resume test: kill mid-saga, resume/retry yields one durable effect per step key.
- [ ] Every money-flow §10 row has a saga test.
**Story Points:** 3
**Depends On:** E06-T03, E06-T04, E06-T05
**Related Docs:** `SPEC.md §6.4`, `docs/money-flow.md §10`, `docs/user-journeys.md §4`
**SDD Gate:** G3

---

### E06-T08: Application test suite (G3 gate)
**Status:** completed
**Background:** G3 requires ≥85% with mocked ports.
**Files:**
- Create: `test/unit/application/...`
**Steps:**
1. Table tests per handler (valid/invalid/boundary/auth-denied/idempotent-replay).
2. Saga tests with in-memory fakes incl. crash-resume.
3. Coverage upload as CI artifact.
**Acceptance Criteria:**
- [ ] `go test ./internal/application/... -race -count=3` passes, coverage ≥85%.
**Story Points:** 2
**Depends On:** E06-T02, E06-T03, E06-T04, E06-T05, E06-T07
**Related Docs:** `SPEC.md §10.2`, `SPEC.md §16`
**SDD Gate:** G3

---

### E06-T09: Extended reporting (reconciliation, regulatory, scheduled delivery, dashboard aggregations)
**Status:** completed
**Background:** E06-T05 covers the 10 named report types. This task covers the
remaining reporting surface from features §4.2/§6: reconciliation reports,
regulatory generation (1099/FATCA/CRS/call reports using E04-T05 field defs),
scheduled report delivery, and dashboard aggregation queries (volume/fees by
period for live charts).
**Files:**
- Create: `internal/application/command/extended_reports.go`,
  `internal/application/query/{recon_reports.go,regulatory.go,dashboard.go}`,
  `internal/application/dto/extended_reports.go`
**Steps:**
1. Reconciliation reports: runs/breaks/resolutions over a period (reads E04 state, no new writes).
2. Regulatory generation: render E04-T05 field definitions to CSV/PDF; each report type succeeds on fixture data.
3. Scheduled delivery: report subscriptions (template + cron + destination) executed by E14-T02's report job; deliver via signed URL + `report.generated.v1`.
4. Dashboard aggregations: volume/fees/exceptions grouped by day/week/tenant with bounded result sizes.
**Acceptance Criteria:**
- [ ] 1099, FATCA, CRS, and call-report outputs validate against their field definitions (tests).
- [ ] Scheduled delivery fires on cron and notifies via webhook event (test with fakes).
- [ ] Aggregation queries paginate/bound results (no unbounded scans — test with large fixture).
**Story Points:** 4
**Depends On:** E06-T05, E04-T01, E04-T05
**Related Docs:** `docs/fintech-ledger-features.md §4.2, §6`, `docs/api-contracts.md §7` (reports), `docs/domain-events.md §3.11`
**SDD Gate:** G3

---

### E06-T10: Tenant usage metering + billing export
**Status:** completed
**Background:** Features §7.1 lists tenant "billing" (P1), but no task metered
anything. Scope: record billable usage and export invoice-ready data; actual
charging stays with an external billing provider fed by webhook/export.
**Files:**
- Create: `internal/application/service/billing_service.go`,
  `internal/application/dto/billing.go`
**Steps:**
1. Meter billable events (transactions posted, API calls, payouts, reports generated)
   into per-tenant daily usage records (idempotent on event ID).
2. Billing export: per-tenant per-period CSV (usage lines + fee-revenue input for reports).
3. Publish usage summaries consumed by the fee-revenue operational report.
**Acceptance Criteria:**
- [ ] Replayed events don't double-count usage (idempotency test).
- [ ] Export for a fixture tenant matches hand-computed totals (test).
**Story Points:** 3
**Depends On:** E06-T01, E06-T12
**Related Docs:** `docs/fintech-ledger-features.md §6.2` (fee revenue), `docs/fintech-ledger-features.md §7.1` (tenants)
**SDD Gate:** G3

---

### E06-T11: Dispute commands/queries
**Status:** completed
**Background:** Application face of E03-T07 for api-contracts §7.11.
**Files:**
- Create: `internal/application/command/dispute.go`,
  `internal/application/query/dispute.go`,
  `internal/application/dto/dispute.go`
**Steps:**
1. Commands: OpenDispute, SubmitEvidence, Represent, CloseDispute (as lost).
2. Queries: GetDispute (with deadline + fee), ListDisputes (status/date filters).
3. SoD passthrough: resolutions above threshold require a different approver (E04-T02 rule surfaced, not re-implemented).
**Acceptance Criteria:**
- [ ] Evidence after deadline rejected; second representment rejected (tests).
- [ ] Webhook payloads for `dispute.*` match api-contracts §10 (test fixtures).
**Story Points:** 3
**Depends On:** E06-T01, E06-T12, E03-T07
**Related Docs:** `docs/api-contracts.md §7` (disputes), `docs/user-journeys.md §2.7`, `docs/domain-events.md §3.12`
**SDD Gate:** G3

---

### E06-T12: Extended workflow and adapter ports
**Status:** completed
**Background:** Payment, compliance, tenancy, security, provider, cache, worker,
and notification contracts change at a different rate from the ledger kernel and
must not block its first vertical proof.
**Files:**
- Create: `internal/application/port/{usecase_account.go,usecase_transfer.go,usecase_payment.go,usecase_compliance.go,usecase_tenant.go,repository_query.go,payment_processor.go,fx_provider.go,secrets.go,cache.go,ratelimiter.go,aml.go,storage.go,lock.go,token.go,oauth.go,apikey.go,crypto.go,audit.go,statements.go,notify.go,webhooks.go,consumer.go}`
**Steps:**
1. Define one inbound use-case contract per non-core bounded context.
2. Define provider/security/cache/messaging ports with timeout, retry,
   idempotency, consistency, and sensitive-data rules.
3. Keep provider SDK types out of application and domain signatures.
4. Ensure every E07–E10 adapter task names the exact port it implements.
**Acceptance Criteria:**
- [ ] `make generate-mocks` produces compiling mocks for all extended ports.
- [ ] `check-tasks.py --ports` confirms every E07–E10 adapter task names its port.
- [ ] Contract review finds no infrastructure/provider type crossing the boundary.
**Story Points:** 2
**Depends On:** E06-T06, E03-T01, E04-T01, E05-T03
**Related Docs:** `SPEC.md §6`, `SPEC.md §7.10`, `docs/data-flow.md §2`
**SDD Gate:** G3

---

### E06-T13: Core posting and strong-balance use cases
**Status:** completed
**Background:** The first vertical slice needs one real financial write/read path
before scheduled transfers, reports, provider sagas, and three public protocols.
**Files:**
- Create: `internal/application/command/posting.go`,
  `internal/application/query/{posting.go,balance.go,entries.go}`,
  `internal/application/dto/{posting.go,balance.go,entry.go}`
**Steps:**
1. Implement restricted PostLedgerPosting using an approved posting template,
   durable idempotency fingerprint, command authorization, and one UnitOfWork.
2. Implement GetPosting, strongly consistent GetBalance, and cursor-based ListEntries.
3. Return the original response for an identical idempotent replay and conflict
   for a changed fingerprint; surface unknown commit outcome without blind retry.
4. Test concurrent attempts against fakes that model the port contracts; E07
   proves database serialization.
**Acceptance Criteria:**
- [ ] Happy path, invalid template, unbalanced asset lot, duplicate replay,
  fingerprint conflict, authorization denial, and unknown-outcome scenarios pass.
- [ ] DTOs expose integer minor units, asset code, as-of time, and ledger cursor.
- [ ] No workflow state is written to Posting or Entry.
**Story Points:** 3
**Depends On:** E06-T01, E02-T04, E02-T07, E02-T08
**Related Docs:** `docs/ledger-core.md §3–§9`, `docs/data-flow.md §2`, `docs/api-contracts.md §7`
**SDD Gate:** G3

## Acceptance Criteria

- [x] E06-T01 … E06-T13 all `completed` (count 40 SP in `tasks/tracking/PROGRESS.md`)
- [x] Every api-contracts §7 endpoint group has handlers (`check-tasks.py --handlers`)
- [x] Every money-flow pattern has a saga path; ports complete for E07–E10
- [x] Application tests ≥85% with `-race -count=3`
- [x] SDD gate G3 checks pass — `tasks/tracking/GATES.md#G3`
