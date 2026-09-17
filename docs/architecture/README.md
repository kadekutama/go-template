# Architecture Decision Records

Index of structural decisions for this template. ADRs are immutable once
accepted; a superseded ADR stays listed with status `Superseded` and a link to
its replacement. Content beyond this index is filled by E18.

For architectural patterns, layers, and DDD structure, see the [Clean Architecture & DDD Guide](./clean-architecture-guide.md).

| ID | Title | Status |
|----|-------|--------|
| [ADR-002](./ADR-002-amount-currency-representation.md) | Amount and currency representation | Accepted |
| [ADR-003](./ADR-003-accounting-fact-immutability.md) | Accounting fact immutability | Accepted |
| [ADR-004](./ADR-004-tenant-isolation.md) | Tenant isolation (shared DB + RLS) | Accepted |
| [ADR-009](./ADR-009-product-boundary.md) | Product boundary | Accepted |
| [ADR-010](./ADR-010-structured-logging.md) | Structured logging API | Superseded by ADR-012 (port stands; default backend → zerolog) |
| [ADR-011](./ADR-011-balance-materialization.md) | Balance materialization and authority | Proposed |
| [ADR-012](./ADR-012-owner-directed-e01-defaults.md) | Owner-directed E01 defaults (Sonic, UUIDv7, zerolog) | Accepted |
| [ADR-013](./ADR-013-citus-sharding-and-flexible-ha.md) | Distributed persistence with Citus sharding and flexible HA | Proposed |
| [ADR-014](./ADR-014-dual-broker-messaging-redpanda-nats.md) | Dual-broker messaging architecture (Redpanda + NATS Core) | Proposed |
| [ADR-015](./ADR-015-l1-cache-otter.md) | High-efficiency L1 in-memory caching via Otter (Adaptive W-TinyLFU) | Proposed |
| [ADR-016](./ADR-016-secrets-management-openbao.md) | Enterprise secrets management & tokenization via OpenBao | Proposed |
| [ADR-017](./ADR-017-distributed-coordination-etcd.md) | Distributed consensus, dynamic config & leader election via etcd | Proposed |
| [ADR-018](./ADR-018-database-migrations-goose-and-atlas.md) | Hybrid database migrations via Goose v3, Atlas CI linter & UTC timestamps | Proposed |

## How to add an ADR

1. Copy the previous ADR number + 1 as `docs/architecture/ADR-<NNN>-<slug>.md`.
2. Fill Context → Decision → Consequences; link the owning task packet.
3. Architectural deviations require the ADR BEFORE implementation
   (`tasks/SDD.md §3.3`); never let code be the only record of a decision.
4. Add one row to the table above and link it from the affected task.
