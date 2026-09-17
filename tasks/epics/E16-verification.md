# Epic E16: Verification (Contract, Performance, Chaos, Coverage)

**Status:** pending
**Story Points:** 14
**Phase:** 8
**Dependencies:** E11, E12, E13, E14, E15 (things to verify)
**SDD Gate:** G7
**Design refs:** `SPEC.md §10.1–§10.6`, `SPEC.md §14` (NFRs: 10K TPS, p99<50ms),
`docs/fintech-ledger-features.md §11, §14`

> Scope note: per-epic tests and the shared E07-T09 harness are built before this
> phase. This epic owns suites that span epics (contract, performance, chaos) and
> the coverage bars that gate releases.

## Tasks

### E16-T02: Contract test suite (Pact + OpenAPI + buf + schema)
**Status:** pending
**Background:** Consumer-driven contracts (`SPEC.md §10.6`) across all three protocols.
**Files:**
- Create: `test/contract/{pact/,openapi/,buf/,graphql/}`
**Steps:**
1. Pact: REST consumer interactions per endpoint group; provider verification in CI with broker.
2. OpenAPI: live-handlers vs `api/openapi/openapi.yaml` conformance.
3. `buf breaking` against main; GraphQL introspection vs committed schema.
**Acceptance Criteria:**
- [ ] Contract suite green in CI; breaking proto/schema change fails the gate.
**Story Points:** 4
**Depends On:** E11-T08, E12-T01, E13-T01
**Related Docs:** `SPEC.md §10.6`, `SPEC.md §11`
**SDD Gate:** G7

---

### E16-T03: k6 performance suites + NFR sign-off inputs
**Status:** pending
**Background:** Prove 10K TPS / p99<50ms (`features §14`) before hardening (E19 tunes).
**Files:**
- Create: `test/performance/k6/{baseline.js,load.js,stress.js,soak.js,http3.js}`, `test/performance/thresholds.md`
**Steps:**
1. Baseline (PR-gate, light): p95<200ms, errors<1%.
2. Load/stress/soak (nightly): ramp to NFR targets; record transfer-post p99, balance-read p99, webhook lag.
3. Thresholds doc maps each NFR row to a suite + pass criteria.
**Acceptance Criteria:**
- [ ] Baseline suite passes in CI on every PR.
- [ ] Nightly load run produces a report artifact (even if NFRs not yet met — E19 closes).
**Story Points:** 4
**Depends On:** E11-T09
**Related Docs:** `SPEC.md §10.4`, `docs/fintech-ledger-features.md §14`, `SPEC.md §2` (k6)
**SDD Gate:** G7

---

### E16-T04: Litmus chaos scenarios
**Status:** pending
**Background:** Resilience proof (`SPEC.md §10.5`, features §11.2): pod kill,
latency, partition, disk, CPU, DNS.
**Files:**
- Create: `test/chaos/litmus/{pod-kill.yaml,network-latency.yaml,partition.yaml,disk-fill.yaml,cpu-hog.yaml,dns-failure.yaml}`,
  `test/chaos/expectations.md`
**Steps:**
1. One experiment file per scenario, scoped to staging namespace.
2. Expectations doc: no data loss, auto-recovery <30s, alerts fire (link each to G6 alerts).
3. Weekly staging run (E17 schedules); manual CI trigger.
**Acceptance Criteria:**
- [ ] Each scenario has an expectation entry traceable to an alert + a recovery bound.
- [ ] Dry-run (`kubectl apply --dry-run`) passes in CI lint stage.
**Story Points:** 3
**Depends On:** E15-T02
**Related Docs:** `SPEC.md §10.5`, `docs/fintech-ledger-features.md §11.2`
**SDD Gate:** G7

---

### E16-T05: Coverage bars + gate wiring (G7 slice)
**Status:** pending
**Background:** Make G2/G3/G7 coverage bars enforceable in CI.
**Files:**
- Modify: `.github/workflows/ci.yml` (coverage upload + thresholds), `tasks/scripts/gate-check.sh`
**Steps:**
1. Enforce: domain ≥90%, application ≥85%, overall ≥80%; fail PR otherwise.
2. Coverage artifacts (HTML + XML) uploaded per run.
**Acceptance Criteria:**
- [ ] A deliberately under-tested PR fails the gate (verified once, reverted).
**Story Points:** 3
**Depends On:** E07-T09, E02-T09, E06-T08, E07.1-T05
**Related Docs:** `SPEC.md §10.2`, `SPEC.md §16`
**SDD Gate:** G7

## Acceptance Criteria

- [ ] E16-T02 … E16-T05 all `completed` (count 14 SP in `tasks/tracking/PROGRESS.md`)
- [ ] Harness shared by all suites; contracts, baseline perf, and chaos expectations all green/tracked
- [ ] SDD gate G7 checks pass — `tasks/tracking/GATES.md#G7`
