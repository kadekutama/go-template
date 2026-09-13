# Epic E08: Cache + Messaging Adapters

**Status:** pending
**Story Points:** 24
**Phase:** 5 (parallel with E07, E09, E10)
**Dependencies:** E06 (ports), E05-T03 (key/subject conventions)
**SDD Gate:** G4
**Design refs:** `SPEC.md §7.3`, `SPEC.md §7.5`, `docs/money-flow.md §7`,
`docs/data-flow.md §2`, `docs/domain-events.md §4–§5, §7`, `docs/user-journeys.md §2.2` (Redlock)

> Why a separate epic: cache correctness (invalidation, TTLs) and messaging
> reliability (topology, DLQ, idempotency, webhooks) are the two async-safety
> pillars — grouping them keeps every delivery guarantee in one reviewable place.

## Tasks

### E08-T01: Ristretto L1 + Valkey L2 + hybrid cache
**Status:** pending
**Background:** Cache-aside with post-commit invalidation (`SPEC.md §7.3`,
money-flow §7). Client library is go-redis v9.22.0 against Valkey 9.0.6.
**Files:**
- Create: `internal/infrastructure/cache/local/ristretto.go`,
  `internal/infrastructure/cache/valkey/client.go`,
  `internal/infrastructure/cache/hybrid/cache.go` (implements `Cache` port)
**Steps:**
1. Ristretto v2.4.2 (NumCounters, MaxCost≈100MB, BufferItems per SPEC).
2. go-redis client: pool sizing, timeouts, TLS optional; Valkey 9.0.6 compat verified.
3. Hybrid: L1→L2→loader fallback; populate cache after a read miss; invalidate
   both layers only after the authoritative database commit; per-kind TTLs
   (display balances 1m/5m with cursor/as-of, configs 5m/30m,
   FX 1h, completed idempotency-response hints 24h, rate-limit 1m).
   Cache values never authorize spending or establish uniqueness.
4. Key builders from E05-T03 (never hand-concatenate keys in callers).
5. Implements: `Cache` port (contract: E06-T12).
**Acceptance Criteria:**
- [ ] L1 hit <1ms, L2 hit <5ms in benchmark test (documents money-flow §7 claims).
- [ ] Write invalidates both layers (stale-read test).
- [ ] No `redis.call`-style Lua outside the rate-limit script (review).
**Story Points:** 5
**Depends On:** E06-T12, E05-T03
**Related Docs:** `SPEC.md §7.3`, `SPEC.md §2` (Ristretto v2.4.2, Valkey 9.0.6, go-redis v9.22.0), `docs/money-flow.md §7`, `docs/data-flow.md §2`
**SDD Gate:** G4

---

### E08-T02: Distributed locks (Redlock over Valkey, coordination only)
**Status:** pending
**Background:** Scheduler/reconciliation coordination and duplicate-work suppression.
PostgreSQL locking/uniqueness remains the money-safety boundary.
**Files:**
- Create: `internal/infrastructure/lock/{redlock.go,guard.go}`
**Steps:**
1. Redlock acquire (TTL 30s default), auto-renewal while held, release on
   completion/failure. Never use the lock as the source of funds, idempotency,
   or uniqueness; database constraints remain authoritative.
2. `WithLock(ctx, key, ttl, fencingToken, fn)` guard used by cron/reconciliation;
   protected durable writes validate lease/fencing or an idempotent run key.
3. Lock-contention metric + `CONCURRENT_TRANSFER`-style typed errors.
4. Implements: `Locker` port (contract: E06-T12).
**Acceptance Criteria:**
- [ ] Two contenders may retry, but one durable run/effect key commits (Testcontainers concurrency test).
- [ ] Valkey loss/expiry cannot duplicate a posting or permit overspend.
**Story Points:** 3
**Depends On:** E08-T01
**Related Docs:** `docs/user-journeys.md §2.2–§2.3`, `docs/api-contracts.md §4` (CONCURRENT_TRANSFER), `docs/money-flow.md §8`
**SDD Gate:** G4

---

### E08-T03: NATS JetStream topology (streams, subjects, DLQ)
**Status:** pending
**Background:** Streams, tenant-scoped subjects, consumer groups, DLQ
(`docs/domain-events.md §4–§5`, data-flow §2).
**Files:**
- Create: `internal/infrastructure/messaging/nats/{streams.go,subjects.go}`,
  `deployments/nats/` (stream/consumer bootstrap configs)
**Steps:**
1. Provision the canonical `LEDGER_EVENTS` stream used by the consumer
   configurations (7-day retention, 10M messages). Separate streams are an
   optional later optimization and require an ADR plus retention/order proof;
   the PostgreSQL outbox remains the durable event archive.
2. Subjects `ledger.{tenant}.{event_type}` (where the versioned event type carries
   domain/entity/action) via E05-T03 builders;
   platform wildcard subscriptions documented.
3. Consumer groups: webhook-dispatcher, analytics-pipeline, audit-logger,
   reconciliation-engine (ack policy, max_deliver=5, ack_wait=30s, DLQ each).
4. TLS + nkeys/JWT auth; reconnect with backoff.
5. Implements: topology backing the `EventPublisher`/`EventConsumer` ports (contract: E06-T12).
**Acceptance Criteria:**
- [ ] `check-tasks.py --subjects` confirms every domain event has a stream+subject.
- [ ] DLQ receives poison messages after max delivers (integration test).
**Story Points:** 4
**Depends On:** E06-T12, E05-T03
**Related Docs:** `SPEC.md §7.5`, `docs/domain-events.md §4–§5`, `docs/data-flow.md §2`, `SPEC.md §2` (NATS 2.14.6)
**SDD Gate:** G4

---

### E08-T04: Publisher (outbox-driven) + idempotent consumer framework
**Status:** pending
**Background:** `subjectForEvent(tenant, type)` with versioned envelope/headers;
consumer dedupes with a durable PostgreSQL inbox committed with its side effects.
**Files:**
- Create: `internal/infrastructure/messaging/nats/publisher/jetstream.go`,
  `internal/infrastructure/messaging/nats/consumer/{framework.go,idempotent.go,dlq.go}`
**Steps:**
1. Publisher: encode through `pkg/jsonparser`, headers, `AckWait(30s)`, called
   by E07 outbox poller.
2. Consumer framework: validate envelope/schema → open transaction → insert
   `(consumer,event_id)` inbox receipt → handler side effects → commit → ack.
   Duplicate receipt skips effects and acks; failure rolls back, retries, then DLQ.
3. Trace propagation both directions (E01-T05 helpers).
4. Implements: `EventPublisher` + consumer `MessageHandler` ports (contract: E06-T12).
**Acceptance Criteria:**
- [ ] Duplicate delivery executes handler once (kill+redeliver test).
- [ ] Trace ID survives HTTP→NATS→consumer→DB (assert in test via headers).
**Story Points:** 4
**Depends On:** E08-T03, E07-T03
**Related Docs:** `docs/domain-events.md §4–§5, §7`, `docs/data-flow.md §2`
**SDD Gate:** G4

---

### E08-T05: Webhook dispatcher consumer
**Status:** pending
**Background:** Delivers all 24+ webhook types (api-contracts §10) to merchant
endpoints with HMAC signatures and retry schedule.
**Files:**
- Create: `internal/infrastructure/messaging/nats/consumer/webhook_dispatcher.go`,
  `internal/infrastructure/webhook/{signer.go,registry.go,retry.go}`
**Steps:**
1. Endpoint registry (CRUD backing the webhook-mgmt API in E11): URL, events[], secret, retry policy.
2. Signer: HMAC over `timestamp + "." + raw_body`, `v1=` signature values,
   timestamp tolerance, key ID, constant-time verification, dual-secret rotation.
3. Retry: 1m → 5m → 15m → 1h → 6h → 24h → 48h (7 attempts, ~72h horizon), then DLQ; per-endpoint circuit breaking.
4. Implements: `EventConsumer` port for the delivery group (contract: E06-T12); reads endpoint config via `WebhookSubscriptionRepository`.
**Acceptance Criteria:**
- [ ] Signature verification/replay-tolerance example in api-contracts §11 passes; old/new keys overlap during rotation.
- [ ] Failing endpoint doesn't block other events (isolation test).
**Story Points:** 3
**Depends On:** E08-T04
**Related Docs:** `docs/api-contracts.md §10–§11`, `docs/user-journeys.md §2.1`
**SDD Gate:** G4

---

### E08-T06: Cache + messaging integration tests (G4 slice)
**Status:** pending
**Background:** G4 evidence for this adapter family.
**Files:**
- Create: `test/integration/cache/...`, `test/integration/messaging/...`
**Steps:**
1. Cache: hit/miss/eviction/TTL/invalidation/concurrency via Testcontainers Valkey.
2. NATS: publish/consume/groups/DLQ/idempotency/reconnect/TLS via Testcontainers NATS.
**Acceptance Criteria:**
- [ ] All green with `-race -count=3`.
**Story Points:** 2
**Depends On:** E08-T01, E08-T03, E08-T04, E07-T09
**Related Docs:** `SPEC.md §10.3`
**SDD Gate:** G4

---

### E08-T07: Valkey token-bucket rate limiter (Lua + middleware)
**Status:** pending
**Background:** E11-T01 lists RateLimit in the middleware chain and E15-T04
verifies it, but no task built it. Token bucket per SPEC §9.4, atomic via Lua.
**Files:**
- Create: `internal/infrastructure/cache/valkey/{ratelimiter.go,ratelimit.lua}`
**Steps:**
1. Lua script: atomic check-and-decrement per key (`ratelimit:{dimension}:{id}`),
   dimensions ip/user/tenant/key/endpoint, per-tenant overrides from config.
2. The adapter implements the `RateLimiter` application port from E06-T12 and
   returns the remaining budget/reset time; it does not know HTTP envelopes or
   headers.
3. Emits `ratelimit.allowed/denied` metrics hooks for E15-T02.
**Acceptance Criteria:**
- [ ] Burst test: sustained over-limit traffic never exceeds budget by >5% (Testcontainers Valkey).
- [ ] Script is the only `redis.call` Lua in the repo besides this file (review guard).
**Story Points:** 3
**Depends On:** E08-T01
**Related Docs:** `SPEC.md §9.4`, `docs/api-contracts.md §2, §12`, `docs/fintech-ledger-features.md §10`
**SDD Gate:** G4

## Acceptance Criteria

- [ ] E08-T01 … E08-T07 all `completed` (count 24 SP in `tasks/tracking/PROGRESS.md`)
- [ ] Hit-rate benchmarks recorded (L1 <1ms, L2 <5ms targets)
- [ ] Every domain event routable (`check-tasks.py --subjects`); webhook signatures verify against docs
- [ ] SDD gate G4 checks pass — `tasks/tracking/GATES.md#G4`
