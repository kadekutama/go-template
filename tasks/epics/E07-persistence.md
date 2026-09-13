# Epic E07: Persistence Adapters (Postgres, Migrations, RLS, Seed, Outbox)

**Status:** pending
**Story Points:** 33
**Phase:** 5 (parallel with E08, E09, E10)
**Dependencies:** Task-level E01–E06 contracts; E07-T01 ledger core precedes extended schema/RLS
**SDD Gate:** G4
**Design refs:** `SPEC.md §7.2`, `docs/data-flow.md §5–§7`, `docs/domain-events.md §4.2`,
`docs/user-journeys.md §2.1` (onboarding seed), `docs/fintech-ledger-features.md §11.3` (PITR)

> Why a separate epic: persistence is where tenant isolation, atomicity, and
> auditability are proven — schema, RLS, outbox, seed, and backup live together
> so no adapter ships without its safety properties.

## Tasks

### E07-T01: Ledger-core schema and explicit posting transaction
**Status:** pending
**Background:** Establish the financial integrity boundary first: ledgers,
accounts, postings, entries, holds, checkpoints, durable idempotency, and outbox.
Workflow/reporting tables are split into E07-T10 so this task is reviewable.
**Files:**
- Create: `internal/infrastructure/database/postgres/{models/ledger*.go,repositories/ledger*.go,posting/*.go}`,
  `internal/infrastructure/database/migration/versions/*.sql` (up/down pairs),
  `internal/infrastructure/database/migration/{migrate.go,embed.go}`
**Steps:**
1. Resolve ADR-011 with the required benchmark plan before selecting a balance
   materialization strategy. Core models implement `docs/data-flow.md §5` and
   `docs/ledger-core.md`: `BIGINT` minor-unit entry amounts, `NUMERIC(38,0)`
   aggregates, tenant+ledger composite scope, positive-entry checks, asset
   registry, monotonic sequences/cursors.
2. Migrations: one version per core group, reversible up/down, embedded via `embed.FS`.
3. `make migrate-up|down|create|version|force` wired to golang-migrate v4.19.1.
4. Use explicit SQL/stored procedure for the posting path: reserve durable
   idempotency request hash, lock accounts deterministically, enforce same
   tenant/ledger and per-asset balance, insert posting/entries/checkpoints/outbox,
   store response, commit once. GORM may handle non-critical metadata CRUD.
5. Database roles/constraints reject direct unbalanced inserts and UPDATE/DELETE
   of posted facts; reversals are new linked postings.
6. Implements: repository and unit-of-work ports from E02-T08/E06-T06.
**Acceptance Criteria:**
- [ ] `make migrate-up` from zero builds the full schema; `make migrate-down` removes it cleanly.
- [ ] Migration files numbered with zero gaps (`check-tasks.py --migrations`).
- [ ] Entry amounts are checked `BIGINT` minor units, aggregates use
  `NUMERIC(38,0)`, and no float/ambiguous decimal money columns exist.
- [ ] Direct SQL attempts to insert an unbalanced posting or mutate/delete a
  posted row fail; per-currency balance is database-enforced.
- [ ] ADR-011 is accepted with workload, latency, contention, rebuild, and
  read-after-write evidence before this task is marked implementation-ready.
**Story Points:** 5
**Depends On:** E02-T08, E06-T06
**Related Docs:** `SPEC.md §7.2`, `SPEC.md §2` (GORM v1.31.2, migrate v4.19.1), `docs/data-flow.md §5`, `docs/architecture/ADR-011-balance-materialization.md`
**SDD Gate:** G4

---

### E07-T02: Row-Level Security + tenant-scoped queries
**Status:** pending
**Background:** Implements the isolation contract from E05-T03: shared DB, hard tenant boundaries.
**Files:**
- Create: `internal/infrastructure/database/postgres/rls/*.sql` (policies),
  modify: all repositories to `SET LOCAL app.current_tenant`
**Steps:**
1. RLS policies per the E05-T03 matrix (tenant×table×operation), incl. service-role bypass for migrations.
2. Every repository method sets tenant from context; missing tenant → error (never all-rows).
3. Connection-pool config from E01-T03 (max_open/idle/lifetime).
4. Implements: no new port — enforces tenant scoping inside every `*Repository` adapter (ports: E02-T08).
**Acceptance Criteria:**
- [ ] Cross-tenant read returns zero rows in integration test (adversarial test).
- [ ] Missing tenant in context fails closed (test).
**Story Points:** 3
**Depends On:** E07-T01, E07-T10, E05-T03
**Related Docs:** `docs/data-flow.md §3`, `tasks/epics/E05-tenancy.md#E05-T03`, `docs/fintech-ledger-features.md §5`
**SDD Gate:** G4

---

### E07-T03: Transactional outbox publisher
**Status:** pending
**Background:** Atomic event publishing per `docs/domain-events.md §4.2` — event
persisted iff the transaction commits.
**Files:**
- Create: `internal/infrastructure/database/postgres/outbox/{store.go,poller.go}`
**Steps:**
1. `outbox_events` envelope writer in the same transaction as the state change.
2. Poller claims rows with `FOR UPDATE SKIP LOCKED`, publishes through E08, and
   marks success; retry with backoff and alert. Explicit aggregate/partition
   sequences define order; timestamp/commit order does not.
3. Dedupe guards: event ID plus `(aggregate_id, aggregate_version)` uniqueness.
4. Implements: publishes through the `EventPublisher` port (contract: E06-T06); storage half is internal to this task.
**Acceptance Criteria:**
- [ ] Kill mid-publish → event is redelivered after restart; E08 durable inbox
  proves the consumer's side effect occurs effectively once.
- [ ] Ordering per aggregate preserved (sequence test).
**Story Points:** 3
**Depends On:** E07-T01
**Related Docs:** `docs/domain-events.md §4.2`, `docs/data-flow.md §2`
**SDD Gate:** G4

---

### E07-T04: Seed data (dev tenant, chart of accounts, fixtures)
**Status:** pending
**Background:** Onboarding journey §2.1 and sandbox DX (features §12) need a
one-command seeded environment; tests need deterministic fixtures.
**Files:**
- Create: `internal/infrastructure/database/seed/{dev.go,fixtures.go}`,
  `scripts/db/{seed.sh,reset.sh}`
**Steps:**
1. Dev seed: tenant + default chart (operating/fee/suspense per USD/EUR/IDR) +
   demo users + API keys + sample transactions covering every money-flow §2 pattern.
2. `make db-seed`, `make db-reset` (drop + migrate + seed).
3. Test fixtures builder package reused by `test/fixtures/`.
4. No port: dev/test tooling — drives the repository ports directly, implements none.
**Acceptance Criteria:**
- [ ] Fresh `make db-reset && make db-seed` yields a tenant that passes onboarding journey §2.1 end-to-end.
- [ ] Seed is idempotent (run twice, same result).
**Story Points:** 3
**Depends On:** E07-T01, E07-T02, E07-T10
**Related Docs:** `docs/user-journeys.md §2.1`, `docs/fintech-ledger-features.md §12`, `docs/money-flow.md §2`
**SDD Gate:** G4

---

### E07-T05: Backup/restore + PITR drill hooks
**Status:** pending
**Background:** Features §11.3 (PITR, RPO<5min) and §7 retention need executable
backup paths, not just docs.
**Files:**
- Create: `scripts/db/{backup.sh,restore.sh,verify-restore.sh}`
**Steps:**
1. `backup.sh`: `pg_dump` (custom format) + WAL archive pointer → object storage (MinIO locally, S3 in prod).
2. `restore.sh`: restore to a scratch DB; `verify-restore.sh`: row counts, per-asset
   balance, posting immutability constraints, and full checkpoint/cursor recomputation.
3. Document RPO/RTO measurement in E18 runbook (this task provides the scripts).
4. No port: ops scripts — talk to Postgres/object storage directly, implement none.
**Acceptance Criteria:**
- [ ] Backup → restore → verify passes in CI weekly job (E17 wires the schedule).
- [ ] Double-entry sums match before/after restore (check in verify script).
**Story Points:** 3
**Depends On:** E07-T01
**Related Docs:** `docs/fintech-ledger-features.md §11.3`, `docs/data-flow.md §7`, `SPEC.md §12`
**SDD Gate:** G4

---

### E07-T06: Persistence integration tests (G4 slice)
**Status:** pending
**Background:** G4 evidence for this adapter family.
**Files:**
- Create: `test/integration/persistence/...`
**Steps:**
1. Testcontainers Postgres 18: CRUD per repo, migration up/down/force, RLS adversarial,
   outbox atomicity/redelivery, durable idempotency fingerprint/replay, direct-SQL
   invariant attacks, and concurrent spend serialization with deterministic locks.
2. Run with `-race -count=3`.
**Acceptance Criteria:**
- [ ] All green; deadlock retry covered where applicable.
**Story Points:** 4
**Depends On:** E07-T01, E07-T02, E07-T03, E07-T09, E07-T10
**Related Docs:** `SPEC.md §10.3`, `SPEC.md §12`
**SDD Gate:** G4

---

### E07-T07: Read replicas + replication-lag monitoring + read routing
**Status:** pending
**Background:** Features §11.3 requires an async replica for DR, and §14 wants
active-active reads. Writes go to the primary; cacheable reads may use replicas
within a staleness bound.
**Files:**
- Create: `internal/infrastructure/database/postgres/{replica.go,lag.go}`,
  modify: repository constructors to accept a read pool
**Steps:**
1. Async replica configuration (compose + K8s manifests reference E17-T03/T04; this task owns the client side).
2. `replication_lag_seconds` metric + alert threshold; reads fall back to primary when lag exceeds bound.
3. Read/write split: queries flagged read-only in E06 use the replica pool; everything else uses primary.
4. Implements: no new port — extends the `*Repository` adapters (ports: E02-T08) with read-pool selection.
**Acceptance Criteria:**
- [ ] Lag beyond bound routes reads back to primary automatically (test with fake lag source).
- [ ] Lag metric + alert rule present (review vs E15-T02).
**Story Points:** 3
**Depends On:** E07-T01
**Related Docs:** `docs/fintech-ledger-features.md §11.3, §14`, `SPEC.md §12`
**SDD Gate:** G4

---

### E07-T08: Per-tenant data-residency routing
**Status:** pending
**Background:** E05-T03 defined per-tenant region/db pointers "enforced in E07" —
this is that enforcement. A tenant pinned to a region must never read/write elsewhere.
**Files:**
- Create: `internal/infrastructure/database/postgres/router.go`
**Steps:**
1. Resolve connection pool from the tenant's residency pointer at request scope (context).
2. Unknown/unconfigured region fails closed (error, never default-region fallback).
3. Residency recorded in audit entries for data-movement proof.
4. Implements: no new port — router inside the `*Repository` adapters (ports: E02-T08).
**Acceptance Criteria:**
- [ ] Request for an EU-pinned tenant never touches the US pool (adversarial test with pool spies).
- [ ] Missing residency config fails closed (test).
**Story Points:** 2
**Depends On:** E05-T03, E07-T01, E07-T10
**Related Docs:** `docs/fintech-ledger-features.md §5`, `tasks/epics/E05-tenancy.md#E05-T03`
**SDD Gate:** G4

---

### E07-T09: Shared Testcontainers harness and deterministic fixtures
**Status:** pending
**Background:** Integration suites must not invent container lifecycle and
fixtures independently. The previous plan placed this harness after E07's tests,
creating an impossible consume-before-build dependency.
**Files:**
- Create: `test/testcontainers/{postgres.go,valkey.go,nats.go,unleash.go,minio.go,maildev.go,harness.go}`,
  `test/fixtures/{tenant.go,accounts.go,postings.go,workflows.go,statements.go}`
**Steps:**
1. Provide one start/wait-ready/connection/terminate helper per dependency with
   pinned image references and bounded startup contexts.
2. Add minimal ledger fixtures first; extended workflow/provider fixtures are
   additive and must reuse stable IDs/builders.
3. Support a CI profile with reduced resources and a local debug profile; every
   test registers cleanup and can run repeatedly without leaked containers.
4. Expose reusable package APIs only; integration suites remain owned by their
   feature tasks.
**Acceptance Criteria:**
- [ ] A harness smoke test boots PostgreSQL, Valkey, and NATS and cleans them up.
- [ ] Two parallel test packages use isolated databases/streams and stable fixtures.
- [ ] E07/E08/E10 integration-test tasks depend on and reuse this harness.
**Story Points:** 4
**Depends On:** E01-T01, E01-T03
**Related Docs:** `SPEC.md §10.3`, `SPEC.md §12`, `tasks/SDD.md §5`
**SDD Gate:** G4

---

### E07-T10: Workflow, tenancy, reconciliation, and control-plane schema
**Status:** pending
**Background:** Non-ledger state has different mutability and rollout rules from
immutable financial facts. It is migrated separately while retaining tenant and
ledger references to the core schema.
**Files:**
- Create: `internal/infrastructure/database/postgres/{models/workflow*.go,models/tenant*.go,models/reconciliation*.go,models/control*.go,repositories/workflow*.go}`,
  additional `internal/infrastructure/database/migration/versions/*.sql` up/down pairs
**Steps:**
1. Add tenants, payment/refund/payout/dispute workflows, scheduled/batch
   transfers, immutable reconciliation sources/match groups/breaks, periods,
   reports, approvals, inbox receipts, and audit references.
2. Define lifecycle mutation/version rules per aggregate; no workflow status or
   provider payload may mutate Posting/Entry rows.
3. Add foreign/composite keys preserving tenant+ledger scope and explicit
   retention/partition indexes for operational tables.
4. Implement extended repository/query ports from E06-T12.
**Acceptance Criteria:**
- [ ] Up/down migrations work independently after the core schema.
- [ ] Schema tests reject cross-tenant references and workflow-to-posting mutation.
- [ ] Immutable provider/reconciliation source hashes and parser versions are retained.
**Story Points:** 3
**Depends On:** E06-T12, E03-T02, E03-T03, E04-T01, E05-T03, E07-T01
**Related Docs:** `docs/data-flow.md §5`, `docs/ledger-core.md §1, §10`, `SPEC.md §7.2`
**SDD Gate:** G4

## Acceptance Criteria

- [ ] E07-T01 … E07-T10 all `completed` (count 33 SP in `tasks/tracking/PROGRESS.md`)
- [ ] Schema migrates cleanly both directions (up/down/force)
- [ ] Tenant isolation adversarial-tested; outbox at-least-once + durable inbox effectively-once effects proven
- [ ] SDD gate G4 checks pass — `tasks/tracking/GATES.md#G4`
