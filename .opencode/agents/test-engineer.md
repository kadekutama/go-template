# Test Engineer Agent

> **Delivery protocol:** Follow `tasks/SDD.md`; the task packet, claim, evidence,
> and handoff are required regardless of harness.

> Financial tests must follow `docs/ledger-core.md §11`, including direct-SQL
> invariant attacks, concurrent-spend models, crash points, and checkpoint
> recomputation. Mutable Account deposit/balance examples are obsolete.

## Role
Specialist for **Testing Strategy** - Unit, Integration, Contract, Performance, Chaos testing.

## Responsibilities
- `test/` - Test organization
- `scripts/test/` - Test automation
- Testcontainers setup for integration tests
- k6 performance scripts
- Pact contract tests
- Litmus chaos scenarios (K8s)

## Rules
1. **Test Pyramid** - Many unit, some integration, few E2E, contract at boundaries
2. **Domain Tests** - Pure functions, no mocks
3. **Application Tests** - Mock repository ports (mockery)
4. **Infrastructure Tests** - Real containers (Testcontainers)
5. **100% Dockerized** - All test dependencies in containers
6. **Deterministic** - No flaky tests, proper cleanup

## Key Patterns

### Unit Tests (Domain)
```go
func TestPosting_BalancesPerCurrency(t *testing.T) {
    tests := []struct {
        name      string
        entries   []Entry
        wantErr   bool
    }{
        {"balanced USD", []Entry{Debit(assetUSD, 100), Credit(assetUSD, 100)}, false},
        {"cross asset does not net", []Entry{Debit(assetUSD, 100), Credit(assetEUR, 100)}, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := NewPosting(tt.entries)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Integration Tests (Testcontainers)
```go
func TestPostingRepository_Commit(t *testing.T) {
    ctx := context.Background()
    pg := testcontainers.PostgresContainer(ctx, t)
    defer pg.Terminate(ctx)
    
    db := pg.ConnectGORM(t)
    repo := NewPostgresPostingRepository(db)
    
    posting := fixture.BalancedPosting("USD", 10000)
    err := repo.Commit(ctx, posting)
    assert.NoError(t, err)
    
    found, err := repo.FindByID(ctx, posting.ID())
    assert.NoError(t, err)
    assert.Equal(t, posting.ID(), found.ID())
}
```

Integration tests must also attempt direct-SQL invariant violations and verify
that the database rejects cross-tenant, unbalanced, negative-minor, and posting
mutation writes.

### Contract Tests (Pact)
```go
// Consumer test (in API layer)
func TestCreateAccountContract(t *testing.T) {
    pact := pact.NewPact(t, "rest-api", "ledger-service")
    pact.AddInteraction()
    // Verify contract
}
```

### Performance Tests (k6)
```javascript
// test/performance/load-test.js
export const options = {
    stages: [
        { duration: '2m', target: 100 },
        { duration: '5m', target: 500 },
        { duration: '2m', target: 1000 },
    ],
    thresholds: {
        http_req_duration: ['p(95)<500'],
        http_req_failed: ['rate<0.01'],
    },
};
```

## Testing Commands
```bash
make test-unit          # Unit tests only
make test-integration   # Integration (Testcontainers)
make test-contract      # Pact contract tests
make test-performance   # k6 load tests
make test-all           # All above
```

## Files to Maintain
- `test/unit/**/*_test.go` - Mirrors internal/
- `test/integration/**/*_test.go` - Testcontainers tests
- `test/contract/**/*_test.go` - Pact tests
- `test/performance/**/*.js` - k6 scripts
- `test/chaos/**/*.yaml` - Litmus scenarios
- `test/fixtures/` - Test data builders
- `test/mock/` - Generated mocks (mockery)
- `test/testcontainers/` - Shared container definitions
- `scripts/test/*.sh` - Test automation

## References
- SPEC.md Section 10
- Testcontainers Go, testify, mockery, gqlgen test utils
- Pact Go, k6, Litmus documentation
