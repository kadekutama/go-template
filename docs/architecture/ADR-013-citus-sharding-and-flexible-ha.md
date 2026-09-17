# ADR-013: Distributed Persistence with Citus Sharding and Flexible HA (Patroni / CloudNativePG)

**Status:** Proposed  
**Date:** 2026-09-17  
**Note:** Proposed for repository owner review and acceptance  

## Context

As transaction volumes scale into thousands of operations per second across multiple tenants, a single PostgreSQL instance faces I/O and storage bounds. In fintech, transactional consistency and low write latency are paramount. 

We evaluated two divergent architectural approaches to database scalability:
1. **Distributed SQL Engines (e.g., YugabyteDB, CockroachDB)**:
   - *Pros*: Distributed consensus per tablet, automatic cross-node rebalancing, multi-region active-active capability.
   - *Cons*: Every single write transaction incurs a multi-node Raft roundtrip penalty (5–25 ms baseline commit latency vs. <0.5 ms in single-node PostgreSQL). Under high concurrency, platform settlement accounts or fee accounts suffer severe lock contention, leading to high abort rates and optimistic lock retry cascades.
2. **Multi-Tenant Sharded PostgreSQL (Citus 14.0)**:
   - *Pros*: In multi-tenant B2B fintech, 99.9% of operations are strictly scoped to a single merchant (`tenant_id`). Citus co-locates all tables for a given `tenant_id` onto the same physical PostgreSQL worker node. Consequently, intra-tenant ledger transactions execute as **native, single-node PostgreSQL ACID writes with sub-millisecond latency (<0.5 ms)**, with zero distributed locking overhead.
   - Cross-tenant aggregations and reference tables (e.g. currencies, fee schedules) are supported natively by Citus's distributed query planner.

Additionally, production infrastructure demands high availability (HA) with automated, split-brain-proof failover:
- On bare-metal, virtual machines, or local dockerized environments, **Patroni v4.1.5** backed by an **etcd v3.7.0** DCS is the battle-tested standard (used by Zalando, GitLab).
- On Kubernetes, **CloudNativePG (CNPG) v1.30.0** is the modern cloud-native standard, eliminating external Python daemons and managing failovers natively through Kubernetes Leases.

## Decision

1. **Horizontal Sharding via Citus 14.0**:
   - Adopt Citus 14.0 as the horizontal sharding engine for PostgreSQL.
   - Shard core ledger tables (`accounts`, `entries`, `postings`, `holds`, `outbox_facts`, `idempotency_keys`) by `tenant_id` using `create_distributed_table()`.
   - Configure reference tables (`currencies`, `settlement_rules`) using `create_reference_table()` so they are replicated to all worker shards for zero-network-hop local joins.
2. **Dual-Profile High Availability (Patroni & CloudNativePG)**:
   - Keep application code 100% decoupled from the HA supervisor by connecting through standard PostgreSQL connection URLs (`DATABASE_URL`).
   - Provide two ready-to-use infrastructure deployment profiles:
     - **Profile A (Bare-Metal / Docker Compose)**: Citus Coordinator and Worker shards orchestrated by **Patroni v4.1.5** with **etcd v3.7.0** for DCS leader election.
     - **Profile B (Kubernetes Native)**: Citus cluster managed via **CloudNativePG (CNPG) v1.30.0** custom resources utilizing native Kubernetes Leases.

## Real-World Scenarios Covered

- **Black Friday / Flash Sales**: A single high-volume merchant processing 10,000 TPS does not saturate other merchants because tenant co-location routes their transactions to an isolated, dedicated worker shard.
- **Hot Platform Settlement Account**: Because postings execute locally within the co-located shard, row locks (`SELECT FOR UPDATE`) resolve in microseconds without cross-datacenter Raft deadlocks.
- **Unplanned Node Hardware Failure**: Patroni/CNPG promotes the synchronous standby in under 10 seconds with **zero data loss (RPO=0)**.

## Consequences

- The application layer remains unchanged (`internal/domain/repository/` and `internal/application/port/` interfaces).
- Sharded tables require `tenant_id` in composite primary keys and unique constraints.
- Multi-node cluster manifests are maintained in `deployments/docker/` and `deployments/k8s/`.
