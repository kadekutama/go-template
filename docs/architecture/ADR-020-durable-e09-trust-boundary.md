# ADR-020: Durable E09 Trust Boundary — No App-Local Security State

**Status:** Accepted
**Date:** 2026-09-25 (supersedes 2026-09-23 proposal)
**Note:** Approved and verified. Replaces in-memory adapter stores, eliminates
advisory transaction locks in favor of Citus-distributed row locks, and standardizes
on tenant transaction runners and RFC-compliant cryptography.

## Context

The initial E09 implementation kept security-critical mutable state in
process-guarded Go maps: refresh-token families (`jwt/refresh.go`), API-key
records (`apikey/manager.go`), OAuth sessions (`oauth2/flow.go`), the audit
hash chain (`audit/logger.go`), and secret values (`secrets/openbao/cache.go`),
each serialized by `sync.Mutex`/`sync.RWMutex`.

Architectural reviews rejected this design on four grounds:
1. **Multi-pod divergence:** A process lock serializes goroutines within a single
   process, not cluster replicas. Two pods hold divergent maps: a revoked API key
   stays valid on the other pod, a spent refresh token spends twice, audit `seq`
   collides.
2. **State loss:** A pod restart wipes revocations, single-use token markers, and
   audit tails, causing silent loss of security and compliance state.
3. **Distributed locking hazards:** Using `pg_advisory_xact_lock(hashtext(?))` hashes
   tenant identifiers to 32-bit signed integers, suffering catastrophic hash collision
   risk in multi-tenant workloads. Furthermore, PostgreSQL advisory locks are local to the
   Citus coordinator and are not distributed across Citus worker nodes.
4. **Out-of-band complexity:** Replacing process locks with complex atomic CAS loops
   across non-E09 modules violates epic ownership boundaries and Specification-Driven
   Delivery (SDD) protocols.

## Decision

1. **No authoritative mutable state in application memory.** Single-use
   receipts (refresh tokens), key records (API keys), and audit facts live in
   PostgreSQL (Goose migrations, RLS-enforced, Citus-distributed); OAuth handshakes
   live in Valkey with TTL via `KVStore` backed by `*valkey.ValkeyClient`.
   Restarting or scaling the app loses nothing.
2. **PostgreSQL native row locks replace advisory locks for audit logging.**
   - Per-tenant audit serialization is achieved via row-level locks on the dedicated
     `audit_heads` table (`SELECT last_seq, last_hash FROM audit_heads WHERE tenant_id = ? FOR UPDATE`).
   - `audit_heads` is co-located and sharded by `tenant_id` on Citus, providing full
     distributed concurrency safety across all Citus worker nodes without coordinator bottlenecks.
   - Eliminates 32-bit hash collisions from `hashtext` advisory locks and avoids costly
     table scans on `audit_entries`.
3. **Durable Transaction Runner (`postgres.WithinTenantTx`).**
   - E09 infrastructure adapters use `postgres.WithinTenantTx` (and `postgres.WithinTx`)
     from `internal/infrastructure/database/postgres/tx_runner.go`.
   - Enforces PostgreSQL session-level tenant isolation (`SET LOCAL app.current_tenant_id = ?`)
     and guarantees atomic commit/rollback.
   - Used across `jwt/refresh.go`, `apikey/manager.go`, and `audit/logger.go`. Read-only
     methods avoid starting unnecessary database transactions.
4. **True Merkle tree root for audit log verification.**
   - Checkpoint calculation in `internal/infrastructure/audit/chain.go` calculates a true
     pairwise binary Merkle tree root over leaf hashes compliant with RFC 6962.
   - Replaces naive sequential hash loops with cryptographically verifiable tamper-evident roots.
5. **256-Bit cryptographic entropy for API keys.**
   - `newSecret()` in `internal/infrastructure/auth/apikey/manager.go` generates 32 bytes
     (256 bits) of cryptographically secure random entropy via `crypto/rand`.
   - Hashing uses Argon2id (`t=3, m=64MiB, p=4`) with separate salt and hash BYTEA storage.
6. **Durable RBAC policy loading.**
   - `internal/infrastructure/auth/rbac/db_loader.go` implements `DBPolicyLoader` with
     `CasbinRuleModel` (`casbin_rules` table), enabling database-backed dynamic policy
     reloading alongside the bootstrap `CSVLoader`.
   - `Enforcer.ReloadFrom(loader PolicyLoader)` enables hot reloading of policies and
     groupings without process restarts.
7. **Complete refresh token rotation chaining.**
   - `RefreshClaim` includes `NextToken string`, returning the newly issued token to the
     caller upon successful rotation, ensuring clients can chain token refreshes deterministically.
8. **HTTP provider testing with httpmock.**
   - Outbound HTTP providers (OAuth2, OpenBao, Transit KEK) use `jarcoal/httpmock` in unit
     tests, isolating transport responses and executing in-memory without binding host network ports.

## Consequences

- Database migrations `20260923000006..09` define:
  - `refresh_families` and `refresh_tokens` (with foreign keys and single-use markers)
  - `api_keys` (with Argon2id hash/salt and prefix indexing)
  - `audit_entries` and `audit_heads` (with RLS policies and Citus distribution)
  - Atlas checksum (`atlas.sum`) updated and validated.
- All out-of-band modifications made to non-E09 packages (`apperror`, `webhook`, `consumer`,
  `etcd`) are reverted to their stable base commit state.
- Statement test coverage meets all promotion gate thresholds (>= 90% for security-critical paths),
  with zero data races verified by `go test -v -race`.
