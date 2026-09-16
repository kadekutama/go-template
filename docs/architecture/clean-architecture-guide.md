# Clean Architecture & Domain-Driven Design (DDD) Guide

This document is the normative architectural reference for engineers and AI agents working on this repository. It explains the design philosophy, the separation of concerns between `domain` and `application`, a granular walkthrough of all subdirectories, and a comparison with traditional layered Go architectures.

---

## 1. The Core Problem: Why Traditional 3-Tier Layering Fails

In traditional Go projects, code is commonly organized into flat layers under `internal/`:
- `entity/`: Struct definitions and database tags.
- `repository/`: Direct database queries, cache calls, and third-party API clients.
- `service/`: A mix of business calculations, repository calls, transaction handling, error mapping, and logging.
- `usecase/` (or `delivery/`): An orchestrator calling multiple services and handling HTTP/gRPC.

### The Pitfalls of Flat Layering:
1. **Anemic Domain Model**: Entities become passive "data holders" with public fields and no behavior. Any piece of code can mutate any field into an invalid state at any time.
2. **"God Services"**: The `service` layer balloons into thousands of lines of procedural code. Business rules (e.g., balance arithmetic, fee formulas) are tightly tangled with database transactions, Redis cache keys, and HTTP headers.
3. **Mocking Hell in Unit Tests**: Testing a simple business rule (such as calculating a fee or verifying an account transition) requires mocking database connections, redis caches, third-party clients, and context loggers. Tests become brittle and slow.
4. **Leaky Infrastructure Abstractions**: Database concerns (SQL queries, GORM hooks, JSON tags) leak upward into the business logic.

---

## 2. The Core Philosophy: Clean Architecture & DDD

Clean Architecture and Domain-Driven Design (DDD) enforce one non-negotiable rule:

> **The Dependency Rule**: Source code dependencies must point strictly inward. Inner layers know nothing about outer layers.

```
       +-------------------------------------------------------------+
       | Delivery / Interface (HTTP, gRPC, CLI, Kafka, Cron)         |
       |  +-------------------------------------------------------+  |
       |  | Infrastructure (Postgres, Valkey, External APIs)      |  |
       |  |  +-------------------------------------------------+  |  |
       |  |  | Application (Commands, Queries, Ports, DTOs)    |  |  |
       |  |  |  +-------------------------------------------+  |  |  |
       |  |  |  | Domain (Entities, Value Objects,          |  |  |  |
       |  |  |  |         Aggregates, Domain Services)      |  |  |  |
       |  |  |  +-------------------------------------------+  |  |  |
       |  |  +-------------------------------------------------+  |  |
       |  +-------------------------------------------------------+  |
       +-------------------------------------------------------------+
```

### The "Pen & Paper" Test
When deciding whether a piece of logic belongs in **Domain** or **Application**, ask:
> *"If computers did not exist and this business ran using pen, paper, physical ledgers, and filing cabinets, would this rule still exist?"*

- **"Debits must equal credits in every posting"**: **YES** $\rightarrow$ **`domain`** (Double-entry bookkeeping rule since 1494).
- **"Asset accounts cannot have a negative balance without overdraft approval"**: **YES** $\rightarrow$ **`domain`**.
- **"Transaction fee is 2.9% + 30 cents, capped at \$10"**: **YES** $\rightarrow$ **`domain`**.
- **"Check JWT token to see if user has permission `transfer.create`"**: **NO** $\rightarrow$ **`application`** (Security orchestration).
- **"Check Redis to ensure this request hasn't run in the last 10 seconds"**: **NO** $\rightarrow$ **`application`** (Distributed systems idempotency).
- **"Store this in PostgreSQL using a transaction and commit"**: **NO** $\rightarrow$ **`application` / `infrastructure`** (Storage mechanism).

---

## 3. Directory Structure & Granular Breakdown

```
internal/
├── domain/                  # PURE BUSINESS RULES (Go stdlib only, zero I/O)
│   ├── aggregate/           # Transactional consistency boundaries protecting invariants
│   ├── entity/              # Objects with mutable state and a distinct identity
│   ├── valueobject/         # Immutable, self-validating data structures without identity
│   ├── service/             # Pure calculation/validation logic across multiple entities
│   ├── specification/       # Executable business rules (predicates)
│   ├── event/               # Records of facts that have already occurred
│   └── repository/          # Persistence contracts for aggregates (interfaces only)
│
├── application/             # USE CASE ORCHESTRATION (glues delivery to domain)
│   ├── command/             # State-mutating use cases (CQRS Writes)
│   ├── query/               # Read-only use cases (CQRS Reads & Projections)
│   ├── port/                # External dependencies required by use cases (interfaces)
│   ├── dto/                 # Request and response structures crossing the boundary
│   ├── service/             # Application-level orchestration helpers
│   └── workflow/            # Multi-step sagas and long-running processes
│
├── infrastructure/          # CONCRETE IMPLEMENTATIONS OF APPLICATION & DOMAIN PORTS
│   ├── database/            # PostgreSQL, GORM, migrations
│   ├── cache/               # Valkey, Ristretto (L1/L2)
│   └── resilience/          # Outbox dispatchers, circuit breakers, rate limiters
│
└── interface/ (or api/)     # PROTOCOL ADAPTERS & DELIVERY
    ├── rest/                # Echo HTTP handlers, middleware, route registration
    ├── grpc/                # gRPC services and Protobuf adapters
    └── cron/                # Scheduled background job triggers
```

---

## 4. `internal/domain/` — Granular Subdirectory Guide

**Golden Rule for Domain**: Zero imports outside the Go standard library (`math`, `strings`, `time`, etc.). Never import database drivers, JSON serialization libraries, web frameworks, or application packages.

### 4.1. `valueobject/` (Value Objects)
- **Definition**: Data structures that represent a descriptive aspect of the domain with **no conceptual identity**. Two Value Objects with identical attributes are equal. They are **immutable** and **self-validating**.
- **Characteristics**:
  - Constructors validate invariants upon creation (e.g., `NewMoney(100, "USD")`).
  - Cannot be changed; operations return a new instance (e.g., `money.Add(other)`).
  - Encapsulate valid state transitions (e.g., `CanTransitionPayment(from, to)`).
- **Examples in repo**:
  - `Money`: Combines minor units (`int64`) and `CurrencyCode`. Prevents floating-point rounding errors.
  - `PaymentStatus`: Enforces valid state transitions (`PENDING -> AUTHORIZED -> CAPTURED`).
  - `FXRate`: Encapsulates currency exchange math with decimal precision.

### 4.2. `entity/` (Entities)
- **Definition**: Objects that have an **explicit identity** (e.g., an ID) that runs through a lifecycle. Two entities with the same ID represent the same object even if their attributes differ.
- **Characteristics**:
  - Contains an ID and timestamps.
  - Has a `Validate() error` method to ensure its internal data structure is valid.
  - Represents domain data, NOT database rows (no database or JSON tags).
- **Examples in repo**:
  - `Account`: Has `AccountID`, currency, account type, and balance.
  - `Dispute`: Has `DisputeID`, reason, evidence deadline, and status.
  - `SettlementBatch`: Has `BatchID`, net amount, and item list.

### 4.3. `aggregate/` (Aggregates)
- **Definition**: A cluster of associated entities and value objects treated as a single unit for data changes. Every aggregate has an **Aggregate Root**. Outer layers can only reference the Root.
- **Characteristics**:
  - **Enforces transactional business invariants**. External code cannot modify internal entities directly; all changes must pass through methods on the Aggregate Root.
  - Emits **Domain Events** when state changes.
- **Examples in repo**:
  - `AccountAggregate`: Protects the account balance invariant. You cannot arbitrarily set `Balance = 50`. You call `ApplyPosting(entry)` which verifies overdraft limits, account types, and currency match.
  - `PostingAggregate`: Protects the double-entry invariant. Ensures `sum(debits) == sum(credits)`.

### 4.4. `service/` (Domain Services)
- **Definition**: Pure business logic or mathematical calculations that naturally involve multiple entities or aggregates and do not belong inside a single entity.
- **Characteristics**:
  - **Stateless & Pure**: Given inputs $A$ and $B$, always returns $C$.
  - Zero I/O: Does not fetch from databases or call APIs.
- **Examples in repo**:
  - `AssessTransactionFee(amount, bps, floor, cap)`: Computes percentage fees, enforces boundaries, and checks for integer overflow.
  - `ValidateRefund(intent, refundAmount, existingRefunds)`: Checks if a refund exceeds the captured amount or falls outside the allowed refund window.

### 4.5. `specification/` (Specifications)
- **Definition**: Encapsulates a reusable business predicate (rule) into an executable object.
- **Characteristics**:
  - Answers: *"Does this candidate satisfy this business rule?"* (`IsSatisfiedBy(candidate) bool` or `error`).
- **Examples in repo**:
  - `TransferRuleSpecification`: Evaluates velocity limits, sanctions list flags, and maximum single-transfer limits.

### 4.6. `event/` (Domain Events)
- **Definition**: Immutable records of a significant event that **already happened** in the domain.
- **Characteristics**:
  - Named in the past tense (e.g., `PaymentAuthorized`, `AccountFrozen`).
  - Contains facts (ID, amount, timestamp), not side-effect instructions.

### 4.7. `repository/` (Domain Repository Interfaces)
- **Definition**: Pure Go interfaces defining the collection-like boundary for persisting and retrieving aggregate roots.
- **Characteristics**:
  - Defined in the domain; implemented in infrastructure.
  - Expressed in domain terms (e.g., `Save(ctx, aggregate)`), not SQL terms (e.g., `UpsertWhereQuery`).

---

## 5. `internal/application/` — Granular Subdirectory Guide

**Golden Rule for Application**: The application layer orchestrates use cases. It coordinates domain logic, permissions, idempotency, and database transactions, but does not own business formulas or protocol details (HTTP/JSON).

### 5.1. `command/` (Commands / Write Use Cases)
- **Definition**: Use cases that mutate state. Follows Command Query Responsibility Segregation (CQRS).
- **Execution Pipeline**:
  Every command handler strictly follows this orchestration pipeline:
  1. **Envelope Validation**: Validate incoming DTO parameters (e.g., non-empty IDs).
  2. **Authorization**: Call `authz.Require(ctx, authorizer, subject, action, resource)`. Fail closed.
  3. **Idempotency Reservation**: Reserve the key via `idempotency.RunIdempotent` or `tx.Idempotency().Reserve()`. If replayed, return cached result.
  4. **Domain Loading**: Fetch aggregate root from repository within a transaction.
  5. **Domain Execution**: Invoke domain aggregate or service to perform the business state change.
  6. **Persistence & Outbox**: Atomically commit the modified aggregate and stage domain event facts in the Outbox table within one Unit of Work (`port.Tx`).
- **Examples in repo**:
  - `TransferService.CreateTransfer`: Executes immediate or scheduled money transfers.
  - `PaymentService.CreateIntent` / `ConfirmIntent`: Manages credit card authorization lifecycles.
  - `TenantService.ProvisionTenant`: Sets up new tenant accounts and default ledgers.

### 5.2. `query/` (Queries / Read Use Cases)
- **Definition**: Read-only operations in CQRS.
- **Characteristics**:
  - Strictly side-effect free: never mutates data and never publishes outbox events.
  - Can read directly from read-optimized stores, replicas, or projections.
  - Handles sorting, pagination (`clampPageLimit`), and filtering.
- **Examples in repo**:
  - `AccountQueryService.GetBalance`: Returns current cleared and available balances.
  - `TransferQueryService.ListTransfers`: Returns cursor-paginated transfer history.

### 5.3. `port/` (Application Ports)
- **Definition**: Interfaces declared by the application layer defining everything it needs from the outside world (Inversion of Control).
- **Types of Ports**:
  - **Infrastructure Ports**: `IdempotencyStore`, `Authorizer`, `UnitOfWork`, `OutboxStore`, `Clock`, `IDGenerator`.
  - **Gateway Ports**: `PaymentProcessorGateway`, `BankSettlementGateway`.
  - **Use Case Ports**: `AccountCommandUseCases`, `AccountQueryUseCases` (consumed by REST/gRPC handlers).

### 5.4. `dto/` (Data Transfer Objects)
- **Definition**: Simple, serializable request and response models that cross the delivery $\leftrightarrow$ application boundary.
- **Characteristics**:
  - Decoupled from domain entities (protects domain from leaking to the outside world).
  - Decoupled from database tables (protects API contracts from schema migrations).

### 5.5. `workflow/` (Sagas & Long-Running Workflows)
- **Definition**: Coordinates multi-step operations that span multiple aggregates, transactions, or external services where immediate consistency is impossible.
- **Examples in repo**:
  - `BatchTransferWorkflow`: Processes 1,000 items in a batch transfer, tracking partial failures and converging batch status.
  - `ReconciliationSaga`: Ingests bank statements, matches ledger entries, and raises reconciliation breaks.

### 5.6. Service Construction & Encapsulation: The Parameter Object Pattern
In Clean Architecture, application services and workflow runners hold long-lived dependencies (Unit of Work, repositories, event outboxes, clocks, authorizers). To ensure robustness, maintainability, and thread safety:
- **Strict Encapsulation**: All fields in service structs must be **unexported** (e.g. `uow`, `accounts`, `clock`, `authz`). External packages must never mutate internal dependencies on an instantiated service.
- **Parameter Object Pattern (`<Service>Params`)**: Every service defines a companion parameter object struct containing exported dependencies.
- **Constructor Functions (`New<Service>`)**: The constructor accepts the parameter object, validates or defaults dependencies, and returns a pointer to the encapsulated service struct.
- **Benefits**:
  - Prevents argument list explosion as services evolve.
  - Guarantees compatibility with both compile-time (`wire`) and container-based (`uber-go/fx`) dependency injection.
  - Isolates unit tests: tests supply only the fakes or ports needed without dealing with positional argument ordering.

---

## 6. How It Fits Together: Request Lifecycle

Here is the exact trace of an incoming HTTP request:

```
[ HTTP POST /v1/transfers ]
         │
         ▼
1. INTERFACE LAYER (`internal/interface/rest/`):
   - Echo handler decodes JSON into an Application DTO (`dto.TransferRequest`).
   - Extracts Actor/Auth tokens from context.
   - Calls Application Port: `app.TransferCommandUseCases.CreateTransfer(ctx, req)`.
         │
         ▼
2. APPLICATION LAYER (`internal/application/command/transfer.go`):
   - Step A: Envelope validation (missing fields?).
   - Step B: Authorization: `authz.Require(...)`.
   - Step C: Idempotency reservation: `Reserve(idempotencyKey)`.
   - Step D: Begin Database Transaction via Unit of Work (`port.UoW`).
         │
         ▼
3. DOMAIN LAYER (`internal/domain/`):
   - Repository loads `AccountAggregate` for source and destination accounts.
   - Domain Aggregate executes: `sourceAccount.Debit(amount)`.
     -> Invariant check: Are available funds >= amount?
     -> Invariant check: Is source account frozen?
   - Domain Service calculates fee: `service.AssessTransactionFee(...)`.
   - Aggregate creates balanced `PostingAggregate` (debits == credits).
   - Domain Event produced: `event.TransferCreated`.
         │
         ▼
4. APPLICATION LAYER (`internal/application/`):
   - Persist modified accounts and posting via `port.Tx`.
   - Stage `event.TransferCreated` into the Outbox table within the SAME transaction.
   - Complete idempotency record with response payload.
   - Commit transaction.
         │
         ▼
5. INTERFACE LAYER (`internal/interface/rest/`):
   - Maps domain result to HTTP 201 Created JSON response.
```

---

## 7. Comparison: Past Architecture vs. Modern Clean Architecture

| Concern | Past Flat Architecture | Modern Clean Architecture (This Repository) |
| :--- | :--- | :--- |
| **Entities** | Structs with DB/JSON tags, public fields, zero logic. | Rich models (`domain/entity`, `domain/valueobject`, `domain/aggregate`) with self-validation and invariant protection. Zero external tags. |
| **Business Logic** | Dumped into large `service` functions mixed with SQL queries and HTTP error codes. | Pure functions and methods in `domain/service` and `domain/aggregate`. Zero I/O, stdlib only. |
| **Use Cases** | Ambiguous boundary between `service` and `usecase`. | Explicit CQRS segregation: `application/command` (mutations) and `application/query` (reads). |
| **Repositories** | Concrete DB implementations calling GORM/SQL directly from services. | Interfaces defined in `domain/repository` or `application/port`; implemented strictly in `internal/infrastructure/database/`. |
| **External APIs** | Called directly inside services, making testing impossible without network mocking. | Defined as application ports (`port.PaymentGateway`); adapted in `infrastructure/`. |
| **Transactions** | Often manually managed in services or leaky SQL transactions. | Coordinated cleanly using the **Unit of Work** (`port.UoW` / `port.Tx`) abstraction. |
| **Error Handling** | Raw SQL or HTTP errors passed around. | Domain returns semantic `entity.Error`; Application/Interface translates to standardized codes via `ToAppError`. |
| **Unit Testing** | Requires mocking DB connections, Redis, and HTTP contexts for basic logic. | Domain tests run 100% in-memory with zero mocks. Application tests use simple in-memory fakes. |

---

## 8. Summary Checklist for Engineers & Agents

Before adding new code, check:

- [ ] **Zero External Imports in Domain**: Does anything in `internal/domain/` import third-party packages or other `internal/` packages? (Must be stdlib only).
- [ ] **No Business Math in Application**: Did you put a fee calculation, balance formula, or state machine check in `application/command`? (Move it to `domain/service` or `domain/aggregate`).
- [ ] **No I/O in Domain**: Does any domain entity or service do database queries, HTTP calls, or file reading? (Domain must be pure).
- [ ] **CQRS Segregation**: Are commands mutating state and queries purely reading? (Never mutate state in a query handler).
- [ ] **Unit Tests Adhere to AGENTS.md**: Are unit tests table-driven, locally scoped, and thoroughly covering boundaries, overflow, and replays?
