# Epic E12: gRPC API

**Status:** pending
**Story Points:** 16
**Phase:** 6 (parallel with E11, E13, E14)
**Dependencies:** E07, E08, E09, E10
**SDD Gate:** G5
**Design refs:** `SPEC.md §8.2`, `docs/api-contracts.md §8` (+ parity rule),
`SPEC.md §9.5–§9.8`

> Why a separate epic: the high-performance internal API with its own IDL,
> interceptors, and gateway — kept apart from REST so proto/governance changes
> stay reviewable on their own.

## Tasks

### E12-T01: Proto definitions + buf pipeline
**Status:** pending
**Background:** Schema-first services (`SPEC.md §8.2`); REST parity rule requires
scheduled fields + batch messages from day one.
**Files:**
- Create: `api/proto/ledger/v1/{common,account,transaction,transfer,payment,refund,payout,reconciliation,period,report}.proto`,
  `buf.yaml`, `buf.gen.yaml`; Output: `api/proto/gen/go/...`
**Steps:**
1. Services with exactly these methods (features §7.2 — no fewer):
   LedgerService{PostTransaction (maps to PostLedgerPosting), GetBalance, GetAccount},
   TransferService{CreateTransfer, GetTransfer, ListTransfers, CreateBatchTransfer, GetBatchStatus},
   ReportingService{StreamReport, GetReportStatus},
   ReconciliationService{RunReconciliation, GetBreaks}.
2. Messages carry `execute_at`/`recurrence`, idempotency keys, cursor pagination, error details with codes.
3. `make generate` runs `buf generate`; `buf lint` + `buf breaking` in CI.
**Acceptance Criteria:**
- [ ] `buf breaking --against main` clean on first commit (baseline).
- [ ] All 10 methods above present with request/response pairs (review vs features §7.2).
- [ ] Batch + scheduled fields present in transfer messages (review vs api-contracts §7.5).
**Story Points:** 4
**Depends On:** E06-T06
**Related Docs:** `SPEC.md §8.2`, `SPEC.md §2` (grpc-go v1.66.0), `docs/api-contracts.md §8`, `docs/fintech-ledger-features.md §7.2`
**SDD Gate:** G5

---

### E12-T02: gRPC server + interceptor chain + gateway
**Status:** pending
**Background:** Same cross-cutting behavior as REST, in interceptor form.
**Files:**
- Create: `pkg/grpcserver/{server.go,gateway.go}`,
  `pkg/grpcserver/interceptors/{recovery,logger,tracer,auth,ratelimit,featureflag,locale}.go`,
  `cmd/grpc-api/main.go`
**Steps:**
1. Interceptor order mirrors E11-T01; recovery maps panics to `codes.Internal` + `request_id` metadata (SPEC §9.7).
2. gRPC-Gateway REST proxy + health (`grpc.health.v1`) + reflection (debug builds).
3. Graceful shutdown via fx hooks.
**Acceptance Criteria:**
- [ ] Panic in handler → `Internal` + request_id metadata, metric + alert fired (test).
- [ ] Unauthenticated call → `Unauthenticated`; out-of-scope → `PermissionDenied` (tests).
**Story Points:** 4
**Depends On:** E12-T01, E09-T01, E09-T03
**Related Docs:** `SPEC.md §8.2`, `SPEC.md §9.5–§9.8`
**SDD Gate:** G5

---

### E12-T03: Service implementations (Ledger/Transfer/Reporting/Reconciliation)
**Status:** pending
**Background:** Thin adapters over E06 use-cases; no business logic here.
**Files:**
- Create: `internal/interface/grpc/{ledger_server.go,transfer_server.go,reporting_server.go,reconciliation_server.go}`
**Steps:**
1. Each method: proto-validate → map to command/query → call use-case → map result/error to status codes.
2. ReportingService streams report bytes/chunks with progress messages.
**Acceptance Criteria:**
- [ ] Every rpc in the proto files has an implemented method (compile + reflection test).
- [ ] Error codes propagate (e.g., INSUFFICIENT_FUNDS → FailedPrecondition with details) (tests).
**Story Points:** 5
**Depends On:** E12-T02, E06-T02, E06-T03, E06-T04, E06-T05
**Related Docs:** `docs/api-contracts.md §8`, `tasks/epics/E06-application.md`
**SDD Gate:** G5

---

### E12-T04: gRPC integration + contract tests (G5 slice)
**Status:** pending
**Background:** G5 evidence for gRPC.
**Files:**
- Create: `test/integration/api/grpc/...`
**Steps:**
1. Generated clients against Testcontainers stack: success/validation/auth/rate-limit/idempotency per method.
2. Gateway proxy smoke tests (REST→gRPC mapping).
**Acceptance Criteria:**
- [ ] `go test ./test/integration/api/grpc/... -race -count=3` green.
**Story Points:** 3
**Depends On:** E12-T03
**Related Docs:** `SPEC.md §10.3`, `SPEC.md §11`
**SDD Gate:** G5

## Acceptance Criteria

- [ ] E12-T01 … E12-T04 all `completed` (count 16 SP in `tasks/tracking/PROGRESS.md`)
- [ ] Proto = REST parity; gateway serves the same contracts; breaking-change CI guard active
- [ ] SDD gate G5 checks pass — `tasks/tracking/GATES.md#G5`
