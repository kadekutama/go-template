# Domain-Driven Design Modeling Skill

> Apply this skill inside the task packet and takeover protocol in `tasks/SDD.md`.

> For this repository's reference domain, `docs/ledger-core.md` is normative.
> Generic DDD examples must not introduce mutable account balances, workflow
> states on postings, or repository-aware idempotency specifications.

## Strategic Design

### Bounded Contexts
- Each context = separate module/package
- Explicit context maps (shared kernel, customer/supplier, anticorruption)
- **This template**: Single bounded context (can be split later)

### Ubiquitous Language
- Domain terms in code: `Account`, `Posting`, `Entry`, `Money`, `AssetCode`
- No technical jargon in domain layer
- Events named in past tense: `AccountOpened`, `TransactionPosted`

## Tactical Design

### Entities
- Identity + continuity + lifecycle
- Mutable state encapsulated
- Equality by ID only
- Example: `Account`, `Posting`, `Hold`

### Value Objects
- No identity, immutable, replaceable
- Equality by all fields
- Self-validating on construction
- Example: `Email`, `Money`, `Currency`, `AccountID`, `TransactionID`

### Aggregates
- **Aggregate Root** = entry point, enforces invariants
- **Consistency boundary** - transactional consistency within
- **References by ID only** - no direct object references across aggregates
- Small aggregates preferred
- Example: `Account` (metadata root), `Posting` (immutable root), and `Hold`
  (authorization root)

### Domain Events
- Something that happened in the domain
- Immutable, published after transaction commits
- Enable eventual consistency across aggregates
- Example: `AccountOpened`, `TransactionPosted` (compatibility name for a
  committed Posting), `BalanceChanged`

### Domain Services
- Stateless operations spanning multiple aggregates
- Named with business verbs: `TransferService`, `ReconciliationService`
- Not application services (no orchestration, no DTOs)

### Repositories
- **Interface in Domain**, Implementation in Infrastructure
- Narrow, consumer-owned interfaces: use intent-specific methods such as
  `Create`, `FindByID`, and metadata updates; do not require generic `Save` or
  `Delete` for immutable ledger facts.
- Return domain objects, not DTOs
- Specification pattern for complex queries

### Domain Specifications
- Business rules as composable, executable objects
- Stateless `Evaluate(candidate) SpecResult` with typed violations
- `All`, `Any`, `Not` combinators with explicit evaluation semantics
- Used in aggregates, domain services, application layer

## Fintech Ledger Example

### Aggregates
```
Ledger / Account (metadata roots)
├── ID: AccountID
├── TenantID: TenantID
├── LedgerID: LedgerID
├── Name: string
├── Type: AccountType (ASSET, LIABILITY, EQUITY, REVENUE, EXPENSE)
├── AssetCode: AssetCode
├── NormalSide: DEBIT | CREDIT
├── Status: AccountStatus
└── Version: int (optimistic lock)

Posting (Root, immutable after construction)
├── ID: PostingID
├── TenantID + LedgerID
├── Reference: string
├── Entries: []Entry
├── EffectiveAt: time.Time
├── RecordedAt: time.Time
└── ReversalOf: *PostingID

Entry (Entity within Posting)
├── ID: EntryID
├── AccountID: AccountID
├── Side: DEBIT | CREDIT
├── AmountMinor: positive int64
├── AssetCode: AssetCode
└── AccountSequence: int64

Hold (Root)
├── AccountID + AssetCode + AmountMinor
├── Kind + ExpiresAt
└── State: ACTIVE | CAPTURED | RELEASED | EXPIRED
```

### Invariants (Specifications)
1. **PostingBalancesPerCurrency** - Debit == credit for every asset code
2. **EntryAmountPositive** - Side carries direction; amount is positive minor units
3. **ValidCurrency** - Entry asset matches its account
4. **SameLedger** - All accounts share tenant and ledger scope
5. **AccountActive** - Account accepts the posting template
6. **PostingTemplateAllowed** - Versioned operation permits the accounts/sides

`SufficientFunds` evaluates an authoritative balance snapshot passed into the
pure rule. Durable idempotency uniqueness/fingerprints and lock serialization are
application/database invariants, not domain repository lookups.

### Domain Events
- `AccountOpened`, `AccountClosed`, `AccountFrozen`, `AccountUnfrozen`
- `TransactionPosted`, `TransactionReversed`, `TransactionFailed`
- `BalanceChanged` (per account, per transaction)

## Modeling Process
1. **Event Storming** - Discover domain events
2. **Identify Aggregates** - Consistency boundaries
3. **Define Value Objects** - Immutable concepts
4. **Write Specifications** - Executable invariants
5. **Design Repositories** - Persistence interfaces
6. **Model Domain Services** - Cross-aggregate logic

## References
- Domain-Driven Design (Evans)
- Implementing DDD (Vernon)
- Domain Modeling Made Functional (Wlaschin)
- SPEC.md Section 13 (Fintech Ledger reference)
