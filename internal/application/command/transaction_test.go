package command_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

var reverseUUIDs = []string{
	"123e4567-e89b-12d3-a456-426614174000",
	"123e4567-e89b-12d3-a456-426614174001",
	"123e4567-e89b-12d3-a456-426614174002",
	"123e4567-e89b-12d3-a456-426614174003",
	"123e4567-e89b-12d3-a456-426614174004",
	"123e4567-e89b-12d3-a456-426614174005",
	"123e4567-e89b-12d3-a456-426614174006",
	"123e4567-e89b-12d3-a456-426614174007",
	"123e4567-e89b-12d3-a456-426614174008",
	"123e4567-e89b-12d3-a456-426614174009",
}

func reverseOriginal() entity.PostingData {
	return entity.PostingData{
		ID: "123e4567-e89b-12d3-a456-426614174010", TenantID: tfrTenant, LedgerID: tfrLedger,
		Operation: "transfer", ExternalReference: "ext-9", Description: "original",
		Entries: []entity.Entry{
			{ID: "123e4567-e89b-12d3-a456-426614174011", PostingID: "123e4567-e89b-12d3-a456-426614174010", AccountID: tfrSrc, Side: valueobject.DirectionDebit, AmountMinor: 5000, AssetCode: tfrAsset, AccountSeq: 1},
			{ID: "123e4567-e89b-12d3-a456-426614174012", PostingID: "123e4567-e89b-12d3-a456-426614174010", AccountID: tfrDst, Side: valueobject.DirectionCredit, AmountMinor: 5000, AssetCode: tfrAsset, AccountSeq: 1},
		},
		EffectiveAt: tfrAt, RecordedAt: tfrAt,
	}
}

func newTransactionService(t *testing.T, uow port.UnitOfWork, authz port.Authorizer, original entity.PostingData, findErr error) *command.TransactionService {
	posting := command.NewPostingService(command.PostingServiceParams{
		UoW:      uow,
		Accounts: newMockAccountRepository(t, tfrTestAccounts()),
		Clock:    newMockClock(t, tfrAt),
		IDs:      newMockIDGenerator(t, reverseUUIDs...),
		Authz:    authz,
	})
	return command.NewTransactionService(command.TransactionServiceParams{
		Posting:  posting,
		UoW:      uow,
		Postings: newMockPostingRepository(t, original, findErr),
		Accounts: newMockAccountRepository(t, tfrTestAccounts()),
		Clock:    newMockClock(t, tfrAt),
		IDs:      newMockIDGenerator(t, reverseUUIDs...),
		Authz:    authz,
	})
}

func reverseTestRequest() command.ReverseTransactionRequest {
	return command.ReverseTransactionRequest{
		TenantID:       tfrTenant,
		PostingID:      "123e4567-e89b-12d3-a456-426614174010",
		Reason:         "duplicate",
		Actor:          "u-1",
		IdempotencyKey: "key-rev-1",
	}
}

func TestReverseTransaction(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		req            command.ReverseTransactionRequest
		original       entity.PostingData
		findErr        error
		preload        func(svc *command.TransactionService)
		expectedCursor string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "reversal commits linked correction",
			req:            reverseTestRequest(),
			original:       reverseOriginal(),
			findErr:        nil,
			preload:        func(_ *command.TransactionService) {},
			expectedCursor: "cursor-5",
			expectedError:  nil,
		},
		{
			name:     "duplicate replay returns original reversal",
			req:      reverseTestRequest(),
			original: reverseOriginal(),
			findErr:  nil,
			preload: func(svc *command.TransactionService) {
				_, err := svc.ReverseTransaction(context.Background(), reverseTestRequest())
				require.NoError(t, err)
			},
			expectedCursor: "cursor-5",
			expectedError:  nil,
		},
		{
			name: "missing reason fails validation",
			req: func() command.ReverseTransactionRequest {
				r := reverseTestRequest()
				r.Reason = ""
				return r
			}(),
			original:       reverseOriginal(),
			findErr:        nil,
			preload:        func(_ *command.TransactionService) {},
			expectedCursor: "",
			expectedError:  entity.NewError("REVERSAL_REASON_REQUIRED", "reversal requires a reason"),
		},
		{
			name:           "unknown original propagates",
			req:            reverseTestRequest(),
			original:       entity.PostingData{},
			findErr:        entity.NewError("POSTING_NOT_FOUND", "posting is unknown"),
			preload:        func(_ *command.TransactionService) {},
			expectedCursor: "",
			expectedError:  entity.NewError("POSTING_NOT_FOUND", "posting is unknown"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := newMockUOW(t, "cursor-5")
			authz := newMockAuthorizer(t)
			svc := newTransactionService(t, uow, authz, tc.original, tc.findErr)
			tc.preload(svc)
			actualResult, err := svc.ReverseTransaction(context.Background(), tc.req)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, reverseUUIDs[0], string(actualResult.PostingID))
				assert.Equal(t, tc.expectedCursor, actualResult.Cursor)
				facts := outboxFacts(uow)
				assert.Len(t, facts, 1)
				assert.Equal(t, "transaction.reversed.v1", facts[0].EventType)
			} else {
				assert.Equal(t, port.PostingResult{}, actualResult)
			}
		})
	}
}

func TestPostTransactionDelegates(t *testing.T) {
	t.Parallel()

	uow := newMockUOW(t, "cursor-7")
	authz := newMockAuthorizer(t)
	posting := command.NewPostingService(command.PostingServiceParams{
		UoW:      uow,
		Accounts: newMockAccountRepository(t, postTestAccounts()),
		Clock:    newMockClock(t, postAt),
		IDs:      newMockIDGenerator(t, postTestIDs(20)...),
		Authz:    authz,
	})
	svc := command.NewTransactionService(command.TransactionServiceParams{Posting: posting})

	actualResult, err := svc.PostTransaction(context.Background(), postTestCommand())
	assert.NoError(t, err)
	assert.Equal(t, "50000000-0000-4000-8000-000000000001", string(actualResult.PostingID))
}
