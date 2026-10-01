package query_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/application/query"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
	mockcommand "github.com/kadekutama/go-template/test/mock/command"
	mockdomain "github.com/kadekutama/go-template/test/mock/domain"
)

func TestTransferQueryServiceGetTransfer(t *testing.T) {
	t.Parallel()

	record := port.TransferRecord{
		ID:        "x-1",
		TenantID:  "t-1",
		Status:    "COMPLETED",
		PostingID: "p-1",
	}

	type testCase struct {
		name           string
		query          port.TransferQuery
		programmed     port.TransferRecord
		programmedErr  error
		expectedResult port.TransferView
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "find transfer succeeds",
			query: port.TransferQuery{
				TenantID:   "t-1",
				TransferID: "x-1",
			},
			programmed:    record,
			programmedErr: nil,
			expectedResult: port.TransferView{
				TransferID: "x-1",
				Status:     "COMPLETED",
				PostingID:  "p-1",
			},
			expectedError: nil,
		},
		{
			name: "transfer not found propagates error",
			query: port.TransferQuery{
				TenantID:   "t-1",
				TransferID: "ghost",
			},
			programmed:     port.TransferRecord{},
			programmedErr:  entity.NewError("TRANSFER_NOT_FOUND", "transfer is unknown"),
			expectedResult: port.TransferView{},
			expectedError:  entity.NewError("TRANSFER_NOT_FOUND", "transfer is unknown"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			transfers := mockcommand.NewMockTransferStore(t)
			transfers.EXPECT().
				FindTransfer(mock.Anything, tc.query.TenantID, tc.query.TransferID).
				Return(tc.programmed, tc.programmedErr).
				Once()

			svc := query.NewTransferQueryService(query.TransferQueryServiceParams{Transfers: transfers})
			actualResult, err := svc.GetTransfer(context.Background(), tc.query)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestTransferQueryServiceGetBatchStatus(t *testing.T) {
	t.Parallel()

	batch := port.BatchRecord{
		ID:       "b-1",
		TenantID: "t-1",
		State:    "PARTIAL",
	}
	items := []port.BatchItem{
		{BatchID: "b-1", Index: 0, TransferID: "x-1", Status: port.TransferCompleted},
		{BatchID: "b-1", Index: 1, TransferID: "x-2", Status: port.TransferFailed, ErrorCode: "INSUFFICIENT_FUNDS"},
	}

	type testCase struct {
		name               string
		query              port.BatchStatusQuery
		programmedBatch    port.BatchRecord
		programmedBatchErr error
		programmedItems    []port.BatchItem
		programmedItemsErr error
		expectedResult     port.BatchStatusResult
		expectedError      error
	}

	testCases := []testCase{
		{
			name: "batch status computes counts and items",
			query: port.BatchStatusQuery{
				TenantID: "t-1",
				BatchID:  "b-1",
			},
			programmedBatch:    batch,
			programmedBatchErr: nil,
			programmedItems:    items,
			programmedItemsErr: nil,
			expectedResult: port.BatchStatusResult{
				BatchID:   "b-1",
				State:     "PARTIAL",
				Succeeded: 1,
				Failed:    1,
				Items: []port.BatchItemStatus{
					{Index: 0, TransferID: "x-1", Status: port.TransferCompleted, ErrorCode: ""},
					{Index: 1, TransferID: "x-2", Status: port.TransferFailed, ErrorCode: "INSUFFICIENT_FUNDS"},
				},
			},
			expectedError: nil,
		},
		{
			name: "batch not found propagates error",
			query: port.BatchStatusQuery{
				TenantID: "t-1",
				BatchID:  "b-missing",
			},
			programmedBatch:    port.BatchRecord{},
			programmedBatchErr: entity.NewError("BATCH_NOT_FOUND", "batch is unknown"),
			programmedItems:    nil,
			programmedItemsErr: nil,
			expectedResult:     port.BatchStatusResult{},
			expectedError:      entity.NewError("BATCH_NOT_FOUND", "batch is unknown"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			transfers := mockcommand.NewMockTransferStore(t)
			transfers.EXPECT().
				FindBatch(mock.Anything, tc.query.TenantID, tc.query.BatchID).
				Return(tc.programmedBatch, tc.programmedBatchErr).
				Once()
			if tc.programmedBatchErr == nil {
				transfers.EXPECT().
					ListBatchItems(mock.Anything, tc.query.TenantID, tc.query.BatchID).
					Return(tc.programmedItems, tc.programmedItemsErr).
					Once()
			}

			svc := query.NewTransferQueryService(query.TransferQueryServiceParams{Transfers: transfers})
			actualResult, err := svc.GetBatchStatus(context.Background(), tc.query)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestTransactionQueryDelegation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		postingID      valueobject.PostingID
		programmed     port.PostingView
		programmedErr  error
		expectedResult port.PostingView
		expectedError  error
	}

	testCases := []testCase{
		{
			name:      "get transaction delegates to posting read",
			postingID: valueobject.PostingID("p-1"),
			programmed: port.PostingView{
				Posting: entity.PostingData{ID: "p-1"},
			},
			programmedErr: nil,
			expectedResult: port.PostingView{
				Posting: entity.PostingData{ID: "p-1"},
			},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			posting := mockapplication.NewMockGetPosting(t)
			posting.EXPECT().
				Execute(mock.Anything, port.GetPostingQuery{TenantID: valueobject.TenantID("t-1"), PostingID: tc.postingID}).
				Return(tc.programmed, tc.programmedErr).
				Once()

			svc := query.NewTransactionQueryService(query.TransactionQueryServiceParams{Posting: posting})
			actualResult, err := svc.GetTransaction(context.Background(), valueobject.TenantID("t-1"), tc.postingID)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestTransactionQueryGetStatement(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)

	account := entity.AccountData{
		ID:       valueobject.AccountID("a-1"),
		TenantID: valueobject.TenantID("t-1"),
		LedgerID: valueobject.LedgerID("l-1"),
	}

	type testCase struct {
		name          string
		account       entity.AccountData
		accountErr    error
		tenant        valueobject.TenantID
		ledger        valueobject.LedgerID
		accountID     valueobject.AccountID
		from          time.Time
		to            time.Time
		setupQuery    func(q *mockapplication.MockPostingQuery)
		expectedCount int
		expectedError error
	}

	testCases := []testCase{
		{
			name:       "valid statement gathers account entries in window",
			account:    account,
			accountErr: nil,
			tenant:     valueobject.TenantID("t-1"),
			ledger:     valueobject.LedgerID("l-1"),
			accountID:  valueobject.AccountID("a-1"),
			from:       from,
			to:         to,
			setupQuery: func(q *mockapplication.MockPostingQuery) {
				q.EXPECT().
					Search(mock.Anything, port.PostingFilter{
						TenantID: valueobject.TenantID("t-1"),
						Cursor:   "",
						Limit:    500,
					}).
					Return(port.PostingSearchPage{
						Postings: []entity.PostingData{
							{
								ID:         "p-1",
								RecordedAt: from.Add(time.Hour),
								Entries: []entity.Entry{
									{ID: "e-1", AccountID: valueobject.AccountID("a-1"), AmountMinor: 1000},
									{ID: "e-2", AccountID: valueobject.AccountID("a-other"), AmountMinor: 1000},
								},
							},
							{
								ID:         "p-outside",
								RecordedAt: to.Add(time.Hour),
								Entries: []entity.Entry{
									{ID: "e-3", AccountID: valueobject.AccountID("a-1"), AmountMinor: 500},
								},
							},
						},
						NextCursor: "",
					}, nil).
					Once()
			},
			expectedCount: 1,
			expectedError: nil,
		},
		{
			name:          "account on different ledger returns LEDGER_MISMATCH",
			account:       account,
			accountErr:    nil,
			tenant:        valueobject.TenantID("t-1"),
			ledger:        valueobject.LedgerID("l-different"),
			accountID:     valueobject.AccountID("a-1"),
			from:          from,
			to:            to,
			setupQuery:    func(_ *mockapplication.MockPostingQuery) {},
			expectedCount: 0,
			expectedError: entity.NewError("LEDGER_MISMATCH", "account belongs to a different ledger"),
		},
		{
			name:       "statement scan exceeding max pages returns STATEMENT_TOO_LARGE",
			account:    account,
			accountErr: nil,
			tenant:     valueobject.TenantID("t-1"),
			ledger:     valueobject.LedgerID("l-1"),
			accountID:  valueobject.AccountID("a-1"),
			from:       from,
			to:         to,
			setupQuery: func(q *mockapplication.MockPostingQuery) {
				for i := 0; i < 20; i++ {
					cursor := ""
					if i > 0 {
						cursor = fmt.Sprintf("cursor-%d", i)
					}
					next := fmt.Sprintf("cursor-%d", i+1)
					q.EXPECT().
						Search(mock.Anything, port.PostingFilter{
							TenantID: valueobject.TenantID("t-1"),
							Cursor:   cursor,
							Limit:    500,
						}).
						Return(port.PostingSearchPage{
							Postings:   nil,
							NextCursor: next,
						}, nil).
						Once()
				}
			},
			expectedCount: 0,
			expectedError: entity.NewError("STATEMENT_TOO_LARGE", "statement exceeds the bounded scan"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			postingQuery := mockapplication.NewMockPostingQuery(t)
			tc.setupQuery(postingQuery)

			acctRepo := mockdomain.NewMockAccountRepository(t)
			acctRepo.EXPECT().
				FindByID(mock.Anything, tc.tenant, tc.accountID).
				Return(tc.account, tc.accountErr).
				Maybe()

			svc := query.NewTransactionQueryService(query.TransactionQueryServiceParams{
				Posting:  mockapplication.NewMockGetPosting(t),
				Query:    postingQuery,
				Accounts: acctRepo,
			})

			stmt, err := svc.GetStatement(context.Background(), tc.tenant, tc.ledger, tc.accountID, tc.from, tc.to)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.expectedCount, len(stmt.Entries))
				assert.Equal(t, tc.account.ID, stmt.Account.ID)
			}
		})
	}
}
