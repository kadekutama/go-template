package command_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

const (
	postAcctSrc  = valueobject.AccountID("40000000-0000-4000-8000-000000000001")
	postAcctDst  = valueobject.AccountID("40000000-0000-4000-8000-000000000002")
	postTenant   = valueobject.TenantID("10000000-0000-4000-8000-000000000001")
	postLedger   = valueobject.LedgerID("20000000-0000-4000-8000-000000000001")
	postPosting1 = valueobject.PostingID("50000000-0000-4000-8000-000000000001")
	postCurrency = valueobject.AssetCode("USD")
)

var postAt = time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

func postTestAccounts() map[valueobject.AccountID]entity.AccountData {
	return map[valueobject.AccountID]entity.AccountData{
		postAcctSrc: {ID: postAcctSrc, TenantID: postTenant, LedgerID: postLedger, Number: "4000", Name: "src", Class: valueobject.ClassLiability, AssetCode: postCurrency, Status: valueobject.StatusActive, Version: 1},
		postAcctDst: {ID: postAcctDst, TenantID: postTenant, LedgerID: postLedger, Number: "4001", Name: "dst", Class: valueobject.ClassLiability, AssetCode: postCurrency, Status: valueobject.StatusActive, Version: 1},
	}
}

func postTestCommand() port.PostPostingCommand {
	return port.PostPostingCommand{
		TenantID:          postTenant,
		LedgerID:          postLedger,
		Operation:         "transfer",
		ExternalReference: "ext-1",
		Description:       "slice",
		Entries: []port.NewEntry{
			{AccountID: postAcctSrc, Side: valueobject.DirectionDebit, AmountMinor: 5000, AssetCode: postCurrency},
			{AccountID: postAcctDst, Side: valueobject.DirectionCredit, AmountMinor: 5000, AssetCode: postCurrency},
		},
		IdempotencyKey: "key-1",
		Actor:          "u-1",
	}
}

func newPostingService(t *testing.T, uow port.UnitOfWork, authz port.Authorizer, idCount int) *command.PostingService {
	return command.NewPostingService(command.PostingServiceParams{
		UoW:      uow,
		Accounts: newMockAccountRepository(t, postTestAccounts()),
		Clock:    newMockClock(t, postAt),
		IDs:      newMockIDGenerator(t, postTestIDs(idCount)...),
		Authz:    authz,
	})
}

func TestPostingServiceExecute(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name               string
		cmd                port.PostPostingCommand
		preload            func(svc *command.PostingService, uow port.UnitOfWork)
		denied             bool
		expectedResult     port.PostingResult
		expectedError      error
		expectedCommits    int
		expectedOutbox     int
		expectedAuthzCalls int
	}

	testCases := []testCase{
		{
			name:    "happy path commits once with cursor",
			cmd:     postTestCommand(),
			preload: func(_ *command.PostingService, _ port.UnitOfWork) {},
			denied:  false,
			expectedResult: port.PostingResult{
				PostingID: postPosting1,
				TenantID:  postTenant,
				LedgerID:  postLedger,
				Cursor:    "cursor-7",
			},
			expectedError:      nil,
			expectedCommits:    1,
			expectedOutbox:     1,
			expectedAuthzCalls: 1,
		},
		{
			name: "corrupt idempotency replay returns IDEMPOTENCY_RECORD_INVALID",
			cmd:  postTestCommand(),
			preload: func(_ *command.PostingService, uow port.UnitOfWork) {
				cmd := postTestCommand()
				parts := []string{cmd.IdempotencyKey, string(cmd.TenantID), string(cmd.LedgerID), cmd.Operation, cmd.ExternalReference}
				for _, line := range cmd.Entries {
					parts = append(parts, string(line.AccountID), string(line.Side), fmt.Sprintf("%d", line.AmountMinor), string(line.AssetCode))
				}
				setUOWIdem(uow, cmd.IdempotencyKey, command.Fingerprint(parts...), []byte("{corrupt-json"))
			},
			expectedResult:     port.PostingResult{},
			expectedError:      entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt"),
			expectedCommits:    0,
			expectedOutbox:     0,
			expectedAuthzCalls: 1,
		},
		{
			name: "unknown account fails without commit",
			cmd: func() port.PostPostingCommand {
				c := postTestCommand()
				c.Entries[0].AccountID = "a-ghost"
				return c
			}(),
			preload:            func(_ *command.PostingService, _ port.UnitOfWork) {},
			denied:             false,
			expectedResult:     port.PostingResult{},
			expectedError:      entity.NewError("ACCOUNT_NOT_FOUND", "account a-ghost is unknown"),
			expectedCommits:    0,
			expectedOutbox:     0,
			expectedAuthzCalls: 1,
		},
		{
			name: "unbalanced lot fails without commit",
			cmd: func() port.PostPostingCommand {
				c := postTestCommand()
				c.Entries[1].AmountMinor = 4000
				return c
			}(),
			preload:            func(_ *command.PostingService, _ port.UnitOfWork) {},
			denied:             false,
			expectedResult:     port.PostingResult{},
			expectedError:      &entity.Error{Code: "UNBALANCED_TRANSACTION", Message: "asset USD unbalanced: debits=5000 credits=4000"},
			expectedCommits:    0,
			expectedOutbox:     0,
			expectedAuthzCalls: 1,
		},
		{
			name: "duplicate replay returns original with one commit",
			cmd:  postTestCommand(),
			preload: func(svc *command.PostingService, _ port.UnitOfWork) {
				_, err := svc.Execute(context.Background(), postTestCommand())
				require.NoError(t, err)
			},
			denied: false,
			expectedResult: port.PostingResult{
				PostingID: postPosting1,
				TenantID:  postTenant,
				LedgerID:  postLedger,
				Cursor:    "cursor-7",
			},
			expectedError:      nil,
			expectedCommits:    1,
			expectedOutbox:     1,
			expectedAuthzCalls: 2,
		},
		{
			name: "fingerprint conflict fails without second commit",
			cmd: func() port.PostPostingCommand {
				c := postTestCommand()
				c.Entries[1].AmountMinor = 4000
				c.Description = "changed"
				return c
			}(),
			preload: func(svc *command.PostingService, _ port.UnitOfWork) {
				_, err := svc.Execute(context.Background(), postTestCommand())
				require.NoError(t, err)
			},
			denied:             false,
			expectedResult:     port.PostingResult{},
			expectedError:      entity.NewError("IDEMPOTENCY_CONFLICT", "idempotency key leased for a different request"),
			expectedCommits:    1,
			expectedOutbox:     1,
			expectedAuthzCalls: 2,
		},
		{
			name:               "denied subject fails before any store touch",
			cmd:                postTestCommand(),
			preload:            func(_ *command.PostingService, _ port.UnitOfWork) {},
			denied:             true,
			expectedResult:     port.PostingResult{},
			expectedError:      entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
			expectedCommits:    0,
			expectedOutbox:     0,
			expectedAuthzCalls: 1,
		},
		{
			name: "missing idempotency key fails validation",
			cmd: func() port.PostPostingCommand {
				c := postTestCommand()
				c.IdempotencyKey = ""
				return c
			}(),
			preload:            func(_ *command.PostingService, _ port.UnitOfWork) {},
			denied:             false,
			expectedResult:     port.PostingResult{},
			expectedError:      entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "posting requires an idempotency key"),
			expectedCommits:    0,
			expectedOutbox:     0,
			expectedAuthzCalls: 0,
		},
		{
			name: "missing actor fails validation",
			cmd: func() port.PostPostingCommand {
				c := postTestCommand()
				c.Actor = "  "
				return c
			}(),
			preload:            func(_ *command.PostingService, _ port.UnitOfWork) {},
			denied:             false,
			expectedResult:     port.PostingResult{},
			expectedError:      entity.NewError("ACTOR_REQUIRED", "posting actor is required"),
			expectedCommits:    0,
			expectedOutbox:     0,
			expectedAuthzCalls: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := newMockUOW(t, "cursor-7")
			authz := newMockAuthorizer(t)
			if tc.denied {
				setAuthzDenied(authz, "u-1|ledger.post|ledger/"+string(postLedger))
			}
			svc := newPostingService(t, uow, authz, 20)
			tc.preload(svc, uow)
			actualResult, err := svc.Execute(context.Background(), tc.cmd)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, tc.expectedCommits, uowCommitCount(uow))
			assert.Equal(t, tc.expectedOutbox, uowOutboxLen(uow))
			assert.Equal(t, tc.expectedAuthzCalls, authzCalls(authz))
		})
	}
}

func TestPostingServiceUnknownOutcome(t *testing.T) {
	t.Parallel()

	uow := newMockUOW(t, "cursor-7")
	setUOWFailAfterCommit(uow, true)
	authz := newMockAuthorizer(t)
	svc := newPostingService(t, uow, authz, 20)

	actualResult, err := svc.Execute(context.Background(), postTestCommand())
	assert.Equal(t, port.PostingResult{}, actualResult)
	assert.Equal(t, entity.NewError("COMMIT_AMBIGUOUS", "commit outcome unknown"), err)
	assert.Equal(t, 1, uowCommitCount(uow))

	setUOWFailAfterCommit(uow, false)
	actualResult, err = svc.Execute(context.Background(), postTestCommand())
	assert.NoError(t, err)
	assert.Equal(t, port.PostingResult{
		PostingID: postPosting1,
		TenantID:  postTenant,
		LedgerID:  postLedger,
		Cursor:    "cursor-7",
	}, actualResult)
	assert.Equal(t, 1, uowCommitCount(uow))
}

func TestPostingServiceConcurrent(t *testing.T) {
	t.Parallel()

	uow := newMockUOW(t, "cursor-7")
	authz := newMockAuthorizer(t)
	svc := newPostingService(t, uow, authz, 100)

	const callers = 8
	results := make([]port.PostingResult, callers)
	errs := make([]error, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index], errs[index] = svc.Execute(context.Background(), postTestCommand())
		}(i)
	}
	wg.Wait()

	for i := 0; i < callers; i++ {
		assert.NoError(t, errs[i])
		assert.Equal(t, results[0], results[i])
	}
	assert.NotEmpty(t, results[0].PostingID)
	assert.Equal(t, 1, uowCommitCount(uow))
	assert.Equal(t, 1, uowOutboxLen(uow))
}
