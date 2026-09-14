package entity_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestEntryValidate(t *testing.T) {
	t.Parallel()

	valid := entity.Entry{
		ID:          "e-1",
		PostingID:   testPosting1,
		AccountID:   testAccount1,
		Side:        valueobject.DirectionDebit,
		AmountMinor: 100,
		AssetCode:   testUSD,
		AccountSeq:  1,
	}

	type testCase struct {
		name          string
		entry         entity.Entry
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid entry",
			entry:         valid,
			expectedError: nil,
		},
		{
			name: "missing entry id",
			entry: func() entity.Entry {
				e := valid
				e.ID = ""
				return e
			}(),
			expectedError: entity.NewError("ENTRY_ID_REQUIRED", "entry id is required"),
		},
		{
			name: "missing posting id",
			entry: func() entity.Entry {
				e := valid
				e.PostingID = ""
				return e
			}(),
			expectedError: entity.NewError("ENTRY_POSTING_REQUIRED", "posting id is required"),
		},
		{
			name: "missing account id",
			entry: func() entity.Entry {
				e := valid
				e.AccountID = ""
				return e
			}(),
			expectedError: entity.NewError("ENTRY_ACCOUNT_REQUIRED", "account id is required"),
		},
		{
			name: "invalid side direction",
			entry: func() entity.Entry {
				e := valid
				e.Side = "SIDEWAYS"
				return e
			}(),
			expectedError: entity.NewError("ENTRY_SIDE_INVALID", "side must be DEBIT or CREDIT"),
		},
		{
			name: "zero amount",
			entry: func() entity.Entry {
				e := valid
				e.AmountMinor = 0
				return e
			}(),
			expectedError: entity.NewError("INVALID_ENTRY_AMOUNT", "entry amount must be a positive minor-unit quantity"),
		},
		{
			name: "negative amount",
			entry: func() entity.Entry {
				e := valid
				e.AmountMinor = -5
				return e
			}(),
			expectedError: entity.NewError("INVALID_ENTRY_AMOUNT", "entry amount must be a positive minor-unit quantity"),
		},
		{
			name: "missing asset code",
			entry: func() entity.Entry {
				e := valid
				e.AssetCode = ""
				return e
			}(),
			expectedError: entity.NewError("ENTRY_ASSET_REQUIRED", "asset code is required"),
		},
		{
			name: "zero sequence",
			entry: func() entity.Entry {
				e := valid
				e.AccountSeq = 0
				return e
			}(),
			expectedError: entity.NewError("ENTRY_SEQUENCE_REQUIRED", "account sequence must be at least 1"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestPostingDataValidate(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 14, 7, 0, 0, 0, time.UTC)
	valid := entity.PostingData{
		ID:        testPosting1,
		TenantID:  testTenantID,
		LedgerID:  testLedgerID,
		Operation: "transfer.v1",
		Entries: []entity.Entry{
			{ID: "e-1", PostingID: testPosting1, AccountID: testAccount1, Side: valueobject.DirectionDebit, AmountMinor: 100, AssetCode: testUSD, AccountSeq: 1},
			{ID: "e-2", PostingID: testPosting1, AccountID: testAccount2, Side: valueobject.DirectionCredit, AmountMinor: 100, AssetCode: testUSD, AccountSeq: 1},
		},
		EffectiveAt: at,
		RecordedAt:  at,
	}

	type testCase struct {
		name          string
		posting       entity.PostingData
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid posting data",
			posting:       valid,
			expectedError: nil,
		},
		{
			name: "valid reversal with reason",
			posting: func() entity.PostingData {
				p := valid
				rev := valueobject.PostingID("p-0")
				p.ReversalOf = &rev
				p.Reason = "fix"
				return p
			}(),
			expectedError: nil,
		},
		{
			name: "missing id",
			posting: func() entity.PostingData {
				p := valid
				p.ID = ""
				return p
			}(),
			expectedError: entity.NewError("POSTING_ID_REQUIRED", "posting id is required"),
		},
		{
			name: "missing tenant id",
			posting: func() entity.PostingData {
				p := valid
				p.TenantID = ""
				return p
			}(),
			expectedError: entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
		{
			name: "missing ledger id",
			posting: func() entity.PostingData {
				p := valid
				p.LedgerID = ""
				return p
			}(),
			expectedError: entity.NewError("LEDGER_REQUIRED", "ledger id is required"),
		},
		{
			name: "missing operation",
			posting: func() entity.PostingData {
				p := valid
				p.Operation = ""
				return p
			}(),
			expectedError: entity.NewError("POSTING_OPERATION_REQUIRED", "operation template name is required"),
		},
		{
			name: "insufficient entries (less than 2)",
			posting: func() entity.PostingData {
				p := valid
				p.Entries = p.Entries[:1]
				return p
			}(),
			expectedError: entity.NewError("POSTING_ENTRIES_REQUIRED", "posting requires at least two entries"),
		},
		{
			name: "invalid entry zero amount",
			posting: func() entity.PostingData {
				p := valid
				p.Entries = []entity.Entry{
					{ID: "e-1", PostingID: testPosting1, AccountID: testAccount1, Side: valueobject.DirectionDebit, AmountMinor: 0, AssetCode: testUSD, AccountSeq: 1},
					{ID: "e-2", PostingID: testPosting1, AccountID: testAccount2, Side: valueobject.DirectionCredit, AmountMinor: 100, AssetCode: testUSD, AccountSeq: 1},
				}
				return p
			}(),
			expectedError: entity.NewError("INVALID_ENTRY_AMOUNT", "entry amount must be a positive minor-unit quantity"),
		},
		{
			name: "zero effective time",
			posting: func() entity.PostingData {
				p := valid
				p.EffectiveAt = time.Time{}
				return p
			}(),
			expectedError: entity.NewError("POSTING_EFFECTIVE_REQUIRED", "effective_at is required"),
		},
		{
			name: "zero recorded time",
			posting: func() entity.PostingData {
				p := valid
				p.RecordedAt = time.Time{}
				return p
			}(),
			expectedError: entity.NewError("POSTING_RECORDED_REQUIRED", "recorded_at is server-assigned and required"),
		},
		{
			name: "reversal missing reason",
			posting: func() entity.PostingData {
				p := valid
				rev := valueobject.PostingID("p-0")
				p.ReversalOf = &rev
				p.Reason = ""
				return p
			}(),
			expectedError: entity.NewError("REVERSAL_REASON_REQUIRED", "reversal requires a reason"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.posting.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestHoldDataValidate(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC)
	valid := entity.HoldData{
		ID:          "h-1",
		TenantID:    testTenantID,
		LedgerID:    testLedgerID,
		AccountID:   testAccount1,
		AssetCode:   testUSD,
		AmountMinor: 100,
		Kind:        "AUTHORIZATION",
		State:       entity.HoldActive,
		ExpiresAt:   base.Add(time.Hour),
		Version:     1,
		CreatedAt:   base,
		UpdatedAt:   base,
	}

	type testCase struct {
		name          string
		hold          entity.HoldData
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid active hold",
			hold:          valid,
			expectedError: nil,
		},
		{
			name: "valid captured state",
			hold: func() entity.HoldData {
				h := valid
				h.State = entity.HoldCaptured
				return h
			}(),
			expectedError: nil,
		},
		{
			name: "valid released state",
			hold: func() entity.HoldData {
				h := valid
				h.State = entity.HoldReleased
				return h
			}(),
			expectedError: nil,
		},
		{
			name: "valid expired state",
			hold: func() entity.HoldData {
				h := valid
				h.State = entity.HoldExpired
				return h
			}(),
			expectedError: nil,
		},
		{
			name: "missing id",
			hold: func() entity.HoldData {
				h := valid
				h.ID = ""
				return h
			}(),
			expectedError: entity.NewError("HOLD_ID_REQUIRED", "hold id is required"),
		},
		{
			name: "missing tenant",
			hold: func() entity.HoldData {
				h := valid
				h.TenantID = ""
				return h
			}(),
			expectedError: entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
		{
			name: "missing ledger",
			hold: func() entity.HoldData {
				h := valid
				h.LedgerID = ""
				return h
			}(),
			expectedError: entity.NewError("LEDGER_REQUIRED", "ledger id is required"),
		},
		{
			name: "missing account",
			hold: func() entity.HoldData {
				h := valid
				h.AccountID = ""
				return h
			}(),
			expectedError: entity.NewError("HOLD_ACCOUNT_REQUIRED", "account id is required"),
		},
		{
			name: "missing asset code",
			hold: func() entity.HoldData {
				h := valid
				h.AssetCode = ""
				return h
			}(),
			expectedError: entity.NewError("HOLD_ASSET_REQUIRED", "asset code is required"),
		},
		{
			name: "zero amount",
			hold: func() entity.HoldData {
				h := valid
				h.AmountMinor = 0
				return h
			}(),
			expectedError: entity.NewError("HOLD_AMOUNT_INVALID", "hold amount must be positive"),
		},
		{
			name: "missing kind",
			hold: func() entity.HoldData {
				h := valid
				h.Kind = ""
				return h
			}(),
			expectedError: entity.NewError("HOLD_KIND_REQUIRED", "hold kind is required"),
		},
		{
			name: "invalid state",
			hold: func() entity.HoldData {
				h := valid
				h.State = "MELTED"
				return h
			}(),
			expectedError: entity.NewError("HOLD_STATE_INVALID", "hold state is invalid"),
		},
		{
			name: "zero expiration time",
			hold: func() entity.HoldData {
				h := valid
				h.ExpiresAt = time.Time{}
				return h
			}(),
			expectedError: entity.NewError("HOLD_EXPIRY_REQUIRED", "expiry is required"),
		},
		{
			name: "zero version",
			hold: func() entity.HoldData {
				h := valid
				h.Version = 0
				return h
			}(),
			expectedError: entity.NewError("HOLD_VERSION_INVALID", "version starts at 1"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.hold.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
