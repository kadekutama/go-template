# Epic E05: Tenancy Domain

**Status:** pending
**Story Points:** 11
**Phase:** 3 (parallel with E03, E04)
**Dependencies:** E02
**SDD Gate:** G2
**Design refs:** `docs/fintech-ledger-features.md §5`, `docs/data-flow.md §3`
(tenant isolation), `docs/user-journeys.md §2.1` (onboarding), `SPEC.md §14` (residency/PII)

> Why a separate epic: tenancy cuts across every query, key, subject, and RLS
> policy. Nemotron left it as one bullet ("Tenant Isolation") with no tasks for
> hierarchies, onboarding, residency, or white-labeling.

## Tasks

### E05-T01: Tenant aggregate + onboarding rules
**Status:** pending
**Background:** Self-serve provisioning (features §5) from journeys §2.1:
tenant + default chart of accounts + API keys, atomically.
**Files:**
- Create: `internal/domain/entity/tenant.go`, `internal/domain/aggregate/tenant.go`,
  `internal/domain/service/tenant_onboarding.go`
**Steps:**
1. `Tenant`: ID, Name (unique), Region, Status, Settings (default currency,
   timezone, enabled features/payment methods), timestamps.
2. Onboarding checklist as domain rule: tenant valid → default accounts created
   (operating + fee + suspense per currency) → API key pair issued → `TenantCreated`.
3. Name-uniqueness and region-allowlist specs.
**Acceptance Criteria:**
- [ ] Onboarding is all-or-nothing at the domain level (checklist test).
- [ ] Duplicate tenant name rejected (test).
- [ ] Disallowed region rejected (test).
**Story Points:** 3
**Depends On:** E02-T03, E02-T06
**Related Docs:** `docs/fintech-ledger-features.md §5`, `docs/user-journeys.md §2.1`, `docs/domain-events.md §3` (TenantCreated)
**SDD Gate:** G2

---

### E05-T02: Tenant hierarchies + consolidated views
**Status:** pending
**Background:** Features §5 parent/child tenants with roll-up reporting.
**Files:**
- Create: `internal/domain/entity/tenant_hierarchy.go` (or extend tenant.go),
  `internal/domain/service/consolidation.go`
**Steps:**
1. Parent links (acyclic — cycle rejected), depth limit.
2. Consolidation helper: roll up child balances with FX conversion at a given rate date.
3. Permission rule: parent read access requires explicit grant per child (no implicit).
**Acceptance Criteria:**
- [ ] Cyclic parenting rejected (test).
- [ ] Consolidated balance equals sum of converted children (property test).
- [ ] Cross-child transfers without grant rejected (test).
**Story Points:** 3
**Depends On:** E05-T01, E03-T05
**Related Docs:** `docs/fintech-ledger-features.md §5`
**SDD Gate:** G2

---

### E05-T03: Isolation contract (RLS policies, key/subject conventions)
**Status:** pending
**Background:** Makes data-flow §3 tenant isolation enforceable: every query,
cache key, and subject carries tenant. This task writes the contract tests
against; E07/E08 implement it.
**Files:**
- Create: `docs/architecture/ADR-004-tenant-isolation.md` (shared DB + RLS —
  features §15 ADR-004),
  `internal/domain/service/isolation.go` (pure helpers: key builders, subject builders)
**Steps:**
1. Key convention: `balance:{tenant}:{account}:{asset}`,
   `idempotency:{tenant}:{operation}:{key}`, `ratelimit:{tenant}:{user}` —
   builder functions with tests. The durable database key is scoped by the same
   tenant/operation/key tuple; cache keys are only hints.
2. Subject convention: `ledger.{tenant}.{event_type}` (the versioned event type
   already contains the domain/action segments) — builder + parser.
3. RLS policy definitions (SQL in E07; here: policy matrix tenant×table×operation).
4. Data-residency settings shape: per-tenant region/db pointer (enforced in E07).
5. White-label settings shape: branding, domains (validated format, no behavior).
**Acceptance Criteria:**
- [ ] Key/subject builders round-trip (parse(build(x)) == x) (test).
- [ ] Policy matrix covers every table E07 will create (review vs E07-T01 file list).
- [ ] ADR-004 written and linked.
**Story Points:** 3
**Depends On:** E05-T01
**Related Docs:** `docs/data-flow.md §3`, `docs/fintech-ledger-features.md §5`, `SPEC.md §15` (ADR-004), `docs/domain-events.md §4.4`
**SDD Gate:** G2

---

### E05-T04: Tenancy unit tests (G2 slice)
**Status:** pending
**Background:** G2 coverage for this epic.
**Files:**
- Create: `test/unit/domain/tenant/...`
**Steps:**
1. Onboarding, hierarchy, consolidation, isolation builders, residency/white-label validation.
2. Run with `-race -count=3`; contributes to the ≥90% domain coverage bar.
**Acceptance Criteria:**
- [ ] `go test ./internal/domain/... -race -count=3` still passes with new packages.
- [ ] No external imports in new code (guard from E02-T09).
**Story Points:** 2
**Depends On:** E05-T01, E05-T02, E05-T03
**Related Docs:** `SPEC.md §10.2`
**SDD Gate:** G2

## Acceptance Criteria

- [ ] E05-T01 … E05-T04 all `completed` (count 11 SP in `tasks/tracking/PROGRESS.md`)
- [ ] Every features §5 row has rules + tests
- [ ] Isolation contract consumable by E07/E08/E11 without follow-up questions
- [ ] SDD gate G2 checks pass — `tasks/tracking/GATES.md#G2`
