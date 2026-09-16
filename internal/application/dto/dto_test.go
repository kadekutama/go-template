package dto_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/application/dto"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestToPostingDTO(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

	type testCase struct {
		name           string
		posting        entity.PostingData
		cursor         string
		expectedResult dto.PostingDTO
	}

	testCases := []testCase{
		{
			name: "posting with entries maps every field",
			posting: entity.PostingData{
				ID: "p-1", TenantID: "t-1", LedgerID: "l-1", Operation: "transfer",
				ExternalReference: "ext-1", Description: "slice",
				Entries: []entity.Entry{
					{ID: "e-1", PostingID: "p-1", AccountID: "a-src", Side: valueobject.DirectionDebit, AmountMinor: 5000, AssetCode: "USD", AccountSeq: 1},
					{ID: "e-2", PostingID: "p-1", AccountID: "a-dst", Side: valueobject.DirectionCredit, AmountMinor: 5000, AssetCode: "USD", AccountSeq: 1},
				},
				EffectiveAt: at, RecordedAt: at,
			},
			cursor: "cursor-7",
			expectedResult: dto.PostingDTO{
				ID: "p-1", TenantID: "t-1", LedgerID: "l-1", Operation: "transfer",
				Reference: "ext-1", Description: "slice",
				Entries: []dto.EntryDTO{
					{ID: "e-1", PostingID: "p-1", AccountID: "a-src", Side: "DEBIT", AmountMinor: 5000, AssetCode: "USD", AccountSeq: 1},
					{ID: "e-2", PostingID: "p-1", AccountID: "a-dst", Side: "CREDIT", AmountMinor: 5000, AssetCode: "USD", AccountSeq: 1},
				},
				EffectiveAt: at, RecordedAt: at, Cursor: "cursor-7",
			},
		},
		{
			name: "posting without entries maps empty lines",
			posting: entity.PostingData{
				ID: "p-2", TenantID: "t-1", LedgerID: "l-1", Operation: "noop",
				Entries:     nil,
				EffectiveAt: at, RecordedAt: at,
			},
			cursor: "",
			expectedResult: dto.PostingDTO{
				ID: "p-2", TenantID: "t-1", LedgerID: "l-1", Operation: "noop",
				Reference: "", Description: "",
				Entries:     []dto.EntryDTO{},
				EffectiveAt: at, RecordedAt: at, Cursor: "",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedResult, dto.ToPostingDTO(tc.posting, tc.cursor))
		})
	}
}

func TestToBalanceDTO(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

	type testCase struct {
		name           string
		view           port.BalanceView
		expectedResult dto.BalanceDTO
	}

	testCases := []testCase{
		{
			name: "balance view maps minor units plus cursor",
			view: port.BalanceView{
				AccountID: "a-1", AssetCode: "USD", AvailableMinor: 6500, AsOf: at, Cursor: "cursor-9",
			},
			expectedResult: dto.BalanceDTO{
				AccountID: "a-1", AssetCode: "USD", AvailableMinor: 6500, AsOf: at, Cursor: "cursor-9",
			},
		},
		{
			name: "zero balance maps without omissions",
			view: port.BalanceView{
				AccountID: "a-2", AssetCode: "EUR", AvailableMinor: 0, AsOf: at,
			},
			expectedResult: dto.BalanceDTO{
				AccountID: "a-2", AssetCode: "EUR", AvailableMinor: 0, AsOf: at,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedResult, dto.ToBalanceDTO(tc.view))
		})
	}
}
