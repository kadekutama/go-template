# Epics Overview

**Source docs:** `tasks/SDD.md`, `tasks/SDD-INTEROP.md`, `tasks/DELIVERY-SLICES.md`,
`docs/ledger-core.md`, `docs/development/go-conventions.md`,
`SPEC.md`, `docs/fintech-ledger-features.md`, `docs/api-contracts.md`,
`docs/money-flow.md`, `docs/data-flow.md`, `docs/user-journeys.md`, `docs/domain-events.md`
**Total:** 21 epics, 451 story points, 8 promotion gates.

## One-liners

| Epic | One-liner | SP | Phase | Depends On | Gate |
|------|-----------|----|-------|------------|------|
| E00 | Git baseline, repo bootstrap, lint/CI/scripts, harness-neutral SDD control plane | 14 | 0 | — | G1 |
| E01 | Platform core: fx DI, validated config, logging, tracing, shared kernel, resilience policy | 17 | 1 | E00 | G1 |
| E02 | Ledger domain core: VOs, Ledger/Account/Posting/Entry/Hold/Period, domain specs, events, ports | 30 | 2 | E01 | G2 |
| E03 | Money-movement domain: transfers (incl. scheduled/recurring/bulk/templates), refunds, payouts, fees, interest, FX, payment methods/settlement/returns, payout policy/recovery | 37 | 3 | E02 | G2 |
| E04 | Compliance domain: reconciliation rules, breaks, period-close rules, GDPR erasure, AML hooks, regulatory reports, SOX approvals | 16 | 3 | E02 | G2 |
| E05 | Tenancy domain: tenant aggregate, hierarchies, RLS policies, onboarding, residency, white-label | 11 | 3 | E02 | G2 |
| E06 | Application layer: core/extended ports, ledger pilot use cases, workflows, reports | 40 | 4 | task-level domain dependencies | G3 |
| E07 | Persistence adapters: SQL posting path, migrations, RLS, outbox, shared test harness, replicas | 34 | 5 | E06 | G4 |
| E07.1 | Distributed persistence: Citus multi-tenant sharding, Patroni HA, CloudNativePG, etcd coordination | 19 | 5.1 | E07 | G4 |
| E08 | Cache + messaging adapters: Otter L1, hybrid cache, Redpanda + NATS Core, webhook dispatcher, rate limiter | 24 | 5.2 | E06, E07.1 | G4 |
| E09 | Identity + security adapters: JWT, OAuth2, Casbin, API keys, OpenBao secrets & transit, audit log, PII | 22 | 5.2 | E06, E07.1 | G4 |
| E10 | Platform integrations: Unleash, FX provider, payment-processor sandbox, statement parsers, SMTP | 17 | 5.2 | E06, E07.1 | G4 |
| E11 | REST API: core server, ledger pilot, public middleware, §7 groups, OpenAPI | 43 | 6 | task-level E06–E10, E07.1 dependencies | G5 |
| E12 | gRPC API: proto, server, interceptors, gateway, parity with REST | 16 | 6 | E07.1, E08–E10 | G5 |
| E13 | GraphQL API: schema, resolvers, DataLoader, subscriptions, aggregations, parity | 18 | 6 | E07.1, E08–E10 | G5 |
| E14 | Workers: cron binary + 8 jobs, consumer binary + 4 groups + DLQ | 16 | 6 | E07.1, E08–E10 | G5 |
| E15 | Observability + resilience: OTel pipeline, metrics/alerts/dashboards, Loki, panic recovery, rate limits, HTTP/3 | 18 | 7 | E11–E14 | G6 |
| E16 | Verification: cross-protocol contracts, k6 suites, litmus, coverage gates | 14 | 8 | E11–E15 | G7 |
| E17 | Delivery: full CI/CD, multi-arch images, compose variants, K8s/Kustomize/ArgoCD | 16 | 9 | E16 | G7 |
| E18 | Docs + DX: ADRs (incl. proposed/pending), layer docs, API docs, runbooks, SDKs, sandbox | 16 | 10 | E01–E17, E07.1 | G8 |
| E19 | Hardening + release: headers/TLS/mTLS, vuln-zero, SBOM/licenses, PGO/bench, backup-DR drills, NFR sign-off | 13 | 11 | E16–E18 | G8 |

## Dependency DAG (reporting view)

This is the capability-promotion order, not the task scheduler. Agents select
work from the acyclic task-level `Depends On` graph. Epics group ownership and
reporting, so a dependency-ready vertical-slice task may start before every task
in its epic's nominal phase is complete. A gate must still pass before its
capability is treated as stable or exposed to the next release boundary.

```
Phase 0:   E00
Phase 1:   E01 → (needs E00)
Phase 2:   E02 → (needs E01)
Phase 3:   E03, E04, E05 → (each needs E02)
Phase 4:   E06 → (needs E02–E05)
Phase 5:   E07 (completed)
Phase 5.1: E07.1 → (needs E07)
Phase 5.2: E08, E09, E10 → (each needs E06, E07.1)
Phase 6:   E11, E12, E13, E14 → (each needs E07.1, E08–E10)
Phase 7:   E15 → (needs E11–E14)
Phase 8:   E16 → (needs E11–E15)
Phase 9:   E17 → (needs E16)
Phase 10:  E18 → (needs E01–E17, E07.1)
Phase 11:  E19 → (needs E16–E18)
```

## Key ordering decisions (why not Nemotron's order)

- **CI skeleton in E00, not E17:** every later PR is gated from day one (lint+build). E17 completes the pipeline; it does not start it.
- **Adapters (E07–E10) after application ports (E06):** adapters implement ports; ports must exist first. Nemotron had infra depending only on E01 while implementing E02/E03's ports — impossible order.
- **Tests live inside each epic:** no big-bang test phase. E07-T09 provides the
  shared Testcontainers harness before adapter suites; E16 owns only cross-cutting
  contract/performance/chaos verification.
- **Middleware chain in E11, pipeline in E15:** APIs need a working chain to be testable; E15 hardens it into the full LGTM pipeline.
- **ADRs concurrent (E18 tracks, but each epic writes its own):** every epic file has an explicit "record ADR" step; E18 collects and indexes.

## SDD gates → epics

| Gate | Meaning | Epics |
|------|---------|-------|
| G1 | Bootstrap + platform core compile, fx wires acyclically | E00, E01 |
| G2 | Domain complete: specs executable, unit tests ≥90% | E02–E05 |
| G3 | Application complete: handlers + sagas tested ≥85% | E06 |
| G4 | Adapters implement ports, integration tests green | E07–E10 |
| G5 | API + worker contracts satisfied | E11–E14 |
| G6 | Observability + resilience end-to-end | E15 |
| G7 | Verification + delivery green | E16, E17 |
| G8 | Docs, hardening, release sign-off | E18, E19 |

Details + check commands: `tasks/tracking/GATES.md`.
Progress dashboard: `tasks/tracking/PROGRESS.md`.
Vertical scheduling: `tasks/DELIVERY-SLICES.md`.
