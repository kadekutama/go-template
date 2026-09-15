# ADR-004: Tenant isolation (shared DB + RLS)

**Status:** Accepted
**Date:** 2026-09-14
**Approved by:** Repository owner

## Context

Every query, cache key, subject, and idempotency record cuts across tenants.
The platform must isolate tenants by default, fail closed on missing scope,
and keep operations (migrations, seeds, analytics) workable without per-tenant
databases. The owning task is `tasks/epics/E05-tenancy.md#E05-T03`; the
enforceable contract lives in `internal/domain/service/isolation.go`.

## Decision

- **Shared PostgreSQL database with Row-Level Security.** All ledger tables
  carry `tenant_id` (composite scope with `ledger_id` per `docs/data-flow.md
  §5`); every repository sets `app.current_tenant` and RLS filters by it.
  Missing tenant fails closed. Service roles bypass RLS only for migrations.
- **Tenant-scoped keys and subjects are deterministic builders.** Cache keys
  (`balance:{tenant}:{account}:{asset}[:{cursor}]`,
  `idempotency:{tenant}:{operation}:{key}`, `ratelimit:{tenant}:{user}`) are
  hints only; the durable idempotency key is `(tenant_id, operation,
  idempotency_key)` (ledger-core §8). NATS subjects are
  `ledger.{tenant}.{event_type}` (data-flow §3).
- **Policy matrix is the contract.** `RLSPolicyMatrix()` covers every §5 core
  table (tenants self-scoped, asset_registry shared-read, inbox service-only,
  rest tenant-scoped) for E07-T02 to enforce in SQL.
- **Residency and white-label are shapes, not behavior here.** Per-tenant
  region/DB pointers route in E07; branding/domains validate format only.

## Consequences

- One database to operate, backup, and migrate; tenant bugs become RLS-policy
  bugs, proven by adversarial cross-tenant integration tests in E07-T02.
- Cache/inbox loss never authorizes spending or leaks rows; authority stays in
  PostgreSQL uniqueness, locks, and RLS.
- Key/subject formats are frozen; renames require a versioned migration and ADR.

## Alternatives considered

- **Separate database per tenant:** rejected for the first release; it
  multiplies migration/backup cost and still needs the same key/subject
  conventions. Revisitable with residency evidence.
- **Separate schema per tenant:** rejected; it complicates migrations and RLS
  auditing without removing the need for tenant-scoped keys/subjects.
- **Application-only filtering (no RLS):** rejected; a single missing WHERE
  clause leaks rows. RLS is the last line of defense per ledger-core §3.

## Traceability

- Normative contract: `docs/ledger-core.md §2–§3, §8`, `docs/data-flow.md §3, §5`
- Domain implementation: `tasks/epics/E05-tenancy.md` (E05-T03), `internal/domain/service/isolation.go`
- Enforcement: `tasks/epics/E07-persistence.md` (E07-T02), `tasks/epics/E08-cache-messaging.md`
