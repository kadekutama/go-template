# ADR-011: Balance materialization and authority

**Status:** Proposed — owner approval required before E07 implementation
**Date:** 2026-09-13

## Context

The ledger must answer balance and spend-authorization queries at the target
throughput and latency without weakening accounting correctness. Immutable
entries are the financial facts, but several materialization strategies are
possible:

- compute balances from entries at read time;
- maintain immutable, cursor-addressed checkpoints;
- maintain a derived mutable balance table refreshed in the posting transaction;
- combine checkpoints with a derived cache or replica projection.

The planning documents contain performance targets, but no workload model or
measurement yet proves which strategy meets them. A cache or replica also has a
different consistency contract from the primary ledger.

## Decision

No performance-driven materialization choice is accepted yet. The invariant is
fixed: committed `Posting`/`Entry` rows are the authoritative accounting facts;
any checkpoint, materialized balance, cache, or replica is derived and must be
rebuildable from those facts. A derived balance must never authorize spending
without the primary transaction's lock/cursor contract.

E07 must select the materialization strategy only after the benchmark and
correctness evidence below is reviewed and this ADR is changed to **Accepted**.
Until then, implementation tasks may use an immutable checkpoint as a
correctness reference, but may not make a mutable balance column or cache the
source of truth.

## Required evidence before acceptance

The benchmark must use the shared Testcontainers harness and record:

1. workload shape (posting mix, account fan-out, tenant distribution, read/write
   ratio, concurrency, hardware, PostgreSQL durability settings);
2. p50/p95/p99 write and balance-read latency and sustained throughput;
3. write amplification, lock contention, storage growth, checkpoint frequency,
   replica/cache lag, and read-after-write behavior;
4. serialization/deadlock, crash-before/after-commit, replay, and rebuild tests;
5. equivalence checks proving every derived result recomputes from immutable
   entries at a declared ledger cursor.

The report must compare at least on-demand aggregation, immutable checkpoints,
and a derived mutable projection. It must state the consistency and operational
trade-offs, not only the fastest benchmark result.

## Consequences

- E07-T01 cannot be marked implementation-ready based only on the 10K TPS or
  p99 targets.
- Read models can evolve without changing the accounting fact model.
- A chosen derived projection needs invalidation/rebuild, lag metrics, and a
  documented read-after-write path.
- The benchmark and approval become durable evidence for future agents rather
  than an undocumented performance assumption.

## Alternatives considered

- **Mutable account balances as authority:** rejected; lost updates, ad-hoc SQL,
  and cache failures could create or destroy spendable money.
- **Entries only forever:** not rejected, but its latency/cost must be measured
  against the declared workload before being treated as the production choice.
- **Cache-first authorization:** rejected; caches are projections and cannot
  provide uniqueness, locking, or overdraft safety.
