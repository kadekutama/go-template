# ADR-015: High-Efficiency L1 In-Memory Caching via Otter (Adaptive W-TinyLFU & Native Generics)

**Status:** Proposed  
**Date:** 2026-09-17  
**Note:** Proposed for repository owner review and acceptance (supersedes Ristretto reference in SPEC §7.3)  

## Context

In our Clean Architecture ledger, an L1 in-memory application cache sits inside each service pod to cache hot read models (display balances, tenant configuration, FX rates, and completed idempotency response hints) ahead of the Valkey L2 cache and PostgreSQL.

Earlier specifications referenced **Ristretto v2.4.2** (`dgraph-io/ristretto`). However, an audit of Ristretto's operational behavior in high-throughput financial environments revealed severe risks:
1. **Asynchronous / Lossy Write Buffer Hazard**: Ristretto’s `Set()` does not write synchronously to the internal map; it pushes items onto an internal buffered channel (`setBuf chan *item`) for processing by a background worker goroutine. Under bursty load, or when the buffer fills, writes are either delayed or silently dropped. Writing an idempotency key response followed immediately by an incoming duplicate read (`Set` -> `Get` in microseconds) can return a **false-negative cache miss**, triggering duplicate transaction evaluation or redundant DB pressure.
2. **Lack of Go Generics**: Ristretto requires boxing every value into `any` / `interface{}`. Runtime type assertions (`v.(*AccountDTO)`) risk production panics and generate garbage collector allocation overhead on the payment hot-path.
3. **Static Frequency Starvation**: Classic static TinyLFU resists admitting newly bursty items (e.g. flash sales on a new merchant) if older keys have accumulated high frequency counts.

Alternative libraries like **BigCache** avoid GC via off-heap byte slices, but BigCache **lacks per-key TTL** (all keys must share one global expiration window), making it impossible to support 1-minute balance TTLs and 1-hour FX rate TTLs in the same cache.

## Decision

1. **Adopt Otter v2.3.0 (`github.com/maypok86/otter`)**:
   - Replace Ristretto with Otter as the official L1 in-memory cache adapter implementing the application `Cache` port (`internal/application/port/cache.go`).
2. **Key Capabilities Leveraged**:
   - **Immediate Synchronous Visibility**: Writes are immediately visible to all concurrent goroutines upon `Set()` return, eliminating false-negative idempotency misses and dropped writes under write saturation.
   - **Adaptive W-TinyLFU Eviction Algorithm**: Based on the Window TinyLFU architecture (ACM paper 3274816, Caffeine architecture) with concurrent hash map and BP-Wrapper lock-free read buffers. Dynamically adapts window cache sizing between recency and frequency based on hit/miss feedback, admitting bursty items immediately while protecting long-term high-frequency entries.
   - **Native Go Generics**: Strongly typed `otter.Cache[K, V]` with compile-time type safety and zero interface allocation overhead.
   - **Per-Key TTL & Cost Bounding**: Supports granular TTLs per item (`1m` for balances, `1h` for FX rates, `24h` for idempotency hints) with strict memory cost limits to prevent container OOMs.

## Real-World Scenarios Covered

- **Idempotency Key Deduplication**: Concurrent payment retries arrive within 50 microseconds; the second request immediately reads the cached response committed by the first request without falling through to the database.
- **Merchant Flash Sales**: A new merchant launches a product drop, instantly receiving 5,000 requests/sec. Adaptive W-TinyLFU dynamically expands the window admission to absorb the bursty keys without TinyLFU frequency starvation.

## Consequences

- `SPEC.md §7.3`, `§7.10`, and `tasks/epics/E08-cache-messaging.md` are updated to specify Otter v2.3.0.
- Zero changes to the application `Cache` port interface.
