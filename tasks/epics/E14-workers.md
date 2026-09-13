# Epic E14: Workers (Cron Binary + Consumer Binary)

**Status:** pending
**Story Points:** 16
**Phase:** 6 (parallel with E11, E12, E13)
**Dependencies:** E07, E08, E09, E10
**SDD Gate:** G5
**Design refs:** `SPEC.md §8.4–§8.5`, `docs/fintech-ledger-features.md §9`,
`docs/user-journeys.md §2.3`, `docs/domain-events.md §5, §7`

> Why a separate epic: scheduled jobs and event consumers are the system's
> background half — leader coordination, idempotent execution, and DLQ ops have
> failure modes the request/response paths never see.

## Tasks

### E14-T01: Cron binary + distributed scheduling
**Status:** pending
**Background:** `cmd/cron` runs all 8 features-§9 jobs with Valkey Redlock
leadership. The lock is coordination-only; every job also has a durable run key
and is safe to retry after lease loss.
**Files:**
- Create: `cmd/cron/main.go`, `internal/interface/cron/{scheduler.go,jobs.go,registry.go}`
**Steps:**
1. gocron v1.5.0 scheduler; per-job Redlock (30s TTL, auto-renew, release on done/fail) as an optimization.
2. Each occurrence has a durable unique run key and idempotent step keys; a
   fencing/lease check prevents a stale leader from committing protected effects.
3. Job records (run key, last/next run, status, duration, error) persisted.
4. Panic wrapper per SPEC §9.7 (FAILED + unlock + alert); health endpoints.
**Acceptance Criteria:**
- [ ] Two replicas/retries → one durable effect per run key (Testcontainers test).
- [ ] Crashed holder's work picked up after TTL (test).
**Story Points:** 4
**Depends On:** E08-T02, E06-T07
**Related Docs:** `SPEC.md §8.4`, `SPEC.md §2` (gocron v1.5.0), `docs/fintech-ledger-features.md §9`
**SDD Gate:** G5

---

### E14-T02: The 8 scheduled jobs
**Status:** pending
**Background:** One task per features-§9 row — Nemotron had a single vague "job definitions" task.
**Files:**
- Create: `internal/interface/cron/jobs/{reconciliation,interest,fee,period_close,report,archival,fx_rates,compliance}.go`
**Steps:**
1. Daily Reconciliation 02:00 UTC → E06 reconciliation saga (journeys §2.3).
2. Interest Accrual daily → E03-T04 rules via saga.
3. Fee Calculation monthly → E03-T04 batch posting.
4. Period Close month-end → E04-T03 validation + E06 saga.
5. Report Generation daily/monthly → pre-render features-§6 templates.
6. Data Archival quarterly → cold-storage move per data-flow §7 retention.
7. FX Rate Update hourly → E10-T02 fetch + cache.
8. Compliance Scans daily → E04-T05 screening queue; includes dispute-evidence-deadline
   sweeps (auto-close overdue `EVIDENCE_DUE` disputes per E03-T07 windows).
**Acceptance Criteria:**
- [ ] Each job: dry-run mode + idempotent re-run (tests with fakes).
- [ ] Missed-tick policy documented per job (catch-up vs skip) and tested.
**Story Points:** 5
**Depends On:** E14-T01, E06-T07
**Related Docs:** `docs/fintech-ledger-features.md §9`, `docs/user-journeys.md §2.3, §2.6`, `docs/data-flow.md §7`
**SDD Gate:** G5

---

### E14-T03: Consumer binary + 4 consumer groups + DLQ ops
**Status:** pending
**Background:** Separately-deployable `cmd/consumer` (SPEC §8.5, §4.3 in original numbering).
**Files:**
- Create: `cmd/consumer/main.go`,
  `internal/interface/consumer/{webhook.go,analytics.go,audit.go,reconciliation.go,dlq_ops.go}`
**Steps:**
1. Groups: webhook-dispatcher, analytics-pipeline, audit-logger, reconciliation-engine
   (explicit ack, max_deliver=5, ack_wait=30s, idempotent handlers).
2. DLQ ops: inspect, replay by event-id range, alert hooks (domain-events §7).
3. Horizontal scaling verified: N replicas share load (NATS queue groups).
4. Health: NATS + Valkey reachability gates.
**Acceptance Criteria:**
- [ ] Poison message lands in DLQ after max delivers; replay redelivers it (test).
- [ ] Duplicate delivery executes handler once (idempotency test).
**Story Points:** 4
**Depends On:** E08-T03, E08-T04
**Related Docs:** `SPEC.md §8.5`, `docs/domain-events.md §5, §7`
**SDD Gate:** G5

---

### E14-T04: Worker integration tests (G5 slice)
**Status:** pending
**Background:** G5 evidence for workers.
**Files:**
- Create: `test/integration/workers/...`
**Steps:**
1. Cron fires each job against Testcontainers; consumer groups process published events end-to-end.
2. Lock contention, crash-resume, DLQ replay covered.
**Acceptance Criteria:**
- [ ] `go test ./test/integration/workers/... -race -count=3` green.
**Story Points:** 3
**Depends On:** E14-T02, E14-T03
**Related Docs:** `SPEC.md §10.3`
**SDD Gate:** G5

## Acceptance Criteria

- [ ] E14-T01 … E14-T04 all `completed` (count 16 SP in `tasks/tracking/PROGRESS.md`)
- [ ] All 8 jobs are at-least-once invocable and idempotent under contention; all 4 consumer groups drain with DLQ coverage
- [ ] SDD gate G5 checks pass — `tasks/tracking/GATES.md#G5`
