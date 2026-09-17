# Epic E11: REST API (Echo)

**Status:** pending
**Story Points:** 43
**Phase:** 6 (parallel with E12, E13, E14)
**Dependencies:** Task-level E06–E10 contracts; local ledger pilot precedes the public edge
**SDD Gate:** G5
**Design refs:** `SPEC.md §8.1`, `SPEC.md §9.2–§9.8`, `docs/api-contracts.md §1–§7, §11–§13`,
`docs/data-flow.md §2`, `docs/user-journeys.md §2–§4`

> Convention (from verification): paths below omit `/v1`; they map 1:1 to
> `docs/api-contracts.md §7` (`POST /accounts` → `POST /v1/accounts`).

## Tasks

### E11-T01: Echo server core and lifecycle
**Status:** pending
**Background:** Establish a small testable HTTP boundary for the ledger pilot;
public authentication, rate limits, feature flags, and hardening are E11-T15.
**Files:**
- Create: `pkg/httpserver/server.go`,
  `pkg/httpserver/middleware/{requestid.go,recovery.go,logger.go,tracer.go}`,
  `cmd/rest-api/main.go`
**Steps:**
1. Factory with bounded timeouts and graceful shutdown via fx hooks.
2. Install RequestID → Recovery → Logger → Tracer in deterministic order.
3. Recovery logs stack/trace internally and returns a safe 500 envelope.
4. Expose route registration without importing infrastructure implementations.
**Acceptance Criteria:**
- [ ] Core middleware order and lifecycle tests pass.
- [ ] Panic in a handler returns 500 envelope with request_id and no stack (test).
- [ ] The focused E11-T14 test can boot without Valkey, NATS, or provider credentials.
**Story Points:** 2
**Depends On:** E01-T02, E01-T04, E01-T05, E07.1-T01
**Related Docs:** `SPEC.md §8.1`, `SPEC.md §9.6–§9.7`, `docs/data-flow.md §2`
**SDD Gate:** G5

---

### E11-T02: Standard envelope, errors, pagination, filtering
**Status:** pending
**Background:** Cross-cutting REST mechanics from api-contracts §2–§6 so endpoint
tasks don't re-implement them.
**Files:**
- Create: `pkg/httpserver/{envelope.go,errors.go,pagination.go,filter.go}`
**Steps:**
1. `Response[T]{data,error,meta,request_id}` + error mapping AppError→HTTP status;
   validation errors → `VALIDATION_FAILED` with field list.
2. Cursor (preferred) + offset pagination; `filter[field][op]`, `sort`, `include` parsing.
3. Response headers: rate-limit trio, `X-Request-ID`, `Retry-After` on 429.
**Acceptance Criteria:**
- [ ] Every error code in api-contracts §4 maps to the documented HTTP status (table test).
- [ ] Pagination/filter parsers fuzz-tested against malformed input (no panics).
**Story Points:** 3
**Depends On:** E11-T01, E01-T06
**Related Docs:** `docs/api-contracts.md §2–§6`, `SPEC.md §9.1–§9.3`
**SDD Gate:** G5

---

### E11-T03: Accounts + tenants endpoints
**Status:** pending
**Background:** api-contracts accounts + tenants groups; journeys §2.1 onboarding.
**Files:**
- Create: `internal/interface/rest/{accounts.go,tenants.go}`, `cmd/rest-api/routes.go` (register)
**Steps:**
1. Accounts: create/get/list/update/freeze/unfreeze/close/balance/entries/statements.
2. Tenants: provision/get/update/list (platform-guarded).
3. External bank-account routes (separate from ledger accounts): create tokenized
   instrument, start microdeposits, confirm amounts, rate-limit attempts.
4. Currency/type enums validated; idempotency on writes; OpenAPI `accounts`+`tenants` tags.
**Acceptance Criteria:**
- [ ] Onboarding journey §2.1 executable end-to-end against these endpoints (test).
- [ ] Hierarchy + statements query params covered (tests).
- [ ] Wrong microdeposit amounts rejected; attempts rate-limited (tests).
**Story Points:** 3
**Depends On:** E11-T02, E11-T15, E06-T02
**Related Docs:** `docs/api-contracts.md §7` (accounts, tenants), `docs/user-journeys.md §2.1`
**SDD Gate:** G5

---

### E11-T04: Transactions + transfers incl. scheduled/bulk
**Status:** pending
**Background:** api-contracts transactions + transfers groups; money-flow §2.3/§2.8/§2.9.
**Files:**
- Create: `internal/interface/rest/{transactions.go,transfers.go,batch.go}`
**Steps:**
1. Transfers: immediate + `execute_at`/`recurrence`; cancel; batch create (≤1000) + batch status with per-item results.
2. Double-entry validation errors surface field-level details; the restricted
   posting endpoints are isolated in E11-T14.
3. Export: `GET /v1/transactions/export?format=csv&...` (sync under 10k rows;
   async job + signed URL above that); same exporter reused for entries and
   account statements (E11-T03 wires it into its endpoints).
**Acceptance Criteria:**
- [ ] Batch with mixed valid/invalid items returns PARTIAL with per-item results (test).
- [ ] Scheduled transfer persists PENDING via API (test).
- [ ] Export round-trips: exported CSV re-imports through validation cleanly (test).
**Story Points:** 5
**Depends On:** E11-T02, E11-T15, E06-T03
**Related Docs:** `docs/api-contracts.md §7` (transactions, transfers), `docs/money-flow.md §2.3, §2.8, §2.9`, `docs/user-journeys.md §2.2`
**SDD Gate:** G5

---

### E11-T05: Payments, refunds, payouts endpoints
**Status:** pending
**Background:** api-contracts payments/refunds/payouts; money-flow §2.1/§2.2/§2.4; journeys §2.4–§2.5.
**Files:**
- Create: `internal/interface/rest/{payments.go,refunds.go,payouts.go}`
**Steps:**
1. Payment intents create/get/confirm/cancel (+ client token, method enum, idempotency on create+confirm).
   Confirm maps declines to 402 `card_error` (+ `decline_code`) and challenges to `requires_action`.
   Capture endpoint enforces E03-T08 rules (full/partial/expiry).
2. Refunds create/get/list with original-txn/date/status filters.
3. Payouts create/get/list/cancel with method enum + destination object; schedule
   get/update; `instant` flag with quoted fee; minimums, first-payout holds,
   reserves, and negative-balance policy enforced (`PAYOUT_MINIMUM_NOT_MET` or
   `PAYOUT_BLOCKED`).
4. Top-ups create/get/list/cancel via E03-T10 (verified accounts only).
**Acceptance Criteria:**
- [ ] Refund journey §2.4 and multi-currency journey §2.5 executable via API (tests).
- [ ] Method enums reject unknown values with `VALIDATION_FAILED` (test).
**Story Points:** 4
**Depends On:** E11-T02, E11-T15, E06-T04
**Related Docs:** `docs/api-contracts.md §7` (payments, refunds, payouts), `docs/money-flow.md §2.1–§2.2, §2.4`, `docs/user-journeys.md §2.4–§2.5`
**SDD Gate:** G5

---

### E11-T06: Reconciliation, periods, reports endpoints
**Status:** pending
**Background:** Ops surface for journeys §2.3/§2.6 and features §4.2/§6.
**Files:**
- Create: `internal/interface/rest/{reconciliation.go,periods.go,reports.go}`
**Steps:**
1. Reconciliation runs/breaks/resolve/acknowledge with filters.
2. Periods list/get/close/reopen (close returns ALL failing checks).
3. Reports generate/status/list/templates; async with signed URL + `report.generated` webhook.
**Acceptance Criteria:**
- [ ] Period close with 3 simultaneous violations returns all 3 (test).
- [ ] All 10 features-§6 report types accepted by generate endpoint (parameterized test).
**Story Points:** 3
**Depends On:** E11-T02, E11-T15, E06-T05
**Related Docs:** `docs/api-contracts.md §7` (reconciliation, periods, reports), `docs/user-journeys.md §2.3, §2.6`, `docs/fintech-ledger-features.md §4.2, §6`
**SDD Gate:** G5

---

### E11-T07: Webhook subscription management + delivery security
**Status:** pending
**Background:** api-contracts §10–§11; dispatcher built in E08-T05, registry+security here.
**Files:**
- Create: `internal/interface/rest/webhooks.go`
**Steps:**
1. CRUD for subscriptions: URL, events[], secret, retry policy.
2. Outbound signing verification test-vector from api-contracts §11 passes.
3. Inbound processor webhooks (E10-T03) verified here at the edge.
**Acceptance Criteria:**
- [ ] Signature example from docs verifies byte-for-byte (test).
- [ ] Retry schedule 1m→5m→15m→1h→6h→24h→48h then DLQ (test with fake endpoint).
**Story Points:** 3
**Depends On:** E11-T02, E11-T15, E08-T05
**Related Docs:** `docs/api-contracts.md §10–§11`
**SDD Gate:** G5

---

### E11-T08: OpenAPI generation + contract validation
**Status:** pending
**Background:** Contract-as-code (`SPEC.md §11`); annotations written alongside handlers above.
**Files:**
- Create: `scripts/generate/openapi.sh`; Output: `api/openapi/openapi.yaml`
**Steps:**
1. Generate OpenAPI 3.1 from handler annotations; `spectral lint` clean.
2. Serve Swagger UI at `/docs/swagger`; contract tests compare live handlers vs spec.
**Acceptance Criteria:**
- [ ] Generated spec validates (`oapi-codegen -validate` + spectral).
- [ ] Every §7 endpoint appears in the spec (`check-tasks.py --openapi`).
**Story Points:** 2
**Depends On:** E11-T03, E11-T04, E11-T05, E11-T06, E11-T07, E11-T14
**Related Docs:** `SPEC.md §11`, `SPEC.md §9.2`, `docs/api-contracts.md §1`
**SDD Gate:** G5

---

### E11-T09: REST integration + contract tests (G5 slice)
**Status:** pending
**Background:** G5 evidence for REST.
**Files:**
- Create: `test/integration/api/rest/...`
**Steps:**
1. Testcontainers (Postgres, Valkey, NATS, Unleash): 2xx/4xx/5xx per endpoint,
   validation, auth matrix, idempotency replay, rate-limit 429, cursor pagination.
2. Pact consumer tests vs OpenAPI; notebook journeys §2.1/§2.2/§2.4 as end-to-end cases.
**Acceptance Criteria:**
- [ ] `go test ./test/integration/api/rest/... -race -count=3` green.
**Story Points:** 4
**Depends On:** E11-T03, E11-T04, E11-T05, E11-T06, E11-T07, E11-T08, E11-T14
**Related Docs:** `SPEC.md §10.3, §10.6`, `docs/user-journeys.md §2–§4`
**SDD Gate:** G5

---

### E11-T10: Operational endpoints (health, readiness, version negotiation)
**Status:** pending
**Background:** Every binary needs probe endpoints and the versioning strategy
from api-contracts §13 (URL path + `Accept-Version` fallback + deprecation
headers) needs an implementation, not just a doc.
**Files:**
- Create: `pkg/httpserver/health.go` (reused by REST + GraphQL binaries)
**Steps:**
1. `GET /healthz` (liveness: process alive), `GET /readyz` (readiness: DB +
   Valkey + NATS reachable; used by K8s probes and Traefik healthchecks).
2. Version negotiation: `Accept-Version` header falls back when the URL has no
   version; `Sunset` + `Deprecation` headers on deprecated versions per §13.
**Acceptance Criteria:**
- [ ] Unhealthy dependency flips `/readyz` with the failing component named (test with stopped Valkey).
- [ ] `Accept-Version: 1` routes identically to `/v1/...`; deprecated version emits `Deprecation` + `Sunset` (tests).
**Story Points:** 2
**Depends On:** E11-T01
**Related Docs:** `docs/api-contracts.md §13`, `SPEC.md §8.1`
**SDD Gate:** G5

---

### E11-T11: Tenant branding + custom-domain resolution (P2 — may slip post-MVP)
**Status:** pending
**Background:** Features §5 white-labeling (P2): serve per-tenant branding and
route custom domains. Settings shape exists (E05-T03); this serves it.
**Files:**
- Create: `pkg/httpserver/middleware/branding.go`
**Steps:**
1. Resolve tenant from Host header (custom domain map) or default domain + path/query.
2. Attach branding (theme, logo URLs) to responses that render them; unknown domain → default branding, never an error.
3. Document Traefik custom-domain routing hook for E17-T03.
**Acceptance Criteria:**
- [ ] Custom-domain request resolves the right tenant's branding (test).
- [ ] Unknown domain falls back safely (test).
**Story Points:** 2
**Depends On:** E05-T03, E11-T01, E11-T15
**Related Docs:** `docs/fintech-ledger-features.md §5`
**SDD Gate:** G5

---

### E11-T12: Dispute endpoints
**Status:** pending
**Background:** api-contracts §7.11 + journeys §2.7, backed by E06-T11.
**Files:**
- Create: `internal/interface/rest/disputes.go`
**Steps:**
1. Open (manual/test path; network path arrives via processor webhooks in E10-T03),
   get (with deadline + fee), list (status/date filters), evidence submit,
   represent (once), close-as-lost.
2. Map domain errors to the dispute codes (`DISPUTE_ACTION_INVALID`, `DISPUTE_WINDOW_EXPIRED`).
**Acceptance Criteria:**
- [ ] Dispute journey §2.7 executable end-to-end via API (test).
**Story Points:** 3
**Depends On:** E11-T02, E11-T15, E06-T11
**Related Docs:** `docs/api-contracts.md §7` (disputes), `docs/user-journeys.md §2.7`, `docs/money-flow.md §2.11`
**SDD Gate:** G5

---

### E11-T13: Search endpoints
**Status:** pending
**Background:** api-contracts §7.13: `field:value` query language over
transactions/accounts without a search cluster.
**Files:**
- Create: `internal/interface/rest/search.go`
**Steps:**
1. Parse `field:value`, AND/OR, quoted phrases, `*` prefix wildcard, ranges;
   reject unknown fields with `VALIDATION_FAILED` (never SQL-injectable — allowlisted fields only).
2. Back with Postgres full-text/trigram indexes (migrations in E07-T01 scope — coordinate).
3. Same envelope + cursor pagination as list endpoints.
**Acceptance Criteria:**
- [ ] Injection-style queries return validation errors, never raw DB errors (adversarial test).
- [ ] Unknown field rejected with the field named (test).
**Story Points:** 2
**Depends On:** E11-T02, E11-T15
**Related Docs:** `docs/api-contracts.md §7` (search), `docs/fintech-ledger-features.md §7.1`
**SDD Gate:** G5

---

### E11-T14: Restricted ledger pilot endpoints
**Status:** pending
**Background:** Prove one end-to-end financial write/read slice before exposing
transfers, provider workflows, reports, GraphQL, or gRPC. This is an internal
operator/service contract, never arbitrary merchant journal CRUD.
**Files:**
- Create: `internal/interface/rest/{postings.go,balances.go,entries.go}` and focused integration tests
**Steps:**
1. Implement template-based post/get/list/reverse plus strong balance and
   cursor-paginated entry reads through E06-T13.
2. Require service authorization, tenant/ledger context, and durable
   `Idempotency-Key`; reject arbitrary account-side combinations.
3. Return integer minor units, asset code, account sequence, cursor, and as-of.
4. Test against PostgreSQL through the real E07-T01 posting path, including
   concurrent spend and crash/replay boundaries.
**Acceptance Criteria:**
- [ ] Capture-style posting → strong balance → entry list works end-to-end.
- [ ] Identical replay returns the original response; changed fingerprint conflicts.
- [ ] Unbalanced, cross-tenant, unauthorized, update, and delete attempts fail.
- [ ] The focused vertical-slice test runs without Valkey, NATS, or a live provider.
**Story Points:** 3
**Depends On:** E11-T02, E06-T13, E07-T01, E07-T03, E07-T09
**Related Docs:** `docs/ledger-core.md §3–§9`, `docs/api-contracts.md §7`, `docs/data-flow.md §2`
**SDD Gate:** G5

---

### E11-T15: Public REST middleware and transport hardening
**Status:** pending
**Background:** Public endpoints require the complete security and policy chain,
but that integration should not block the local ledger-core proof.
**Files:**
- Create: `pkg/httpserver/http3.go`,
  `pkg/httpserver/middleware/{ratelimit.go,auth.go,featureflag.go,locale.go,validation.go,secure.go}`
**Steps:**
1. Extend the core chain to RequestID → Recovery → Logger → Tracer → RateLimit →
   Auth → FeatureFlag → Locale → Validation with an explicit route policy matrix.
2. Add CORS/CSP/HSTS and other OWASP 2025 A02 secure defaults; reject missing or
   ambiguous tenant/authentication context.
3. Add TLS configuration and optional HTTP/3 behind a disabled-by-default flag.
4. Test degraded cache/flag-provider behavior and ensure authorization fails closed.
**Acceptance Criteria:**
- [ ] Middleware-order regression test covers the full public chain.
- [ ] Missing/invalid JWT returns 401; insufficient scope returns 403.
- [ ] Secure-header/TLS tests pass and no internal pilot route is publicly registered.
**Story Points:** 2
**Depends On:** E11-T01, E09-T01, E09-T03, E08-T01, E08-T07, E10-T01
**Related Docs:** `SPEC.md §9.5–§9.8`, `SPEC.md §14.1`, `docs/data-flow.md §2`
**SDD Gate:** G5

## Acceptance Criteria

- [ ] E11-T01 … E11-T15 all `completed` (count 43 SP in `tasks/tracking/PROGRESS.md`)
- [ ] Every api-contracts §7 endpoint live, documented in OpenAPI, and covered by integration + contract tests
- [ ] SDD gate G5 checks pass — `tasks/tracking/GATES.md#G5`
