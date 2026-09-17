# ADR-017: Distributed Consensus, Dynamic Configuration & Leader Election via etcd

**Status:** Proposed  
**Date:** 2026-09-17  
**Note:** Proposed for repository owner review and acceptance  

## Context

Distributed systems require a strongly consistent coordination plane to handle three critical operational concerns:
1. **High Availability Database Failover**: Managing leader election and split-brain fencing for PostgreSQL clusters (as required by Patroni).
2. **Dynamic Configuration & Feature Flags**: Pushing runtime updates (e.g., fee schedules, tenant rate-limit overrides, maintenance modes) to thousands of Go application pods in real time without restarting pods.
3. **Single-Writer Worker Coordination**: Guaranteeing that scheduled financial jobs (e.g. daily reconciliation, interest accrual, period-closing sagas) execute on exactly one worker pod at a time.

We evaluated two leading coordination platforms:
- **Consul (HashiCorp)**: Feature-rich with service mesh and DNS, but uses HTTP long-polling for watchers and suffers from BSL 1.1 licensing restrictions. In Patroni deployments, Consul's session-based locks have historically triggered false-positive database failovers under transient network jitter.
- **etcd (CNCF Graduated)**: Pure, rock-solid distributed key-value store based on the Raft consensus protocol. It is the core consensus engine of Kubernetes, 100% open-source under Apache 2.0, and natively supported by Patroni's maintainers (Zalando).

## Decision

1. **Deploy etcd v3.7.0 as the Platform Coordination Plane**:
   - Deploy a 3-node etcd cluster (`etcd-1`, `etcd-2`, `etcd-3`) using Raft consensus.
2. **Patroni High Availability Integration**:
   - Standardize on etcd as the default Distributed Configuration Store (DCS) for all Patroni-managed PostgreSQL clusters.
3. **HTTP/2 gRPC Streaming Watchers (`clientv3.Watcher`)**:
   - Application pods maintain a single persistent gRPC stream watching configuration prefixes (e.g., `/config/tenants/`).
   - Configuration changes and feature flag updates are pushed to watching pods in **under 1 millisecond** with zero polling CPU overhead.
4. **Single-Writer Leader Election for Financial Sagas**:
   - Background worker processes (Epic E14) utilize etcd's native Go concurrency package (`go.etcd.io/etcd/client/v3/concurrency`) to campaign for leadership before executing financial reconciliation or ledger period closes:
     ```go
     session, _ := concurrency.NewSession(client, concurrency.WithTTL(5))
     election := concurrency.NewElection(session, "/finance/reconciliation-leader")
     if err := election.Campaign(ctx, podID); err == nil {
         // Safely execute single-writer reconciliation
     }
     ```

## Real-World Scenarios Covered

- **OOM Kill on Active Reconciliation Pod**: The active worker pod running the daily multi-million-dollar ledger reconciliation suffers a memory spike and terminates. Within 5 seconds, its etcd lease expires, and an idle standby worker pod is elected leader to resume the reconciliation saga.
- **Emergency Payment Rail Circuit Breaker**: An upstream payment processor begins returning 500 errors. An operator updates the processor status in etcd; every API pod disables the rail within 500 microseconds without requiring rolling pod restarts.

## Consequences

- etcd v3.7.0 client (`go.etcd.io/etcd/client/v3`) is added as an infrastructure dependency.
- Docker Compose and Kubernetes manifests include a 3-node etcd cluster with automated snapshots.
