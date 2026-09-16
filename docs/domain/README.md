# Domain Layer

The Domain layer contains pure business logic with **zero external dependencies** (Go standard library only). It defines the fundamental rules, accounting principles, invariants, and entity lifecycles of the ledger system.

For a comprehensive explanation of how the Domain layer compares to the Application layer, see the [Clean Architecture & DDD Guide](../architecture/clean-architecture-guide.md).

## Directory Structure & Responsibilities

- **`aggregate/`**: Transactional consistency boundaries that protect business invariants. External callers mutate state only by invoking methods on Aggregate Roots (e.g. `AccountAggregate`, `PostingAggregate`).
- **`entity/`**: Data models that possess a distinct identity across time (e.g. `Account`, `Dispute`, `SettlementBatch`). Implements structural validation via `Validate() error`.
- **`valueobject/`**: Immutable, self-validating data structures without conceptual identity (e.g. `Money`, `Currency`, `PaymentStatus`, `FXRate`).
- **`service/`**: Pure mathematical calculations or multi-entity business logic that does not belong to a single entity (e.g. `AssessTransactionFee`, `CalculateInterest`, `ValidateRefund`). Pure functions, zero I/O.
- **`specification/`**: Executable business predicates and rules evaluated against candidate entities (e.g. `TransferRuleSpecification`).
- **`event/`**: Domain Events recording immutable business facts that occurred in the past (e.g. `TransferInitiated`, `PostingCommitted`).
- **`repository/`**: Narrow interfaces declaring collection-style persistence contracts for Aggregate Roots. (Implementations reside in `internal/infrastructure/database/`).

## Architectural Rules

1. **Zero External Dependencies**: Must only import the Go standard library. Never import database drivers (GORM, SQL), serialization codecs (JSON, YAML), protocol frameworks, or application packages.
2. **Zero I/O**: Domain entities and services never perform network requests, disk reads, or database operations.
3. **Invariants Enforced by Construction**: Invalid states must be rejected upon creation or transition.
