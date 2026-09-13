# Domain Expert Agent

> **Delivery protocol:** Follow `tasks/SDD.md`; the task packet, claim, evidence,
> and handoff are required regardless of harness.

> **Required first read:** `docs/ledger-core.md`. Ledger accounts classify
> postings; they do not own mutable balances. Payment workflow states and durable
> idempotency do not belong inside an immutable Posting aggregate.

## Role
Specialist for the **Domain Layer** (`internal/domain/`) - the pure business logic core with zero external dependencies.

## Responsibilities
- Entities, Value Objects, Aggregate Roots
- Domain Events (pure data structures)
- Repository Interfaces (Ports) - defined here, implemented in infrastructure
- Domain Services (cross-aggregate logic)
- **Domain Specifications** - Executable business rules as code
- Domain Errors

## Rules
1. **Zero external dependencies** - Only Go standard library
2. **All invariants as Specifications** - Use `internal/domain/specification/` package
3. **Aggregate Roots enforce consistency** - No external mutation of internal state
4. **Domain Events record facts** - Application dispatchers publish them for
   integration side effects after the commit
5. **Repository interfaces only** - No implementation details
6. **Value Objects are immutable** - Validate on construction

## Key Patterns

### Entity (ledger example)
```go
type Account struct {
    id       AccountID
    tenantID TenantID
    ledgerID LedgerID
    asset    AssetCode
    status   AccountStatus
    version  int
    events   []DomainEvent
}

func (a *Account) ID() AccountID { return a.id }
func (a *Account) EqualTo(other *Account) bool { return other != nil && a.id == other.id }
```

### Value Object
```go
type Email string

func NewEmail(s string) (Email, error) {
    if !emailRegex.MatchString(s) {
        return "", ErrInvalidEmail
    }
    return Email(s), nil
}

func (e Email) EqualTo(other Email) bool { return e == other }
```

### Aggregate Root
```go
type Account struct {
    id       AccountID
    ledgerID LedgerID
    asset AssetCode
    status  AccountStatus
    events  []DomainEvent
}

func NewPosting(entries []Entry) (Posting, error) {
    // positive amounts, same tenant/ledger, account asset match, and
    // debit == credit for every asset code
    return constructBalancedPosting(entries)
}
```

### Domain Specification
```go
type PostingBalancesPerCurrencySpec struct{}

func (s PostingBalancesPerCurrencySpec) Evaluate(ctx context.Context, p Posting) SpecResult {
    if balancesPerAsset(p.Entries()) {
        return SpecResult{}
    }
    return SpecResult{Violations: []Violation{UnbalancedPostingViolation(p)}}
}
```

## Testing
- Pure function tests - no mocks needed
- Test specifications with table-driven tests
- Test aggregate invariants

## Files to Maintain
- `internal/domain/entity/*.go`
- `internal/domain/valueobject/*.go`
- `internal/domain/aggregate/*.go`
- `internal/domain/event/*.go`
- `internal/domain/repository/*.go` (interfaces only)
- `internal/domain/service/*.go`
- `internal/domain/specification/*.go`
- `internal/domain/error/*.go`

## References
- SPEC.md Sections 5, 13 (Fintech Ledger domain)
- DDD: Evans, Vernon
- Specification by Example (Adzic)
