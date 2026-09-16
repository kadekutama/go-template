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
		transfers      *stubTransfers
		query          port.TransferQuery
		expectedResult port.TransferView
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "find transfer succeeds",
			transfers: &stubTransfers{
				transfer: record,
			},
			query: port.TransferQuery{
				TenantID:   "t-1",
				TransferID: "x-1",
			},
			expectedResult: port.TransferView{
				TransferID: "x-1",
				Status:     "COMPLETED",
				PostingID:  "p-1",
			},
			expectedError: nil,
		},
		{
			name: "transfer not found propagates error",
			transfers: &stubTransfers{
				err: entity.NewError("TRANSFER_NOT_FOUND", "transfer is unknown"),
			},
			query: port.TransferQuery{
				TenantID:   "t-1",
				TransferID: "ghost",
			},
			expectedResult: port.TransferView{},
			expectedError:  entity.NewError("TRANSFER_NOT_FOUND", "transfer is unknown"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := query.NewTransferQueryService(query.TransferQueryServiceParams{Transfers: tc.transfers})
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
		name           string
		transfers      *stubTransfers
		query          port.BatchStatusQuery
		expectedResult port.BatchStatusResult
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "batch status computes counts and items",
			transfers: &stubTransfers{
				batch:      batch,
				batchItems: items,
			},
			query: port.BatchStatusQuery{
				TenantID: "t-1",
				BatchID:  "b-1",
			},
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
			transfers: &stubTransfers{
				err: entity.NewError("BATCH_NOT_FOUND", "batch is unknown"),
			},
			query: port.BatchStatusQuery{
				TenantID: "t-1",
				BatchID:  "b-missing",
			},
			expectedResult: port.BatchStatusResult{},
			expectedError:  entity.NewError("BATCH_NOT_FOUND", "batch is unknown"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := query.NewTransferQueryService(query.TransferQueryServiceParams{Transfers: tc.transfers})
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
			posting := new(mockapplication.MockGetPosting)
			posting.On("Execute", mock.Anything, port.GetPostingQuery{TenantID: valueobject.TenantID("t-1"), PostingID: tc.postingID}).Return(tc.programmed, tc.programmedErr).Once()
			svc := query.NewTransactionQueryService(query.TransactionQueryServiceParams{Posting: posting})
			actualResult, err := svc.GetTransaction(context.Background(), valueobject.TenantID("t-1"), tc.postingID)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
			posting.AssertExpectations(t)
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
			name:      "valid statement gathers account entries in window",
			account:   account,
			tenant:    valueobject.TenantID("t-1"),
			ledger:    valueobject.LedgerID("l-1"),
			accountID: valueobject.AccountID("a-1"),
			from:      from,
			to:        to,
			setupQuery: func(q *mockapplication.MockPostingQuery) {
				q.On("Search", mock.Anything, port.PostingFilter{
					TenantID: valueobject.TenantID("t-1"),
					Cursor:   "",
					Limit:    500,
				}).Return(port.PostingSearchPage{
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
				}, nil).Once()
			},
			expectedCount: 1,
			expectedError: nil,
		},
		{
			name:          "account on different ledger returns LEDGER_MISMATCH",
			account:       account,
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
			name:      "statement scan exceeding max pages returns STATEMENT_TOO_LARGE",
			account:   account,
			tenant:    valueobject.TenantID("t-1"),
			ledger:    valueobject.LedgerID("l-1"),
			accountID: valueobject.AccountID("a-1"),
			from:      from,
			to:        to,
			setupQuery: func(q *mockapplication.MockPostingQuery) {
				for i := 0; i < 20; i++ {
					cursor := ""
					if i > 0 {
						cursor = fmt.Sprintf("cursor-%d", i)
					}
					next := fmt.Sprintf("cursor-%d", i+1)
					q.On("Search", mock.Anything, port.PostingFilter{
						TenantID: valueobject.TenantID("t-1"),
						Cursor:   cursor,
						Limit:    500,
					}).Return(port.PostingSearchPage{
						Postings:   nil,
						NextCursor: next,
					}, nil).Once()
				}
			},
			expectedCount: 0,
			expectedError: entity.NewError("STATEMENT_TOO_LARGE", "statement exceeds the bounded scan"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			postingQuery := new(mockapplication.MockPostingQuery)
			tc.setupQuery(postingQuery)
			acctRepo := &stubAccounts{account: tc.account, err: tc.accountErr}

			svc := query.NewTransactionQueryService(query.TransactionQueryServiceParams{
				Posting:  new(mockapplication.MockGetPosting),
				Query:    postingQuery,
				Accounts: acctRepo,
			})

			stmt, err := svc.GetStatement(context.Background(), tc.tenant, tc.ledger, tc.accountID, tc.from, tc.to)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.expectedCount, len(stmt.Entries))
				assert.Equal(t, tc.account.ID, stmt.Account.ID)
			}
			postingQuery.AssertExpectations(t)
		})
	}
}
