# ADR-019: DB-Generated UUIDv7 Identity with Composite Tenant PKs

**Status:** Accepted  
**Date:** 2026-09-17  
**Approved:** repository-owner 2026-09-17 (composite UUID PK design; industry-standard track)  
**Note:** Accepted for implementation via E07.1-T06 + E06-T14. Pre-release one-time re-baseline: no production or staging database exists (E17 has not deployed), so history is rewritten once with rationale instead of migrated.

## Context

Ledger entities currently use app-assigned `TEXT` primary keys (UUIDv7
strings minted via the `IDGenerator` port). Production measurement on Citus
14.0 (E07.1-T05 evidence) proved only `idempotency_records` — whose primary
key already covers `tenant_id` — distributes; every other core table defers
with `referenced table must be a distributed table`, `cannot distribute
because it has triggers`, or `cannot create constraint`. Citus requires the
distribution column inside every unique constraint, so bare `PRIMARY KEY
(id)` permanently blocks tenant sharding no matter the ID format.

PostgreSQL 18 ships native `uuidv7()` (verified: time-ordered, version
nibble 7, no extension). Replay safety in this codebase lives in the
idempotency-key layer (`IDEMPOTENCY_KEY_REQUIRED` + stored responses), not in
client-supplied entity IDs, and provider-facing calls already key on the
idempotency key (`PayoutReference{IdempotencyKey}`) — so DB-generated entity
IDs remove no safety mechanism. The owner additionally approved breaking
changes with full DB resets at this pre-release stage.

## Decision

1. **Native UUID identity, DB-generated**: entity `id` columns become `UUID
   NOT NULL DEFAULT uuidv7()`. `DEFAULT` preserves explicit inserts, so
   deterministic fixtures/tests keep stable IDs by switching their literals
   to canonical UUID strings.
2. **Composite tenant-first primary keys** on tenant-scoped tables:
   `PRIMARY KEY (tenant_id, id)` for ledgers, accounts, postings, entries,
   holds (and tenant-first ordering for checkpoints). Every foreign key
   carries `tenant_id` alongside the referenced id. `tenant_id` columns
   become `UUID` type to match; RLS tenant comparisons cast
   `current_setting('app.current_tenant', true)::uuid`.
3. **Human slugs via alias columns**: `tenants.alias TEXT NOT NULL UNIQUE`
   (globally unique) and `ledgers.alias TEXT NOT NULL` with `UNIQUE
   (tenant_id, alias)`; display names stay free-form and non-unique.
   Accounts keep `number`, postings keep `external_reference`. Alias
   population (slugify + numeric-suffix retry) belongs to E06-T14.
4. **Integer cursors retained**: `ledger_seq`, `outbox_events.id`,
   `audit_refs.id` stay `BIGSERIAL` — ordering is their purpose, and UUIDs
   would destroy poll/cursor semantics. `idempotency_records`,
   `inbox_receipts`, and the asset registry are unchanged in shape.
5. **Domain untouched**: value objects already enforce canonical UUID strings
   (`validUUID`), so the domain needs no change; repositories map
   string↔UUID at the port boundary (GORM `type:uuid` string fields with
   `RETURNING` proof; `uuid.UUID` model type is the documented fallback).
6. **One-time pre-release re-baseline**: baseline files rewritten with
   `UUID` identity (see E07.1-T06); `atlas.sum` is regenerated. Goose
   starts empty on fresh databases; the legacy bootstrap stays dormant
   for safety. History immutability (ADR-018) resumes immediately after:
   this is the single sanctioned exception, justified by zero deployed
   databases.

## Amendment 2026-09-17 (owner directive: no incrementing names)

The `000001`–`000004` numeric prefixes were renamed to timestamped
`20260901000001`–`20260901000004` (order-preserving, ahead of the Citus
`...005` file), superseding ADR-018's number-preservation rule for the same
zero-deployment reason. All migrations — past and future — are now
14-digit UTC timestamps; the validator rejects bad names, legacy gaps, and
duplicate versions. Per-version Up/Down reversibility is unchanged
(rename, not squash).

## Real-World Scenarios Covered

- **Replayed payout**: duplicate request hits the same idempotency key,
  returns the stored response with the DB-generated payout ID; no second
  posting exists to reconcile.
- **Tenant sharding verification**: `accounts`, `holds`, `ledgers`, and
  `idempotency_records` distribute on `tenant_id` (composite PKs satisfy
  Citus); `postings`/`entries` remain trigger-blocked and `outbox_events`
  constraint-blocked under the resilient path — each with a named follow-up,
  none failing migration.
- **Stale deployment detection**: unchanged (`version` + lifecycle logs).

## Consequences

- Application stops pre-minting entity IDs (`~15 NewID sites` audited in
  E06-T14); handlers read back `RETURNING` IDs. Any flow handing an entity
  ID to an external call must switch to the idempotency key first.
- Fixture/test literals (`tnt-test-01`, …) become canonical UUID strings;
  constant names are preserved so consumers do not churn.
- `IDGenerator` port stays (event IDs, keys, non-persisted correlation);
  only entity persistence moves to DB generation.
- Reference-table and trigger interactions keep the resilient NOTICE +
  WARNING posture from E07.1-T01; the summary block now counts 7+ tables.

## Measured Citus Limitation (2026-09-17, two-node cluster evidence)

Composite PKs are necessary but NOT sufficient for distribution in this
FK-dense schema — verified order-independent on Citus 14.0:

- `idempotency_records` (no FKs at all) is the only ledger table that
  distributes.
- `create_reference_table('assets')` succeeds alone, but then every later
  distribution fails: Citus auto-adds FK-connected locals to metadata, and
  FKs from local/reference tables to distributed tables are forbidden
  (`cannot modify table "ledgers" because there was a parallel operation on
  a distributed table`).
- Reference-first ordering fails symmetrically: distributing `ledgers` after
  the reference step hits the same local→distributed FK veto from the
  auto-added tables; `accounts`/`holds` additionally fail colocated-FK
  checks, `postings`/`entries` fail on triggers, `outbox_events` on its
  non-tenant unique.
- Conclusion: real sharding needs FK-constraint redesign (drop FKs with
  application enforcement, or co-located FK coverage including `tenant_id`)
  plus a trigger-safe distribution procedure — a dedicated follow-up, NOT
  this task. The composite-PK work stands on its own merits (native type
  safety, tenant-scoped uniqueness, alias addressing, future readiness)
  and the migration stays resilient-by-design.

## Alternatives Considered

- **Bare `PRIMARY KEY (id)` UUIDs**: simpler port, but Citus distribution
  stays blocked exactly as today — rejected as misaligned with ADR-013.
- **Keep app-assigned TEXT**: zero churn, but permanently forfeits native
  type safety, index density, and sharding — rejected per owner direction.
