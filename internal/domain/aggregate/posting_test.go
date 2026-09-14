package aggregate_test

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/aggregate"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// stubIDs is a deterministic valueobject.IDGenerator for reversal entry IDs.
type stubIDs struct {
	mu sync.Mutex
	n  int
}

func (s *stubIDs) NewID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.n++
	return fmt.Sprintf("11111111-1111-4111-8111-%012x", s.n)
}

func postingAccounts() map[valueobject.AccountID]entity.AccountData {
	mk := func(id valueobject.AccountID, number string, class valueobject.AccountClass, status valueobject.AccountStatus) entity.AccountData {
		return entity.AccountData{
			ID: id, TenantID: testTenantID, LedgerID: testLedgerID, Number: number, Name: number,
			Class: class, AssetCode: testUSD, Status: status, Version: 1,
		}
	}
	return map[valueobject.AccountID]entity.AccountData{
		testAccount1: mk(testAccount1, "1000", valueobject.ClassAsset, valueobject.StatusActive),
		testAccount2: mk(testAccount2, "2000", valueobject.ClassLiability, valueobject.StatusActive),
	}
}

func postingEntries(pid valueobject.PostingID, dAmt, cAmt int64) []entity.Entry {
	return []entity.Entry{
		{ID: "e-1", PostingID: pid, AccountID: testAccount1, Side: valueobject.DirectionDebit, AmountMinor: dAmt, AssetCode: testUSD, AccountSeq: 1},
		{ID: "e-2", PostingID: pid, AccountID: testAccount2, Side: valueobject.DirectionCredit, AmountMinor: cAmt, AssetCode: testUSD, AccountSeq: 1},
	}
}

func postingParams(pid valueobject.PostingID, entries []entity.Entry, accounts map[valueobject.AccountID]entity.AccountData) aggregate.PostingParams {
	at := time.Date(2026, 9, 14, 7, 0, 0, 0, time.UTC)
	return aggregate.PostingParams{
		ID: pid, TenantID: testTenantID, LedgerID: testLedgerID, Operation: "transfer.v1",
		Description: "test", Entries: entries, Accounts: accounts,
		EffectiveAt: at, RecordedAt: at, EventID: testEvent1,
	}
}

func TestConstructPosting(t *testing.T) {
	t.Parallel()

	baseParams := postingParams(testPosting1, postingEntries(testPosting1, 250, 250), postingAccounts())

	type testCase struct {
		name          string
		p             aggregate.PostingParams
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid balanced posting",
			p:             baseParams,
			expectedError: nil,
		},
		{
			name: "unbalanced entries debits 100 credits 90",
			p: func() aggregate.PostingParams {
				p := baseParams
				p.Entries = postingEntries(testPosting1, 100, 90)
				return p
			}(),
			expectedError: &entity.Error{
				Code:    "UNBALANCED_TRANSACTION",
				Message: "asset USD unbalanced: debits=100 credits=90",
			},
		},
		{
			name: "balanced cross currency FX lots",
			p: func() aggregate.PostingParams {
				accounts := postingAccounts()
				accounts[testAccount3] = entity.AccountData{ID: testAccount3, TenantID: testTenantID, LedgerID: testLedgerID, Number: "3000", Name: "eur", Class: valueobject.ClassAsset, AssetCode: testEUR, Status: valueobject.StatusActive, Version: 1}
				accounts[testAccount4] = entity.AccountData{ID: testAccount4, TenantID: testTenantID, LedgerID: testLedgerID, Number: "4000", Name: "fx", Class: valueobject.ClassLiability, AssetCode: testEUR, Status: valueobject.StatusActive, Version: 1}
				entries := []entity.Entry{
					{ID: "e-1", PostingID: testPosting1, AccountID: testAccount1, Side: valueobject.DirectionDebit, AmountMinor: 10850, AssetCode: testUSD, AccountSeq: 1},
					{ID: "e-2", PostingID: testPosting1, AccountID: testAccount2, Side: valueobject.DirectionCredit, AmountMinor: 10850, AssetCode: testUSD, AccountSeq: 1},
					{ID: "e-3", PostingID: testPosting1, AccountID: testAccount3, Side: valueobject.DirectionDebit, AmountMinor: 10000, AssetCode: testEUR, AccountSeq: 1},
					{ID: "e-4", PostingID: testPosting1, AccountID: testAccount4, Side: valueobject.DirectionCredit, AmountMinor: 10000, AssetCode: testEUR, AccountSeq: 1},
				}
				params := postingParams(testPosting1, entries, accounts)
				params.Metadata = map[string]string{"fx_trade_id": "fx-1"}
				return params
			}(),
			expectedError: nil,
		},
		{
			name: "broken FX lot in secondary currency",
			p: func() aggregate.PostingParams {
				accounts := postingAccounts()
				accounts[testAccount3] = entity.AccountData{ID: testAccount3, TenantID: testTenantID, LedgerID: testLedgerID, Number: "3000", Name: "eur", Class: valueobject.ClassAsset, AssetCode: testEUR, Status: valueobject.StatusActive, Version: 1}
				accounts[testAccount4] = entity.AccountData{ID: testAccount4, TenantID: testTenantID, LedgerID: testLedgerID, Number: "4000", Name: "fx", Class: valueobject.ClassLiability, AssetCode: testEUR, Status: valueobject.StatusActive, Version: 1}
				entries := []entity.Entry{
					{ID: "e-1", PostingID: testPosting1, AccountID: testAccount1, Side: valueobject.DirectionDebit, AmountMinor: 10850, AssetCode: testUSD, AccountSeq: 1},
					{ID: "e-2", PostingID: testPosting1, AccountID: testAccount2, Side: valueobject.DirectionCredit, AmountMinor: 10850, AssetCode: testUSD, AccountSeq: 1},
					{ID: "e-3", PostingID: testPosting1, AccountID: testAccount3, Side: valueobject.DirectionDebit, AmountMinor: 10000, AssetCode: testEUR, AccountSeq: 1},
					{ID: "e-4", PostingID: testPosting1, AccountID: testAccount4, Side: valueobject.DirectionCredit, AmountMinor: 9999, AssetCode: testEUR, AccountSeq: 1},
				}
				return postingParams(testPosting1, entries, accounts)
			}(),
			expectedError: &entity.Error{
				Code:    "UNBALANCED_TRANSACTION",
				Message: "asset EUR unbalanced: debits=10000 credits=9999",
			},
		},
		{
			name: "frozen account rejected",
			p: func() aggregate.PostingParams {
				accounts := postingAccounts()
				frozen := accounts[testAccount1]
				frozen.Status = valueobject.StatusFrozen
				accounts[testAccount1] = frozen
				return postingParams(testPosting1, postingEntries(testPosting1, 100, 100), accounts)
			}(),
			expectedError: entity.Errorf("ACCOUNT_FROZEN", "account %s is frozen", testAccount1),
		},
		{
			name: "closed account rejected",
			p: func() aggregate.PostingParams {
				accounts := postingAccounts()
				closed := accounts[testAccount1]
				closed.Status = valueobject.StatusClosed
				accounts[testAccount1] = closed
				return postingParams(testPosting1, postingEntries(testPosting1, 100, 100), accounts)
			}(),
			expectedError: entity.Errorf("ACCOUNT_CLOSED", "account %s is closed", testAccount1),
		},
		{
			name: "unknown account rejected",
			p: func() aggregate.PostingParams {
				p := baseParams
				p.Accounts = map[valueobject.AccountID]entity.AccountData{}
				return p
			}(),
			expectedError: entity.Errorf("ACCOUNT_NOT_FOUND", "account %s is unknown", testAccount1),
		},
		{
			name: "account tenant mismatch",
			p: func() aggregate.PostingParams {
				accounts := postingAccounts()
				mismatch := accounts[testAccount1]
				mismatch.TenantID = "other-tenant"
				accounts[testAccount1] = mismatch
				return postingParams(testPosting1, postingEntries(testPosting1, 100, 100), accounts)
			}(),
			expectedError: entity.Errorf("ACCOUNT_SCOPE_MISMATCH", "account %s is outside the posting scope", testAccount1),
		},
		{
			name: "account ledger mismatch",
			p: func() aggregate.PostingParams {
				accounts := postingAccounts()
				mismatch := accounts[testAccount1]
				mismatch.LedgerID = "other-ledger"
				accounts[testAccount1] = mismatch
				return postingParams(testPosting1, postingEntries(testPosting1, 100, 100), accounts)
			}(),
			expectedError: entity.Errorf("ACCOUNT_SCOPE_MISMATCH", "account %s is outside the posting scope", testAccount1),
		},
		{
			name: "account asset mismatch",
			p: func() aggregate.PostingParams {
				accounts := postingAccounts()
				mismatch := accounts[testAccount1]
				mismatch.AssetCode = "EUR"
				accounts[testAccount1] = mismatch
				return postingParams(testPosting1, postingEntries(testPosting1, 100, 100), accounts)
			}(),
			expectedError: entity.Errorf("ACCOUNT_ASSET_MISMATCH", "entry asset %s does not match account %s asset", testUSD, testAccount1),
		},
		{
			name: "entry posting id mismatch",
			p: func() aggregate.PostingParams {
				p := baseParams
				p.Entries = []entity.Entry{
					{ID: "e-1", PostingID: "other-post", AccountID: testAccount1, Side: valueobject.DirectionDebit, AmountMinor: 250, AssetCode: testUSD, AccountSeq: 1},
					{ID: "e-2", PostingID: testPosting1, AccountID: testAccount2, Side: valueobject.DirectionCredit, AmountMinor: 250, AssetCode: testUSD, AccountSeq: 1},
				}
				return p
			}(),
			expectedError: entity.NewError("ENTRY_POSTING_MISMATCH", "entry posting id must match the posting"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res, err := aggregate.ConstructPosting(tc.p)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, len(tc.p.Entries), len(res.Record().Entries))
				assert.Equal(t, 1, len(res.UncommittedEvents()))
				assert.Equal(t, "transaction.posted.v1", res.UncommittedEvents()[0].EventType())
				res.ClearEvents()
				assert.Empty(t, res.UncommittedEvents())
			}
		})
	}
}

func TestReversePosting(t *testing.T) {
	t.Parallel()

	accounts := postingAccounts()
	origPosting, err := aggregate.ConstructPosting(postingParams(testPosting1, postingEntries(testPosting1, 100, 100), accounts))
	assert.NoError(t, err)

	at := time.Date(2026, 9, 14, 8, 0, 0, 0, time.UTC)
	baseReverseParams := aggregate.ReverseParams{
		NewID:   testPosting2,
		Reason:  "entry error",
		Actor:   testUser1,
		EventID: testEvent2,
		At:      at,
		IDGen:   &stubIDs{},
	}

	type testCase struct {
		name          string
		original      aggregate.Posting
		accounts      map[valueobject.AccountID]entity.AccountData
		p             aggregate.ReverseParams
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "successful reversal",
			original:      origPosting,
			accounts:      accounts,
			p:             baseReverseParams,
			expectedError: nil,
		},
		{
			name:     "missing reason fails",
			original: origPosting,
			accounts: accounts,
			p: func() aggregate.ReverseParams {
				rp := baseReverseParams
				rp.Reason = ""
				return rp
			}(),
			expectedError: entity.NewError("REVERSAL_REASON_REQUIRED", "reversal requires a reason"),
		},
		{
			name:     "nil id generator fails closed",
			original: origPosting,
			accounts: accounts,
			p: func() aggregate.ReverseParams {
				rp := baseReverseParams
				rp.IDGen = nil
				return rp
			}(),
			expectedError: entity.NewError("ID_GENERATOR_REQUIRED", "reversal requires an ID generator for mirrored entries"),
		},
		{
			name:     "reversal with frozen account fails",
			original: origPosting,
			accounts: func() map[valueobject.AccountID]entity.AccountData {
				m := postingAccounts()
				f := m[testAccount1]
				f.Status = valueobject.StatusFrozen
				m[testAccount1] = f
				return m
			}(),
			p:             baseReverseParams,
			expectedError: entity.Errorf("ACCOUNT_FROZEN", "account %s is frozen", testAccount1),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rev, err := aggregate.ReversePosting(tc.original, tc.accounts, tc.p)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				rec := rev.Record()
				assert.NotNil(t, rec.ReversalOf)
				assert.Equal(t, valueobject.PostingID(testPosting1), *rec.ReversalOf)
				assert.Equal(t, valueobject.DirectionCredit, rec.Entries[0].Side)
				assert.Equal(t, valueobject.DirectionDebit, rec.Entries[1].Side)
				assert.Len(t, rev.UncommittedEvents(), 2)
				assert.Equal(t, "transaction.reversed.v1", rev.UncommittedEvents()[1].EventType())
				// Verify original unchanged
				assert.Nil(t, tc.original.Record().ReversalOf)
				assert.Len(t, tc.original.UncommittedEvents(), 1)
			}
		})
	}
}

func TestPostingImmutable(t *testing.T) {
	t.Parallel()
	typ := reflect.TypeOf(aggregate.Posting{})
	for i := 0; i < typ.NumField(); i++ {
		if typ.Field(i).IsExported() {
			t.Errorf("Posting exposes mutable field %s", typ.Field(i).Name)
		}
	}
	ptr := reflect.PointerTo(typ)
	for i := 0; i < ptr.NumMethod(); i++ {
		n := ptr.Method(i).Name
		if strings.HasPrefix(n, "Set") || strings.HasPrefix(n, "Update") || strings.HasPrefix(n, "Delete") || strings.HasPrefix(n, "Mutate") {
			t.Errorf("Posting has mutator %s", n)
		}
	}
	// Record copies isolate the aggregate.
	p, err := aggregate.ConstructPosting(postingParams(testPosting1, postingEntries(testPosting1, 100, 100), postingAccounts()))
	if err != nil {
		t.Fatalf("ConstructPosting: %v", err)
	}
	rec := p.Record()
	rec.Entries[0].AmountMinor = 1
	if p.Record().Entries[0].AmountMinor != 100 {
		t.Fatal("Record must return a copy")
	}
}

func TestPostingIDAccessor(t *testing.T) {
	t.Parallel()
	p, err := aggregate.ConstructPosting(postingParams(testPosting7, postingEntries(testPosting7, 500, 500), postingAccounts()))
	if err != nil {
		t.Fatalf("ConstructPosting: %v", err)
	}
	if p.ID() != testPosting7 {
		t.Fatalf("ID = %s", p.ID())
	}
}
