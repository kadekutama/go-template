# Progress Dashboard (single source of truth for progress)

**Last updated:** 2026-09-13
**How to update:** flip task `**Status:**` in the epic file, then tick the box here.
Run `python3 tasks/scripts/check-tasks.py` to verify consistency.

## Epics

| Epic | Title | SP | Done | Status | Gate |
|------|-------|----|------|--------|------|
| E00 | Foundation & repo bootstrap | 14 | 6/14 | pending | G1 |
| E01 | Platform core (fx, config, logging, tracing, kernel, resilience) | 17 | 0/17 | pending | G1 |
| E02 | Ledger domain core | 30 | 0/30 | pending | G2 |
| E03 | Money-movement domain | 37 | 0/37 | pending | G2 |
| E04 | Compliance domain | 16 | 0/16 | pending | G2 |
| E05 | Tenancy domain | 11 | 0/11 | pending | G2 |
| E06 | Application layer | 40 | 0/40 | pending | G3 |
| E07 | Persistence adapters | 34 | 0/34 | pending | G4 |
| E08 | Cache + messaging adapters | 24 | 0/24 | pending | G4 |
| E09 | Identity + security adapters | 22 | 0/22 | pending | G4 |
| E10 | Platform integrations | 17 | 0/17 | pending | G4 |
| E11 | REST API | 43 | 0/43 | pending | G5 |
| E12 | gRPC API | 16 | 0/16 | pending | G5 |
| E13 | GraphQL API | 18 | 0/18 | pending | G5 |
| E14 | Workers (cron + consumer) | 16 | 0/16 | pending | G5 |
| E15 | Observability + resilience | 18 | 0/18 | pending | G6 |
| E16 | Verification (contract, perf, chaos) | 14 | 0/14 | pending | G7 |
| E17 | Delivery (CI/CD, images, K8s) | 16 | 0/16 | pending | G7 |
| E18 | Docs + DX | 16 | 0/16 | pending | G8 |
| E19 | Hardening + release | 13 | 0/13 | pending | G8 |

**Total:** 6/432 SP completed.

## Task checklists (tick as epic files flip to completed)

### E00 — Foundation
- [x] E00-T00 repository boundary + Git baseline
- [x] E00-T01 Go module + folders
- [x] E00-T02 Makefile
- [x] E00-T03 golangci-lint
- [x] E00-T04 hot-reload + dev scripts
- [ ] E00-T05 minimal CI + dependency automation
- [ ] E00-T06 docs skeleton
- [ ] E00-T07 scripts package
- [ ] E00-T08 harness-neutral SDD control plane

### E01 — Platform core
- [ ] E01-T01 go.mod versions
- [ ] E01-T02 fx registry
- [ ] E01-T03 koanf config
- [ ] E01-T04 logging port + slog
- [ ] E01-T05 tracing port + OTel
- [ ] E01-T06 kernel (errors/i18n/pagination/shutdown/safe/clock/ID)
- [ ] E01-T07 JSON codec wrapper
- [ ] E01-T08 resilience primitives (breaker/timeout/retry)

### E02 — Ledger domain
- [ ] E02-T01 domain event + specification primitives
- [ ] E02-T02 Money/Currency/IDs
- [ ] E02-T03 Account aggregate
- [ ] E02-T04 Posting + Entry + Hold
- [ ] E02-T05 Journal/Period/sub-ledgers
- [ ] E02-T06 domain events catalog
- [ ] E02-T07 specifications catalog
- [ ] E02-T08 repository ports
- [ ] E02-T09 domain test suite (G2)

### E03 — Money movement
- [ ] E03-T01 transfers (immediate/scheduled/recurring/bulk/templates)
- [ ] E03-T02 refunds
- [ ] E03-T03 payouts + settlement
- [ ] E03-T04 fees + interest
- [ ] E03-T05 FX + gain/loss
- [ ] E03-T06 methods/settlement/returns/linking
- [ ] E03-T07 disputes (evidence/representment/fees)
- [ ] E03-T08 auth-capture/descriptors/SCA
- [ ] E03-T09 platform split (destination charges)
- [ ] E03-T10 top-ups
- [ ] E03-T11 payout eligibility + negative-balance recovery

### E04 — Compliance
- [ ] E04-T01 matching + break taxonomy
- [ ] E04-T02 resolution + SoD approvals
- [ ] E04-T03 period-close rules
- [ ] E04-T04 GDPR erasure + portability
- [ ] E04-T05 AML hooks + regulatory reports

### E05 — Tenancy
- [ ] E05-T01 tenant + onboarding
- [ ] E05-T02 hierarchies + consolidation
- [ ] E05-T03 isolation contract + ADR-004
- [ ] E05-T04 tenancy unit tests

### E06 — Application
- [ ] E06-T01 handler plumbing
- [ ] E06-T02 account + tenant handlers
- [ ] E06-T03 transaction + transfer handlers
- [ ] E06-T04 payment + refund + payout handlers
- [ ] E06-T05 recon + period + report + compliance handlers
- [ ] E06-T06 core ledger integrity ports
- [ ] E06-T07 sagas
- [ ] E06-T08 app test suite (G3)
- [ ] E06-T09 extended reporting
- [ ] E06-T10 metering + billing export
- [ ] E06-T11 dispute handlers
- [ ] E06-T12 extended workflow + adapter ports
- [ ] E06-T13 core posting + strong-balance use cases

### E07 — Persistence
- [ ] E07-T01 ledger-core schema + posting transaction
- [ ] E07-T02 RLS
- [ ] E07-T03 outbox
- [ ] E07-T04 seed data
- [ ] E07-T05 backup/restore scripts
- [ ] E07-T06 integration tests (G4 slice)
- [ ] E07-T07 read replicas
- [ ] E07-T08 residency routing
- [ ] E07-T09 shared Testcontainers harness + fixtures
- [ ] E07-T10 workflow/tenancy/reconciliation schema

### E08 — Cache + messaging
- [ ] E08-T01 hybrid cache
- [ ] E08-T02 Redlock
- [ ] E08-T03 NATS topology
- [ ] E08-T04 publisher + idempotent consumer
- [ ] E08-T05 webhook dispatcher
- [ ] E08-T06 integration tests (G4 slice)
- [ ] E08-T07 rate limiter impl

### E09 — Identity + security
- [ ] E09-T01 JWT
- [ ] E09-T02 OAuth2/OIDC
- [ ] E09-T03 Casbin
- [ ] E09-T04 API keys
- [ ] E09-T05 Bitwarden
- [ ] E09-T06 envelope crypto + PII
- [ ] E09-T07 audit logger

### E10 — Integrations
- [ ] E10-T01 feature flags
- [ ] E10-T02 FX provider
- [ ] E10-T03 payment processor + sandbox
- [ ] E10-T04 statement parsers
- [ ] E10-T05 SMTP/maildev
- [ ] E10-T06 provider tests (G4 slice)

### E11 — REST
- [ ] E11-T01 server core + lifecycle
- [ ] E11-T02 envelope/errors/pagination/filter
- [ ] E11-T03 accounts + tenants
- [ ] E11-T04 transactions + transfers + bulk
- [ ] E11-T05 payments + refunds + payouts
- [ ] E11-T06 recon + periods + reports
- [ ] E11-T07 webhooks mgmt
- [ ] E11-T08 OpenAPI generation
- [ ] E11-T09 integration + contract tests (G5 slice)
- [ ] E11-T10 ops endpoints (health/versioning)
- [ ] E11-T11 white-label (P2)
- [ ] E11-T12 dispute endpoints
- [ ] E11-T13 search endpoints
- [ ] E11-T14 restricted ledger pilot endpoints
- [ ] E11-T15 public middleware + transport hardening

### E12 — gRPC
- [ ] E12-T01 proto + buf
- [ ] E12-T02 server + interceptors + gateway
- [ ] E12-T03 service implementations
- [ ] E12-T04 integration tests (G5 slice)

### E13 — GraphQL
- [ ] E13-T01 schema + codegen
- [ ] E13-T02 resolvers + DataLoader
- [ ] E13-T03 subscriptions
- [ ] E13-T04 server + tests (G5 slice)
- [ ] E13-T05 aggregations

### E14 — Workers
- [ ] E14-T01 cron binary + leadership
- [ ] E14-T02 all 8 jobs
- [ ] E14-T03 consumer binary + DLQ ops
- [ ] E14-T04 worker tests (G5 slice)

### E15 — Observability
- [ ] E15-T01 OTel pipeline
- [ ] E15-T02 metrics/alerts/dashboards
- [ ] E15-T03 panic verification
- [ ] E15-T04 rate-limit verification
- [ ] E15-T05 HTTP/3 enablement
- [ ] E15-T06 breaker audit

### E16 — Verification
- [ ] E16-T02 contract suite
- [ ] E16-T03 k6 suites
- [ ] E16-T04 litmus scenarios
- [ ] E16-T05 coverage bars

### E17 — Delivery
- [ ] E17-T01 full CI
- [ ] E17-T02 Dockerfiles
- [ ] E17-T03 compose variants
- [ ] E17-T04 K8s + ArgoCD

### E18 — Docs + DX
- [ ] E18-T01 pending ADRs
- [ ] E18-T02 layer docs
- [ ] E18-T03 API docs publishing
- [ ] E18-T04 runbooks
- [ ] E18-T05 SDKs + sandbox + onboarding

### E19 — Hardening + release
- [ ] E19-T01 security sign-off
- [ ] E19-T02 SBOM + licenses
- [ ] E19-T03 perf tuning + baselines
- [ ] E19-T04 DR drills + NFR sign-off + checklist

## Gates

| Gate | Status | Promotes capability for |
|------|--------|-------------------------|
| G1 | pending | E02 |
| G2 | pending | E06 |
| G3 | pending | E07–E10 |
| G4 | pending | E11–E14 |
| G5 | pending | E15 |
| G6 | pending | E16 |
| G7 | pending | E17–E19 |
| G8 | pending | release |

Details: `tasks/tracking/GATES.md`.
