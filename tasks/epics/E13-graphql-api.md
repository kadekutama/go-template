# Epic E13: GraphQL API

**Status:** pending
**Story Points:** 18
**Phase:** 6 (parallel with E11, E12, E14)
**Dependencies:** E07.1, E08, E09, E10
**SDD Gate:** G5
**Design refs:** `SPEC.md §8.3`, `docs/api-contracts.md §9` (schema, subscriptions),
`docs/fintech-ledger-features.md §7.3, §8`

> Why a separate epic: flexible queries plus real-time subscriptions with their
> own N+1 and complexity hazards — resolvers, DataLoader, and WS lifecycle
> deserve focused review away from REST/gRPC.

## Tasks

### E13-T01: Schema with full input coverage + codegen
**Status:** pending
**Background:** Schema-first with gqlgen; inputs must include scheduled/batch
fields per the parity rule.
**Files:**
- Create: `api/graphql/schema.graphqls`, `gqlgen.yml`; Output: `pkg/graphql/{model,resolver}/`
**Steps:**
1. Types for all domain objects + `CreateTransferInput{executeAt, recurrence}`,
   `CreateBatchTransferInput{items}`, connection types for cursor pagination.
2. Scalars: DateTime, Decimal, Currency, JSON. Directives: `@auth`, `@rateLimit`.
3. `make generate` runs `gqlgen generate`; schema diff checked in CI.
**Acceptance Criteria:**
- [ ] `execute_at`/`recurrence`/batch inputs present (review vs api-contracts §7.5).
- [ ] Introspection matches committed schema file (test).
**Story Points:** 4
**Depends On:** E06-T06, E07.1-T01
**Related Docs:** `SPEC.md §8.3`, `SPEC.md §2` (gqlgen v0.17.94), `docs/api-contracts.md §9`
**SDD Gate:** G5

---

### E13-T02: Resolvers + DataLoader (N+1 safe)
**Status:** pending
**Background:** Resolvers delegate to E06; loaders batch account/txn/entry fetches.
**Files:**
- Create: `pkg/graphql/resolver/{account,transaction,transfer,payment,refund,payout,reconciliation,period,report}.go`,
  `pkg/graphql/dataloader/{loaders.go,keys.go}`
**Steps:**
1. Query/mutation resolvers map 1:1 to E06 handlers (no business logic).
2. DataLoaders: AccountByID, TransactionByID, EntriesByTransaction/Account with request-scoped caching.
3. Complexity limit 1000, depth limit 15.
**Acceptance Criteria:**
- [ ] N+1 test: nested account→transactions→entries issues bounded queries (counting test).
- [ ] Over-complexity query rejected (test).
**Story Points:** 5
**Depends On:** E13-T01, E06-T02, E06-T03, E06-T04, E06-T05
**Related Docs:** `SPEC.md §8.3`, `docs/api-contracts.md §9`
**SDD Gate:** G5

---

### E13-T03: Subscriptions over WebSocket (real-time)
**Status:** pending
**Background:** Features §8 + api-contracts subscriptions: balance changes,
transaction posted, payment succeeded, break found, period closed.
**Files:**
- Create: `pkg/graphql/resolver/subscriptions.go`, `pkg/graphql/ws/{handler.go,auth.go}`
**Steps:**
1. WS handler: connection init with auth token, tenant-scoped topic filter, 30s heartbeat.
2. Bridge E08 NATS subjects → subscription topics (`account.balance.changed` etc.).
3. Backpressure: slow consumers disconnected with metric, resumable via cursor.
**Acceptance Criteria:**
- [ ] Post a transaction → subscriber receives event <1s in integration test.
- [ ] Cross-tenant events never delivered (adversarial test).
**Story Points:** 4
**Depends On:** E13-T02, E08-T03
**Related Docs:** `docs/api-contracts.md §9` (subscriptions), `docs/fintech-ledger-features.md §7.3, §8`, `docs/domain-events.md §3`
**SDD Gate:** G5

---

### E13-T04: GraphQL server + integration tests (G5 slice)
**Status:** pending
**Background:** Server wiring + G5 evidence.
**Files:**
- Create: `pkg/graphql/server.go`, `cmd/graphql-api/main.go`, `test/integration/api/graphql/...`
**Steps:**
1. Server with E11-equivalent middleware behavior (recovery/logging/tracing/auth/limits/locale).
2. Playground/Voyager dev-only; disabled in prod config.
3. Integration: queries/mutations/subscriptions incl. WS lifecycle; schema-introspection match.
**Acceptance Criteria:**
- [ ] `go test ./test/integration/api/graphql/... -race -count=3` green.
**Story Points:** 3
**Depends On:** E13-T02, E13-T03
**Related Docs:** `SPEC.md §8.3`, `SPEC.md §10.3`
**SDD Gate:** G5

---

### E13-T05: Aggregation queries (sum/count/group-by, complexity-bounded)
**Status:** pending
**Background:** Features §7.3 requires aggregations; without an explicit task
they fall through the cracks between resolvers (E13-T02) and reports (E06-T09).
Fine-grained reconciliation-run progress stays polling-based (documented decision —
subscriptions cover state changes, E13-T03).
**Files:**
- Create: `pkg/graphql/resolver/aggregations.go`
**Steps:**
1. Aggregation fields (volume/fees/counts grouped by day/week/tenant) delegating to E06-T09 dashboard queries.
2. Same complexity/depth budget as E13-T02; aggregations costed higher per group key.
**Acceptance Criteria:**
- [ ] Aggregation over a large fixture returns bounded results (test).
- [ ] Over-budget aggregation rejected like other complex queries (test).
**Story Points:** 2
**Depends On:** E13-T02, E06-T09
**Related Docs:** `docs/fintech-ledger-features.md §7.3, §8`, `docs/api-contracts.md §9`
**SDD Gate:** G5

## Acceptance Criteria

- [ ] E13-T01 … E13-T05 all `completed` (count 18 SP in `tasks/tracking/PROGRESS.md`)
- [ ] Schema = REST parity; subscriptions live; N+1 bounded by test
- [ ] SDD gate G5 checks pass — `tasks/tracking/GATES.md#G5`
