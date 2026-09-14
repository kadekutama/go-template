# SDD Gates (single source of truth for gates)

**Status values:** `pending` → `in_progress` → `completed` (+ `blocked` with reason).

These are capability-promotion gates, not permission to start every task in a
later-numbered epic. Scheduling follows the acyclic task dependency graph in
`tasks/SDD.md`; consumers may be developed against approved contracts/fakes but
the capability is not stable or externally exposed until its gate passes.

## G1 — Bootstrap + platform core compile
**Status:** completed
**Promotes capability for:** ledger/domain slice dependencies
**Checks:**
- [x] `make build-all` succeeds (5 stub binaries).
- [x] `make lint` clean.
- [x] CI lint+build green on a test PR.
- [x] `fx.ValidateApp` passes (no cycles) — `E01-T02`.
- [x] Bad config fails with all violations listed — `E01-T03`.
- [x] `python3 tasks/scripts/check-tasks.py --format --graph --sdd` passes.
- [x] SDD lifecycle fixture tests prove cycle, claim, progress, evidence, and
  handoff failures are rejected — `E00-T08`.

## G2 — Domain complete
**Status:** pending
**Promotes capability for:** application-layer consumers
**Checks:**
- [ ] `go test ./internal/domain/... -race -count=3` passes, coverage ≥90%.
- [ ] Every `money-flow.md §8` rule has an executable spec (`check-tasks.py --specs`).
- [ ] Property/model tests prove positive minor units, checked arithmetic,
  per-currency balance, normal-side display, reversal linkage, and hold state machines.
- [ ] Every webhook in `api-contracts.md §10` maps to a domain event (`check-tasks.py --events`).
- [ ] No external imports in `internal/domain/...`.
- [ ] `make generate-mocks` compiles against all ports.

## G3 — Application complete
**Status:** pending
**Promotes capability for:** adapter integration
**Checks:**
- [ ] `go test ./internal/application/... -race -count=3` passes, coverage ≥85%.
- [ ] Every `api-contracts.md §7` endpoint group has handlers (`check-tasks.py --handlers`).
- [ ] Saga crash-resume tests pass; every `money-flow.md §10` row has a saga test.

## G4 — Adapters implement ports, integration green
**Status:** pending
**Promotes capability for:** public API and worker integration
**Checks:**
- [ ] Migrations up/down clean; RLS adversarial test passes.
- [ ] Outbox at-least-once redelivery + durable inbox effectively-once effects proven; cache invalidation/cursor labeling proven.
- [ ] Direct SQL cannot insert unbalanced postings or mutate/delete posted facts;
  concurrent-spend model never overspends when overdraft is disabled.
- [ ] NATS groups drain; DLQ + delivery replay proven; event replay never re-runs ledger commands.
- [ ] Auth matrix (journeys §5) green; no secret in code/images.
- [ ] Provider fakes allow zero-vendor-credential test runs.

## G5 — API + worker contracts satisfied
**Status:** pending
**Promotes capability for:** end-to-end observability and resilience
**Checks:**
- [ ] All `api-contracts.md §7` endpoints live + in OpenAPI + covered by integration/contract tests.
- [ ] The E11-T14 restricted ledger pilot proves posting → strong balance →
  entries through real PostgreSQL before broader workflow endpoints are promoted.
- [ ] `buf breaking` clean; GraphQL introspection matches schema.
- [ ] All 8 cron jobs have durable run keys and idempotent effects under contention; all 4 consumer groups drain with DLQ coverage.
- [ ] Journeys §2.1/§2.2/§2.4–§2.6 executable end-to-end against the APIs.

## G6 — Observability + resilience end-to-end
**Status:** pending
**Promotes capability for:** cross-cutting verification
**Checks:**
- [ ] Request→Tempo trace demo recorded.
- [ ] Test alert reaches its channel; dashboards provisioned.
- [ ] Panic suite (6 boundaries) green; rate-limit suite green.
- [ ] Breaker audit clean (`check-tasks.py --breakers`).

## G7 — Verification + delivery green
**Status:** pending
**Promotes capability for:** delivery and release hardening
**Checks:**
- [ ] Contract suite green; baseline k6 passes on PR; chaos expectations tracked.
- [ ] Full CI matrix green; staging auto-deploys; compose variants validated; one-Postgres proven.
- [ ] Coverage bars enforced (domain ≥90%, app ≥85%, overall ≥80%).

## G8 — Docs, hardening, release sign-off
**Status:** pending
**Promotes capability for:** release
**Checks:**
- [ ] Zero pending ADRs; docs link-check clean; onboarding ≤10 min timed.
- [ ] Header/TLS scans clean; vuln scans zero HIGH/CRITICAL; SBOM + licenses attached.
- [ ] Perf baselines committed with regression gate; DR drill measured inside RPO/RTO; NFR table signed.
- [ ] Release checklist fully ticked.
