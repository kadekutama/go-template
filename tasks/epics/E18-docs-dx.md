# Epic E18: Docs + Developer Experience

**Status:** pending
**Story Points:** 16
**Phase:** 10
**Dependencies:** E01–E17 (documents what exists; each epic already wrote its ADRs inline)
**SDD Gate:** G8
**Design refs:** `SPEC.md §4` (docs skeleton from E00-T06), `SPEC.md §11` (SDK gen),
`SPEC.md §15` (ADR backlog incl. 4 pending), `docs/fintech-ledger-features.md §12`

> Note: content accrues during each epic (every epic file has an ADR step).
> This epic completes, indexes, and verifies — it does not start documentation.

## Tasks

### E18-T01: Record audit decisions, resolve the pending ADRs, and build index
**Status:** pending
**Background:** The ledger audit decisions for ADR-002/003/009 were approved by
the repository owner on 2026-09-13, and ADR-006
(reconciliation ingestion), ADR-007 (FX source/freshness), ADR-008 (archival),
and ADR-011 (balance materialization/authority) remain pending. ADR-002/003/009
already have permanent accepted records; this task must record the four pending
decisions and finish the ADR index, with owner approval where noted.
**Files:**
- Create: `docs/architecture/ADR-00{2,3,6,7,8,9,11}-*.md` (and index the accepted
  ADR-010 logging decision); Modify: `docs/architecture/README.md`
**Steps:**
1. Each ADR: Context → Decision → Consequences → Alternatives, linked to the
   implementing tasks (e.g., ADR-002 → E02-T02).
2. Index table updated; `check-tasks.py --adrs` verifies every features §15 row resolved.
**Acceptance Criteria:**
- [ ] Zero "Pending" rows in the features §15 table (update it as part of this task).
- [ ] Each decision traceable to code (file links in ADR).
**Story Points:** 3
**Depends On:** E02-T02, E04-T01, E10-T02, E07-T05
**Related Docs:** `SPEC.md §15`, `tasks/epics/E02-ledger-domain.md`, `tasks/epics/E04-compliance.md`
**SDD Gate:** G8

---

### E18-T02: Layer documentation completion
**Status:** pending
**Background:** Fill the E00-T06 skeleton: architecture, domain, application,
infrastructure, api, development READMEs + per-topic pages.
**Files:**
- Modify: `docs/{architecture,domain,application,infrastructure,api,development}/**`
**Steps:**
1. Each layer doc: scope, diagrams (reuse design mermaid), key files table, cross-links.
2. Document the 11 detailed verification fixes (subjects, spellings, error codes) as decided outcomes.
3. Sync `AGENTS.md` + `.opencode/agents|skills|permissions` with the final epic/task structure
   (roles reference real epic IDs; skills reference real paths/versions).
4. Link-check pass over all docs.
**Acceptance Criteria:**
- [ ] `check-tasks.py --docs` (link check) passes with zero broken section links.
- [ ] Every epic's "Related Docs" links resolve (validates this whole tasks/ tree).
- [ ] A fresh agent following only `AGENTS.md` + `tasks/README.md` reaches the right epic file (review walkthrough).
**Story Points:** 4
**Depends On:** E02-T09, E06-T08, E07-T06, E11-T09
**Related Docs:** `SPEC.md §4`, `tasks/README.md`
**SDD Gate:** G8

---

### E18-T03: API documentation publishing
**Status:** pending
**Background:** OpenAPI/Swagger/ReDoc, GraphQL Voyager/Playground, buf docs, Postman.
**Files:**
- Modify: `api/openapi/openapi.yaml` (final regen), `docs/api/*`
**Steps:**
1. Regen + publish: Swagger UI `/docs/swagger`, ReDoc, Voyager/Playground (dev-only),
   `buf doc`, Postman collection artifact.
2. Every example request/response validated against live handlers (contract suite E16-T02).
**Acceptance Criteria:**
- [ ] Docs build has zero warnings; examples execute green.
**Story Points:** 3
**Depends On:** E11-T08, E12-T01, E13-T01, E16-T02
**Related Docs:** `SPEC.md §11`, `docs/api-contracts.md §1`
**SDD Gate:** G8

---

### E18-T04: Runbooks (deployment, incident, migration, secrets, DR, capacity)
**Status:** pending
**Background:** Production operations per SPEC §15 phase 8.
**Files:**
- Create: `docs/development/runbooks/{deployment,incident-response,database-migration,secrets-rotation,disaster-recovery,feature-flag-rollout,capacity-planning}.md`
**Steps:**
1. Each runbook: prerequisites, steps, verification, rollback, contacts — referencing
   E07-T05 (backup scripts), E17 (pipelines), E15 (alerts) concretely.
2. DR runbook encodes RPO<5min/RTO<30min with measured timings from E19 drill.
**Acceptance Criteria:**
- [ ] Each runbook reviewed by walking it against staging (sign-off line per file).
**Story Points:** 3
**Depends On:** E07-T05, E15-T02, E17-T01
**Related Docs:** `SPEC.md §15`, `docs/fintech-ledger-features.md §11.3`
**SDD Gate:** G8

---

### E18-T05: SDKs, sandbox, onboarding, webhook testing
**Status:** pending
**Background:** Features §12 developer experience.
**Files:**
- Create: `pkg/sdk/` (Go client), generator configs for TS/Python, `docs/development/onboarding.md`,
  `docs/development/webhook-testing.md`, sandbox seed profile
**Steps:**
1. Generate TS/Python/Go SDKs from OpenAPI; smoke-test each against sandbox.
2. Onboarding: 10-minute quickstart (compose → tenant → first payment → webhook receipt).
3. Webhook testing guide: local tunnel setup + signature verification walkthrough.
4. Sandbox: pre-seeded tenant + test cards + E07-T04 seed profile.
5. Test clocks: sandbox-only virtual time per tenant (backed by the `Clock` port override)
   to time-travel trials, interest accrual, renewals, and evidence deadlines in tests/demos.
**Acceptance Criteria:**
- [ ] Fresh engineer completes onboarding in ≤10 min (timed walkthrough recorded as checklist result).
- [ ] All three SDKs perform create→confirm→refund against sandbox (tests).
- [ ] Virtual clock advance of 30 days accrues interest and ages deadlines deterministically (test).
**Story Points:** 3
**Depends On:** E11-T08, E07-T04, E10-T03
**Related Docs:** `docs/fintech-ledger-features.md §12`, `docs/api-contracts.md §11`
**SDD Gate:** G8

## Acceptance Criteria

- [ ] E18-T01 … E18-T05 all `completed` (count 16 SP in `tasks/tracking/PROGRESS.md`)
- [ ] Zero pending ADRs; docs link-check clean; onboarding timed ≤10 min
- [ ] SDD gate G8 checks pass — `tasks/tracking/GATES.md#G8`
