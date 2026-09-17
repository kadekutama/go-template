# Vertical Delivery Slices

**Status:** Normative scheduling guidance  
**Task source of truth:** `tasks/epics/*.md` task-level dependencies

The epic phase diagram is useful for ownership and reporting, but building every
domain feature, then every application feature, then every adapter is too risky
for a financial system and produces poor AI handoffs. Schedule the following
vertical slices through the task DAG. A later slice may be specified in parallel,
but cannot consume an earlier capability before its promotion checks pass.

The task-level `Depends On` DAG remains authoritative when it differs from a
slice or epic phase. Slices recommend promotion order and cannot bypass a task
dependency or its SDD gate.

## S0 — Repository and SDD Control Plane

**Tasks:** E00-T00–T03, E00-T05, E00-T08, then the E01 kernel tasks required by S1.

**Exit:** A fresh harness can find a runnable task, validate an approved packet,
claim it, run baseline checks, record evidence, and hand it to another harness.
CI rejects task cycles and invalid lifecycle state.

## S1 — Ledger Kernel Vertical Proof

**Domain:** E02-T01–T05, E02-T07–T08  
**Application:** E06-T06, E06-T01, E06-T13  
**Persistence/test:** E07-T09, E07-T01, E07-T03  
**Local interface:** E11-T01, E11-T02, E11-T14

E07-T01 must not select a production balance materialization strategy until
ADR-011's benchmark and correctness evidence is accepted.

**Exit:** One restricted capture-style posting can be written, replayed
idempotently, read back, and reflected in a strongly consistent balance through
real PostgreSQL. Tests prove per-asset balance, immutability, authorization hook,
deterministic concurrent-spend behavior, outbox atomicity, and operation without
Valkey/NATS/provider credentials. This interface is local/test-only until S2.

## S2 — Tenant Isolation, Distributed Persistence, and Public Edge

**Focus:** E05 tenancy/isolation, E07.1 Citus sharding and Patroni/CNPG HA, E06-T12,
E07-T10/T02, core E08 Otter L1 + Valkey L2 cache policy and Redpanda/NATS messaging,
E09 OpenBao secrets/Transit encryption/authentication/authorization/API keys,
E10-T01 feature flags, and E11-T15.

**Exit:** RLS and application authorization fail closed across tenants; Citus
distributed tables partition cleanly by `tenant_id`; secrets and dynamic DB
credentials are managed by OpenBao; public middleware and transport controls pass.
Only then may S1 endpoints be exposed outside the trusted test environment.

## S3 — Capture, Settlement, Refund, and Payout

**Focus:** The matching E03 domain tasks (including payout eligibility,
reserves, and negative-balance recovery), E06 workflow handlers/sagas, payment
processor fake, persistence models, REST contract, worker, and reconciliation
fixtures for each flow. Deliver one operation template end-to-end at a time:

1. capture and provider settlement;
2. refund including unknown provider outcomes;
3. payout submission, settlement, failure, and return;
4. internal transfer and scheduled execution.

**Exit:** Each flow has immutable journals, provider idempotency/status lookup,
outbox/inbox replay proof, and external-report reconciliation. Do not treat all
of E03 as one integration event.

## S4 — Reconciliation and Financial Operations

**Focus:** E04, statement parsers, period controls, approvals, reports, audit
checkpoints, backup/restore, and operational worker jobs.

**Exit:** Immutable source snapshots, many-to-many match groups, breaks,
maker-checker adjustments, period close/reopen, and restore recomputation are
proven against the S3 postings.

## S5 — Extended Product and Scale

**Focus:** FX after ADR-007, disputes, platform splits, reserves, top-ups,
GraphQL/gRPC parity, replicas/residency, performance, chaos, delivery, and release
hardening. P2 white-label/search/reporting features must not delay S1–S4.

**Exit:** Gates G5–G8 pass with workload/topology evidence. Availability,
performance, recovery, and compliance remain targets until their evidence files
and accountable approvals exist.

## Parallelization Guidance

- Spec authors may work ahead on different packets; implementation starts only
  when their inputs are stable.
- Domain rules, schema contracts, and API schemas can run in parallel after their
  shared IDs/types are merged.
- One owner reserves migration numbers, generated contracts, and shared registry
  files; other tasks consume them.
- Prefer completing a usable slice over maximizing the number of simultaneously
  open tasks.
