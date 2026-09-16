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

## How to add an ADR

1. Copy the previous ADR number + 1 as `docs/architecture/ADR-<NNN>-<slug>.md`.
2. Fill Context → Decision → Consequences; link the owning task packet.
3. Architectural deviations require the ADR BEFORE implementation
   (`tasks/SDD.md §3.3`); never let code be the only record of a decision.
4. Add one row to the table above and link it from the affected task.
