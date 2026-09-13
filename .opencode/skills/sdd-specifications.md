# Domain Specification Pattern Skill

> This file describes executable domain rules. It is not the multi-agent
> Specification-Driven Delivery protocol. For task packets, claims, evidence,
> and takeover behavior, follow `tasks/SDD.md`.

> For ledger work, read `docs/ledger-core.md` first. Specifications are pure:
> durable idempotency, database uniqueness, locking, and outbox atomicity are
> verified application/persistence invariants rather than repository calls from
> domain specs.

## Concept
**Domain specifications as executable code** - Business rules written as composable, testable objects that live in the domain layer.

## Specification Pattern

### Core Interface
```go
// internal/domain/specification/specification.go
type Specification[T any] interface {
    Evaluate(ctx context.Context, candidate T) SpecResult
}

type SpecResult struct { Violations []Violation }
func (r SpecResult) Passed() bool { return len(r.Violations) == 0 }
```

### Base Implementation
```go
type spec[T any] struct {
    evaluate func(ctx context.Context, candidate T) SpecResult
}

func (s spec[T]) Evaluate(ctx context.Context, candidate T) SpecResult {
    return s.evaluate(ctx, candidate)
}
```

### Combinators
```go
func All[T any](specs ...Specification[T]) Specification[T] {
    return spec[T]{evaluate: func(ctx context.Context, candidate T) SpecResult {
        var violations []Violation
        for _, s := range specs {
            violations = append(violations, s.Evaluate(ctx, candidate).Violations...)
        }
        return SpecResult{Violations: violations}
    }}
}

// Any short-circuits on first pass; if none pass it returns all violations.
// Not receives an explicit violation to return when its child passes.
```

## Creating Specifications

### Factory Function
```go
func NewSpec[T any](name string, check func(context.Context, T) bool, violation Violation) Specification[T] {
    return spec[T]{evaluate: func(ctx context.Context, candidate T) SpecResult {
        if check(ctx, candidate) { return SpecResult{} }
        return SpecResult{Violations: []Violation{violation}}
    }}
}
```

### Domain-Specific Specifications
```go
// internal/domain/specification/account/specs.go
package accountspec

var (
    // AccountNotFrozen - Account must be ACTIVE
    AccountNotFrozen = NewSpec("account_not_frozen",
        func(ctx context.Context, a *Account) bool {
            return a.Status() == AccountStatusActive
        },
        ErrAccountFrozen,
    )

    // ValidCurrency - Account currency must be supported
    ValidCurrency = NewSpec("valid_currency",
        func(ctx context.Context, a *Account) bool {
            return currency.IsSupported(a.Currency())
        },
        ErrUnsupportedCurrency,
    )

    // PostingBalancesPerCurrency - every asset lot balances independently
    PostingBalancesPerCurrency = NewSpec("posting_balances_per_currency",
        func(ctx context.Context, p Posting) bool {
            return balancesPerAsset(p.Entries())
        },
        ErrUnbalancedPosting,
    )
)
```

### Parameterized Specifications
```go
type FundsCandidate struct {
    Snapshot BalanceSnapshot // loaded strongly by the application/repository
    Amount   Money
}

func SufficientFunds() Specification[FundsCandidate] {
    return NewSpec("sufficient_funds",
        func(ctx context.Context, c FundsCandidate) bool {
            return c.Snapshot.Available.Cmp(c.Amount) >= 0
        },
        ErrInsufficientFunds,
    )
}
```

## Using Specifications

### In Aggregate Root
```go
func NewPosting(ctx context.Context, candidate Posting) (Posting, error) {
    result := All(postingspec.PostingBalancesPerCurrency, postingspec.EntryAmountPositive).Evaluate(ctx, candidate)
    if !result.Passed() {
        return Posting{}, NewDomainError(result.Violations)
    }
    return candidate, nil
}
```

### In Domain Service
```go
func (s *TransferService) ExecuteTransfer(ctx context.Context, c TransferCandidate) (TransferResult, error) {
    spec := transferspec.ValidTransfer()
    result := spec.Evaluate(ctx, c)
    if !result.Passed() {
        return TransferResult{}, NewDomainError(result.Violations)
    }
    // ... execute transfer
    return TransferResult{/* posting ID */}, nil
}
```

### In Application Layer (Command Handler)
```go
func (h *TransferCommandHandler) Handle(ctx context.Context, cmd TransferCommand) (*TransferResult, error) {
    // Validate via specifications before executing
    spec := h.specProvider.ForTransfer(cmd)
    result := spec.Evaluate(ctx, cmd)
    if !result.Passed() {
        return nil, NewDomainError(result.Violations)
    }
    candidate := TransferCandidate{
        FromAccountID: cmd.FromAccountID,
        ToAccountID:   cmd.ToAccountID,
        Amount:        cmd.Amount,
    }
    result, err := h.transferService.ExecuteTransfer(ctx, candidate)
    if err != nil {
        return nil, err
    }
    return &result, nil
}
```

## Testing Specifications

### Unit Tests (Table-Driven)
```go
func TestAccountNotFrozenSpec(t *testing.T) {
    tests := []struct {
        name     string
        status   AccountStatus
        expected bool
    }{
        {"active", AccountStatusActive, true},
        {"frozen", AccountStatusFrozen, false},
        {"closed", AccountStatusClosed, false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            acc := &Account{status: tt.status}
            spec := accountspec.AccountNotFrozen
            result := spec.Evaluate(context.Background(), acc)
            assert.Equal(t, tt.expected, result.Passed())
            if !tt.expected {
                assert.Contains(t, result.Violations, AccountFrozenViolation)
            }
        })
    }
}
```

### Combinator Tests
```go
func TestSpecCombinators(t *testing.T) {
    specA := AlwaysTrue()
    specB := AlwaysFalse()
    
    // AND
    allSpec := All(specA, specB)
    assert.False(t, allSpec.Evaluate(ctx, nil).Passed())
    
    // OR
    anySpec := Any(specA, specB)
    assert.True(t, anySpec.Evaluate(ctx, nil).Passed())
    
    // NOT
    notSpec := Not(specA, UnexpectedPassViolation)
    assert.False(t, notSpec.Evaluate(ctx, nil).Passed())
}
```

## Documentation (domain specifications as living docs)

### Markdown Specification Table
```markdown
# Account Specifications

| Spec ID | Name | Description | Error Code |
|---------|------|-------------|------------|
| ACC-001 | AccountNotFrozen | Account must be ACTIVE status | ACCOUNT_FROZEN |
| ACC-002 | ValidCurrency | Currency must be ISO 4217 supported | UNSUPPORTED_CURRENCY |
| PST-001 | PostingBalancesPerCurrency | Debit equals credit for every asset | UNBALANCED_TRANSACTION |
| PST-002 | EntryAmountPositive | Every entry is positive minor units | INVALID_ENTRY_AMOUNT |

## Composition Rules
- Transfer posting requires: AccountActive AND SufficientFunds(snapshot) AND PostingTemplateAllowed
- Posting construction requires: EntryAmountPositive AND PostingBalancesPerCurrency AND SameLedger
```

### Generated from Code
```go
// scripts/generate/generate-spec-docs.go
// Parses specification definitions → Markdown tables
// Runs in CI to keep docs in sync
```

## Best Practices

1. **One spec per business rule** - Small, focused, reusable
2. **Name by business term** - `SufficientFunds`, not `BalanceCheck`
3. **Composable** - Build complex rules from simple ones
4. **Testable** - Pure functions, easy to unit test
5. **Documented** - Error codes map to user messages
6. **Versioned** - Keep stable spec IDs (ACC-001, TXN-005) for documentation
   and evidence; map violations to the public error-code registry rather than
   embedding spec IDs in transport errors.

## References
- Specification by Example (Gojko Adzic)
- Domain-Driven Design (Evans) - Specification Pattern
- SPEC.md Sections 5.2, 13.2
