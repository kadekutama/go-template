# Epic E15: Observability + Resilience

**Status:** pending
**Story Points:** 18
**Phase:** 7
**Dependencies:** E11, E12, E13, E14 (live traffic to observe; basic middleware already in E11–E13)
**SDD Gate:** G6
**Design refs:** `SPEC.md §7.9`, `SPEC.md §9.4`, `SPEC.md §9.7–§9.8`,
`docs/money-flow.md §9`, `docs/domain-events.md §10`, `docs/data-flow.md §3`

> Scope note: E01 bootstrapped tracing/logging SDKs; E11–E13 installed the
> middleware chain. This epic completes the *pipeline* (collector → backends →
> dashboards → alerts) and wires resilience behaviors (breakers, limits, panic
> recovery verification, HTTP/3 enablement).

## Tasks

### E15-T01: OTel collector pipeline (traces → Tempo, metrics → Prometheus, logs → Loki)
**Status:** pending
**Background:** Single pipeline for all signals (`SPEC.md §7.9`, compose §12.4).
**Files:**
- Create: `deployments/observability/otel-collector.yaml`, `deployments/observability/tempo.yaml` (if missing)
**Steps:**
1. Collector: OTLP receivers, batch + tail-sampling (10%, always errors) processors,
   exporters to Tempo/Prometheus/Loki.
2. Resource attrs: service.name/version, deployment.environment, host.
3. Verify: one request produces spans in Tempo across HTTP→app→DB→NATS.
**Acceptance Criteria:**
- [ ] Trace for a posted transaction visible end-to-end in Tempo (manual + test hook).
- [ ] Sampling config change requires only config edit + rollout (no code).
**Story Points:** 4
**Depends On:** E11-T01
**Related Docs:** `SPEC.md §7.9`, `SPEC.md §12.4`, `SPEC.md §2` (OTel v1.46.0, Tempo 2.9.4, Loki 3.7.7, Prom 3.14.0)
**SDD Gate:** G6

---

### E15-T02: RED/USE/business metrics, alerts, dashboards
**Status:** pending
**Background:** Money-flow §9 metric catalog + features §11.1 dashboards/alerting.
**Files:**
- Create: `deployments/observability/{prometheus-rules.yaml,alertmanager.yaml,dashboards/ledger-*.json}`
**Steps:**
1. Record every money-flow §9 metric (transactions posted, balances, transfer latency,
   reconciliation breaks, FX age, idempotency conflicts, payout delay) + RED/USE per service.
2. Alert rules with thresholds + runbook links; Alertmanager routes (critical→PagerDuty, warn→Slack).
3. Grafana dashboards: ledger-ops (business), service-health (RED), infra (USE); auto-provisioned.
**Acceptance Criteria:**
- [ ] Firing a test alert reaches the routed channel (integration test with webhook receiver).
- [ ] Dashboard JSON lint-validated in CI.
**Story Points:** 5
**Depends On:** E15-T01
**Related Docs:** `docs/money-flow.md §9`, `docs/fintech-ledger-features.md §11.1`, `SPEC.md §12.4`
**SDD Gate:** G6

---

### E15-T03: Panic-recovery verification across all boundaries
**Status:** pending
**Background:** SPEC §9.7 behaviors were installed per-boundary (E11–E14); this
task proves them uniformly + sets the panic budget.
**Files:**
- Create: `test/integration/resilience/panic_test.go`
**Steps:**
1. Trigger panics in: Echo handler, gRPC method, GraphQL resolver, NATS consumer,
   cron job, background goroutine — assert each boundary's specified behavior
   (log+stack+trace, metric, alert, correct client response / Nak+DLQ / FAILED+unlock).
2. Document the panic budget (any prod panic pages; postmortem required).
**Acceptance Criteria:**
- [ ] All six boundaries verified in one suite, green with `-race`.
**Story Points:** 3
**Depends On:** E11-T01, E12-T02, E13-T04, E14-T01, E14-T03
**Related Docs:** `SPEC.md §9.7`
**SDD Gate:** G6

---

### E15-T04: Rate-limit enforcement verification + tuning
**Status:** pending
**Background:** Token-bucket behavior (SPEC §9.4) with E08 Lua + E11 headers.
**Files:**
- Create: `test/integration/resilience/ratelimit_test.go`
**Steps:**
1. Burst tests per dimension (ip/user/tenant/key/endpoint) → 429 with headers + `Retry-After`.
2. Tune defaults from k6 baseline (E16) and record in config comments.
**Acceptance Criteria:**
- [ ] Sustained over-limit traffic never exceeds budget by >5% (test).
- [ ] Per-tenant overrides honored (test).
**Story Points:** 2
**Depends On:** E11-T01, E08-T01, E08-T07, E11-T15
**Related Docs:** `SPEC.md §9.4`, `docs/api-contracts.md §2, §12`
**SDD Gate:** G6

---

### E15-T05: HTTP/3 enablement (behind flag)
**Status:** pending
**Background:** SPEC §9.8 — off by default; enable path must be tested, not just documented.
**Files:**
- Modify: `pkg/httpserver/http3.go`; add focused HTTP/3 transport tests. E17 owns
  the gateway/compose manifests and consumes this transport contract.
**Steps:**
1. UDP 443 exposure, TLS cert wiring, Alt-Svc advertisement, fallback verification.
2. k6 `--http3` smoke suite (nightly, not PR-gate).
**Acceptance Criteria:**
- [ ] QUIC handshake succeeds with flag on; HTTP/2 fallback works with flag off (tests).
**Story Points:** 2
**Depends On:** E11-T01, E11-T15
**Related Docs:** `SPEC.md §9.8`
**SDD Gate:** G6

---

### E15-T06: Resilience wiring audit (breakers on every external call)
**Status:** pending
**Background:** E01-T08 defines the breaker/retry policy; E10 providers and
other external adapters must all use it. This audit closes the loop.
**Files:**
- Modify: provider call sites as needed; Create: `docs/architecture/ADR-breaker-policy.md`
**Steps:**
1. Enumerate every external call (payment processor, FX, SMTP, OAuth, Unleash, object storage) → assert breaker+timeout+retry present.
2. Record policy (thresholds per dependency) in an ADR.
**Acceptance Criteria:**
- [ ] `check-tasks.py --breakers` (new check: provider files reference breaker) passes.
- [ ] Breaker state metrics visible in Grafana (screenshot/link in ADR).
**Story Points:** 2
**Depends On:** E01-T08, E10-T01, E10-T02, E10-T03, E09-T07
**Related Docs:** `SPEC.md §7.8`, `docs/fintech-ledger-features.md §11.2`
**SDD Gate:** G6

## Acceptance Criteria

- [ ] E15-T01 … E15-T06 all `completed` (count 18 SP in `tasks/tracking/PROGRESS.md`)
- [ ] Request→Tempo trace demo recorded; alerts fire to channels
- [ ] Panic/rate-limit suites green; breaker audit clean
- [ ] SDD gate G6 checks pass — `tasks/tracking/GATES.md#G6`
