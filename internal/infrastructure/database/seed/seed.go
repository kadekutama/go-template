// Package seed builds the deterministic dev-seed plan (E07-T04) and applies
// it idempotently through narrow store interfaces. Stable IDs are owned here;
// test/fixtures mirrors them (parity proven in seed_test.go). Production code
// never imports test helpers.
package seed

import (
	"context"
	"fmt"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// Stable seed identifiers (single source of truth for the dev plan).
const (
	TenantID = "tnt-test-01"
	LedgerID = "ldg-test-01"

	AcctOperatingUSD = "acct-test-operating-USD"
	AcctOperatingEUR = "acct-test-operating-EUR"
	AcctOperatingIDR = "acct-test-operating-IDR"
	AcctFeeUSD       = "acct-test-fee-USD"
	AcctFeeEUR       = "acct-test-fee-EUR"
	AcctFeeIDR       = "acct-test-fee-IDR"
	AcctSuspenseUSD  = "acct-test-suspense-USD"
	AcctSuspenseEUR  = "acct-test-suspense-EUR"
	AcctSuspenseIDR  = "acct-test-suspense-IDR"
)

// Seed classification literals matching chart AccountClass and Direction value objects.
const (
	ClassOperating = string(valueobject.ClassLiability)
	ClassFee       = string(valueobject.ClassRevenue)
	ClassSuspense  = string(valueobject.ClassAsset)
	StatusActive   = string(valueobject.StatusActive)
	AssetUSD       = "USD"
	AssetEUR       = "EUR"
	AssetIDR       = "IDR"
	SideDebit      = string(valueobject.DirectionDebit)
	SideCredit     = string(valueobject.DirectionCredit)
)

// TenantSeed is the dev tenant to ensure.
type TenantSeed struct {
	TenantID string
	LedgerID string
	Name     string
	Region   string
}

// LedgerSeed is one ledger to ensure.
type LedgerSeed struct {
	ID           string
	TenantID     string
	Name         string
	BaseAsset    string
	ChartVersion string
}

// AccountSeed is one account to ensure.
type AccountSeed struct {
	ID        string
	TenantID  string
	LedgerID  string
	Number    string
	Name      string
	Class     string
	AssetCode string
	Status    string
	Version   int64
}

// PostingLine is one deterministic journal line in minor units.
type PostingLine struct {
	AccountID   string
	Side        string
	AmountMinor int64
	AssetCode   string
}

// PostingSeed is one posting to ensure with balanced minor-unit lines.
type PostingSeed struct {
	ID          string
	TenantID    string
	LedgerID    string
	Operation   string
	Description string
	AmountMinor int64
	AssetCode   string
	Debit       AccountSeed
	Credit      AccountSeed
}

// Plan is the full deterministic seed.
type Plan struct {
	Tenant   TenantSeed
	Ledgers  []LedgerSeed
	Accounts []AccountSeed
	Postings []PlannedPosting
}

// PlannedPosting is a balanced posting described by journal lines.
type PlannedPosting struct {
	ID          string
	TenantID    string
	LedgerID    string
	Operation   string
	Description string
	Lines       []PostingLine
}

// DevPlan returns the default development plan covering funding, transfer,
// fee, and refund patterns.
func DevPlan() Plan {
	tenant := TenantSeed{TenantID: TenantID, LedgerID: LedgerID, Name: "Test Tenant 01", Region: "local"}

	accounts := []AccountSeed{
		{ID: AcctOperatingUSD, TenantID: TenantID, LedgerID: LedgerID, Number: "001000", Name: AcctOperatingUSD, Class: ClassOperating, AssetCode: AssetUSD, Status: StatusActive, Version: 1},
		{ID: AcctOperatingEUR, TenantID: TenantID, LedgerID: LedgerID, Number: "001001", Name: AcctOperatingEUR, Class: ClassOperating, AssetCode: AssetEUR, Status: StatusActive, Version: 1},
		{ID: AcctOperatingIDR, TenantID: TenantID, LedgerID: LedgerID, Number: "001002", Name: AcctOperatingIDR, Class: ClassOperating, AssetCode: AssetIDR, Status: StatusActive, Version: 1},
		{ID: AcctFeeUSD, TenantID: TenantID, LedgerID: LedgerID, Number: "002000", Name: AcctFeeUSD, Class: ClassFee, AssetCode: AssetUSD, Status: StatusActive, Version: 1},
		{ID: AcctFeeEUR, TenantID: TenantID, LedgerID: LedgerID, Number: "002001", Name: AcctFeeEUR, Class: ClassFee, AssetCode: AssetEUR, Status: StatusActive, Version: 1},
		{ID: AcctFeeIDR, TenantID: TenantID, LedgerID: LedgerID, Number: "002002", Name: AcctFeeIDR, Class: ClassFee, AssetCode: AssetIDR, Status: StatusActive, Version: 1},
		{ID: AcctSuspenseUSD, TenantID: TenantID, LedgerID: LedgerID, Number: "003000", Name: AcctSuspenseUSD, Class: ClassSuspense, AssetCode: AssetUSD, Status: StatusActive, Version: 1},
		{ID: AcctSuspenseEUR, TenantID: TenantID, LedgerID: LedgerID, Number: "003001", Name: AcctSuspenseEUR, Class: ClassSuspense, AssetCode: AssetEUR, Status: StatusActive, Version: 1},
		{ID: AcctSuspenseIDR, TenantID: TenantID, LedgerID: LedgerID, Number: "003002", Name: AcctSuspenseIDR, Class: ClassSuspense, AssetCode: AssetIDR, Status: StatusActive, Version: 1},
	}

	postings := []PlannedPosting{
		{
			ID: "pst-test-funding-01", TenantID: TenantID, LedgerID: LedgerID,
			Operation: "FUNDING", Description: "initial funding",
			Lines: []PostingLine{
				{AccountID: AcctOperatingUSD, Side: SideDebit, AmountMinor: 100000, AssetCode: AssetUSD},
				{AccountID: AcctSuspenseUSD, Side: SideCredit, AmountMinor: 100000, AssetCode: AssetUSD},
			},
		},
		{
			ID: "pst-test-transfer-01", TenantID: TenantID, LedgerID: LedgerID,
			Operation: "TRANSFER", Description: "test transfer",
			Lines: []PostingLine{
				{AccountID: AcctOperatingUSD, Side: SideCredit, AmountMinor: 25000, AssetCode: AssetUSD},
				{AccountID: AcctOperatingEUR, Side: SideDebit, AmountMinor: 25000, AssetCode: AssetUSD},
			},
		},
		{
			ID: "pst-test-fee-01", TenantID: TenantID, LedgerID: LedgerID,
			Operation: "FEE", Description: "test fee",
			Lines: []PostingLine{
				{AccountID: AcctOperatingUSD, Side: SideDebit, AmountMinor: 290, AssetCode: AssetUSD},
				{AccountID: AcctFeeUSD, Side: SideCredit, AmountMinor: 290, AssetCode: AssetUSD},
			},
		},
		{
			ID: "pst-test-refund-01", TenantID: TenantID, LedgerID: LedgerID,
			Operation: "REFUND", Description: "test refund",
			Lines: []PostingLine{
				{AccountID: AcctOperatingUSD, Side: SideDebit, AmountMinor: 5000, AssetCode: AssetUSD},
				{AccountID: AcctSuspenseUSD, Side: SideCredit, AmountMinor: 5000, AssetCode: AssetUSD},
			},
		},
	}

	return Plan{
		Tenant: tenant,
		Ledgers: []LedgerSeed{
			{ID: LedgerID, TenantID: TenantID, Name: "Dev Ledger", BaseAsset: AssetUSD, ChartVersion: "v1"},
		},
		Accounts: accounts,
		Postings: postings,
	}
}

// LedgerStore ensures ledgers by ID.
type LedgerStore interface {
	Exists(ctx context.Context, id string) (bool, error)
	Create(ctx context.Context, ledger LedgerSeed) error
}

// AccountStore ensures accounts by ID.
type AccountStore interface {
	Exists(ctx context.Context, id string) (bool, error)
	Create(ctx context.Context, account AccountSeed) error
}

// PostingStore ensures postings by ID.
type PostingStore interface {
	Exists(ctx context.Context, id string) (bool, error)
	Create(ctx context.Context, posting PostingSeed) error
}

// Stores bundles the narrow dependencies Apply needs.
type Stores struct {
	Ledgers  LedgerStore
	Accounts AccountStore
	Postings PostingStore
}

// Apply ensures every planned row, skipping stable IDs that already exist.
// Running Apply twice yields the same row set (idempotent).
func Apply(ctx context.Context, plan Plan, stores Stores) error {
	if stores.Ledgers == nil || stores.Accounts == nil || stores.Postings == nil {
		return fmt.Errorf("seed: ledger, account, and posting stores are required")
	}

	if err := ensureLedgers(ctx, plan.Ledgers, stores.Ledgers); err != nil {
		return err
	}

	if err := ensureAccounts(ctx, plan.Accounts, stores.Accounts); err != nil {
		return err
	}

	return ensurePostings(ctx, plan.Postings, stores.Postings)
}

// ensureLedgers creates missing ledgers by stable ID.
func ensureLedgers(ctx context.Context, ledgers []LedgerSeed, store LedgerStore) error {
	for _, ledger := range ledgers {
		exists, err := store.Exists(ctx, ledger.ID)
		if err != nil {
			return fmt.Errorf("seed: ledger exists: %w", err)
		}

		if exists {
			continue
		}

		if err := store.Create(ctx, ledger); err != nil {
			return fmt.Errorf("seed: create ledger: %w", err)
		}
	}

	return nil
}

// ensureAccounts creates missing accounts by stable ID.
func ensureAccounts(ctx context.Context, accounts []AccountSeed, store AccountStore) error {
	for _, account := range accounts {
		exists, err := store.Exists(ctx, account.ID)
		if err != nil {
			return fmt.Errorf("seed: account exists: %w", err)
		}

		if exists {
			continue
		}

		if err := store.Create(ctx, account); err != nil {
			return fmt.Errorf("seed: create account: %w", err)
		}
	}

	return nil
}

// ensurePostings creates missing postings by stable ID.
func ensurePostings(ctx context.Context, postings []PlannedPosting, store PostingStore) error {
	for _, posting := range postings {
		exists, err := store.Exists(ctx, posting.ID)
		if err != nil {
			return fmt.Errorf("seed: posting exists: %w", err)
		}

		if exists {
			continue
		}

		seed, err := postingSeed(posting)
		if err != nil {
			return err
		}

		if err := store.Create(ctx, seed); err != nil {
			return fmt.Errorf("seed: create posting: %w", err)
		}
	}

	return nil
}

// postingSeed maps one planned posting onto legs by side.
func postingSeed(posting PlannedPosting) (PostingSeed, error) {
	if len(posting.Lines) < 2 {
		return PostingSeed{}, fmt.Errorf("seed: posting %s needs at least two lines", posting.ID)
	}

	seed := PostingSeed{
		ID:          posting.ID,
		TenantID:    posting.TenantID,
		LedgerID:    posting.LedgerID,
		Operation:   posting.Operation,
		Description: posting.Description,
	}

	for _, line := range posting.Lines {
		switch line.Side {
		case SideDebit:
			seed.Debit = AccountSeed{ID: line.AccountID}
			seed.AmountMinor = line.AmountMinor
			seed.AssetCode = line.AssetCode
		case SideCredit:
			seed.Credit = AccountSeed{ID: line.AccountID}
		}
	}

	return seed, nil
}
