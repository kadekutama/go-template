# ADR-014: Dual-Broker Messaging Architecture: Redpanda Event Backbone + NATS Core Real-Time Edge Mesh

**Status:** Proposed  
**Date:** 2026-09-17  
**Note:** Proposed for repository owner review and acceptance  

## Context

A production fintech system handles two fundamentally different categories of asynchronous messaging:
1. **Durable, Ordered, Replayable System-of-Record Events**:
   - Outbox facts, audit logs, financial ledger postings, and webhook deliveries must never be lost. They require strict partition-level ordering, consumer group offset commits, long-term disk retention (e.g. 30 days), and horizontal partition scaling.
2. **Ephemeral, Low-Latency Edge Broadcasts**:
   - Live balance updates pushing to frontend WebSockets or merchant dashboards require sub-millisecond fanout without writing duplicates to disk or paying disk fsync penalties.

Running a single broker creates an unavoidable conflict:
- If NATS JetStream is configured with file storage for durability, high-throughput consumer group fanout and multi-tenant scaling require complex stream partition management.
- If Apache Kafka is used directly for WebSocket fanout, thousands of ephemeral browser connections will exhaust Kafka connection limits and broker memory.

## Decision

We adopt a **Dual-Broker Messaging Architecture**:
1. **Durable Backbone: Redpanda v26.2 (Kafka API)**:
   - Deploy a 3-node Redpanda cluster operating via native Raft consensus (C++ Seastar thread-per-core engine, eliminating JVM garbage collection pauses).
   - All atomically committed outbox facts from PostgreSQL are published to partitioned Redpanda topics (`ledger.events.v1`, `outbox.facts.v1`, `webhook.jobs.v1`, `audit.streams.v1`).
   - Strict partition keying on `tenant_id:account_id` ensures deterministic event ordering per ledger account.
2. **Real-Time Edge Mesh: NATS Core v2.14.6 (In-Memory)**:
   - Deploy a 3-node NATS Core cluster in pure in-memory mode (JetStream disk persistence is explicitly disabled to prevent redundant double-disk writes).
   - A dedicated notification worker consumes events from Redpanda and broadcasts lightweight delta updates across NATS subject hierarchies (`ledger.{tenant_id}.account.balance.changed.v1` via E05-T03 builders).
   - Web / mobile WebSocket termination gateways subscribe to NATS subjects to deliver live UI updates in <5 microseconds.

## Real-World Scenarios Covered

- **High-Volume Merchant Webhooks**: If a merchant's endpoint goes down for 3 hours, Redpanda consumer groups hold the backlog with exponential backoff and dead-letter queues (DLQ) without losing a single webhook event.
- **Regulatory Audit Event Replay**: Regulators require replaying all transactions from 18 months ago to verify balance materialization; Redpanda tiered storage streams historical segments directly from S3.
- **Flaky Mobile Connections**: 50,000 retail users walking through subway tunnels constantly disconnect and reconnect; NATS Core manages subscriptions in RAM with zero cluster rebalancing.

## Consequences

- Go application code remains protected behind `port.EventPublisher` and `port.NotificationPublisher`.
- Operational footprint requires running Redpanda and NATS Core in production (standardized via Docker Compose and Helm).
