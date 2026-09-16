# Application Layer

The Application layer orchestrates use cases. It glues the delivery interfaces (REST, gRPC, Cron) to the domain core without leaking protocol or infrastructure details into the business domain.

For a comprehensive explanation of how the Application layer compares to the Domain layer, see the [Clean Architecture & DDD Guide](../architecture/clean-architecture-guide.md).

## Directory Structure & Responsibilities

- **`command/`**: State-mutating use cases following CQRS write principles (e.g. `CreateTransfer`, `ExecuteTransfer`, `ProvisionTenant`, `CreateIntent`).
  - **Standard Command Pipeline**: Envelope Validation $\rightarrow$ Authorization (`Require`) $\rightarrow$ Idempotency Check (`Reserve`) $\rightarrow$ Domain Aggregate Loading $\rightarrow$ Domain Invariant Execution $\rightarrow$ Atomic Persistence & Outbox Event Staging via Unit of Work (`port.Tx`) $\rightarrow$ Complete Idempotency.
  - Contains co-located command execution helpers: `idempotency.go` (`Fingerprint`, `RunIdempotent`) and `authz.go` (`Require`).
- **`query/`**: Read-only use cases following CQRS read principles (e.g. `GetBalance`, `ListTransfers`).
  - Strictly side-effect free: never mutates state and never publishes outbox events.
  - Handles cursor pagination (`clampPageLimit`), filtering, and projections.
  - Contains presentation translation helpers: `translation.go` (`ToAppError`, `Localize`).
- **`port/`**: Interfaces declaring external requirements (Inversion of Control):
  - Infrastructure ports: `IdempotencyStore`, `Authorizer`, `UnitOfWork` (`port.Tx`), `OutboxStore`, `Clock`, `IDGenerator`.
  - Gateway ports: `PaymentProcessorGateway`, `BankSettlementGateway`.
  - Segregated CQRS Use Case interfaces: `AccountCommandUseCases`, `AccountQueryUseCases`, etc.
- **`dto/`**: Data Transfer Objects (Requests & Responses) crossing the delivery $\leftrightarrow$ application boundary. Completely decoupled from domain entities and database tables.
- **`service/`**: Application-level helpers and policy evaluators (e.g. payout eligibility checks).
- **`workflow/`**: Long-running sagas and multi-step processes (e.g. `BatchTransferWorkflow`, `ReconciliationSaga`).

## Architectural Rules

1. **Inward Dependencies**: May only import `internal/domain/`, own `port/` and `dto/`, and kernel utilities (`pkg/jsonparser`, `internal/shared/kernel`). Never import `internal/infrastructure/` or `internal/interface/`.
2. **Protocol Independence**: Application services must have zero knowledge of HTTP headers, gRPC metadata, URL query parameters, or JSON tags.
3. **CQRS Segregation**: Queries must never mutate state. Commands and Queries declare separate port interfaces.
