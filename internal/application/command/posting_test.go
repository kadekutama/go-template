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
	"github.com/kadekutama/go-template/internal/domain/repository"
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

type postAccounts struct {
	accounts map[valueobject.AccountID]entity.AccountData
}

func (f *postAccounts) Create(_ context.Context, _ entity.AccountData) (entity.AccountData, error) {
	return entity.AccountData{}, nil
}

func (f *postAccounts) FindByID(_ context.Context, _ valueobject.TenantID, id valueobject.AccountID) (entity.AccountData, error) {
	account, ok := f.accounts[id]
	if !ok {
		return entity.AccountData{}, entity.NewError("ACCOUNT_NOT_FOUND", "account "+string(id)+" is unknown")
	}
	return account, nil
}

func (f *postAccounts) FindByTenant(_ context.Context, _ valueobject.TenantID, _ string, _ int) ([]entity.AccountData, string, error) {
	return nil, "", nil
}

func (f *postAccounts) UpdateMetadata(_ context.Context, _ entity.AccountData, _ int64) error {
	return nil
}

func (f *postAccounts) UpdateStatus(_ context.Context, _ valueobject.TenantID, _ valueobject.AccountID, _ valueobject.AccountStatus, _ int64) error {
	return nil
}

type postIdemEntry struct {
	fingerprint string
	response    []byte
	completed   bool
}

type postTx struct {
	postings map[valueobject.PostingID]entity.PostingData
	outbox   []port.OutboxFact
	idem     map[string]postIdemEntry
	cursor   string
}

type postUOW struct {
	mu              sync.Mutex
	postings        map[valueobject.PostingID]entity.PostingData
	outbox          []port.OutboxFact
	idem            map[string]postIdemEntry
	commits         int
	failAfterCommit bool
}

func (u *postUOW) Do(ctx context.Context, fn func(ctx context.Context, tx port.Tx) error) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	staged := &postTx{
		postings: map[valueobject.PostingID]entity.PostingData{},
		idem:     map[string]postIdemEntry{},
		cursor:   "cursor-7",
	}
	tx := &postTxAdapter{uow: u, staged: staged}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	for id, posting := range staged.postings {
		if u.postings == nil {
			u.postings = map[valueobject.PostingID]entity.PostingData{}
		}
		u.postings[id] = posting
	}
	u.outbox = append(u.outbox, staged.outbox...)
	for key, entry := range staged.idem {
		if u.idem == nil {
			u.idem = map[string]postIdemEntry{}
		}
		u.idem[key] = entry
	}
	if u.failAfterCommit {
		return entity.NewError("COMMIT_AMBIGUOUS", "commit outcome unknown")
	}
	return nil
}

type postTxAdapter struct {
	uow    *postUOW
	staged *postTx
}

func (t *postTxAdapter) Postings() repository.PostingRepository { return &postTxPostings{tx: t} }
func (t *postTxAdapter) Holds() repository.HoldRepository       { return &postTxHolds{} }
func (t *postTxAdapter) Idempotency() port.IdempotencyStore     { return &postTxIdem{tx: t} }
func (t *postTxAdapter) Outbox() port.EventOutbox               { return &postTxOutbox{tx: t} }
func (t *postTxAdapter) Cursor() string                         { return t.staged.cursor }

type postTxPostings struct {
	tx *postTxAdapter
}

func (p *postTxPostings) Commit(_ context.Context, posting entity.PostingData) (entity.PostingData, error) {
	if posting.ID == "" {
		posting.ID = valueobject.PostingID(fmt.Sprintf("50000000-0000-4000-8000-%012d", len(p.tx.staged.postings)+len(p.tx.uow.postings)+1))
	}
	for i := range posting.Entries {
		if posting.Entries[i].ID == "" {
			posting.Entries[i].ID = valueobject.EntryID(fmt.Sprintf("70000000-0000-4000-8000-%012d", i+1))
		}
		if posting.Entries[i].PostingID == "" {
			posting.Entries[i].PostingID = posting.ID
		}
	}
	if _, dup := p.tx.uow.postings[posting.ID]; dup {
		return entity.PostingData{}, entity.NewError("POSTING_CONFLICT", "posting id already committed")
	}
	if _, dup := p.tx.staged.postings[posting.ID]; dup {
		return entity.PostingData{}, entity.NewError("POSTING_CONFLICT", "posting id already committed")
	}
	p.tx.staged.postings[posting.ID] = posting
	p.tx.uow.commits++
	return posting, nil
}

func (p *postTxPostings) FindByID(_ context.Context, _ valueobject.TenantID, _ valueobject.PostingID) (entity.PostingData, error) {
	return entity.PostingData{}, entity.NewError("POSTING_NOT_FOUND", "posting is unknown")
}

func (p *postTxPostings) FindByExternalReference(_ context.Context, _ valueobject.TenantID, _ string) (entity.PostingData, error) {
	return entity.PostingData{}, entity.NewError("POSTING_NOT_FOUND", "posting is unknown")
}

func (p *postTxPostings) FindByAccount(_ context.Context, _ valueobject.TenantID, _ valueobject.AccountID, _ string, _ int) ([]entity.PostingData, string, error) {
	return nil, "", nil
}

type postTxHolds struct{}

func (*postTxHolds) Create(_ context.Context, _ entity.HoldData) (entity.HoldData, error) {
	return entity.HoldData{}, nil
}

func (*postTxHolds) FindByID(_ context.Context, _ valueobject.TenantID, _ valueobject.HoldID) (entity.HoldData, error) {
	return entity.HoldData{}, entity.NewError("HOLD_NOT_FOUND", "hold is unknown")
}

func (*postTxHolds) FindActiveByAccount(_ context.Context, _ valueobject.TenantID, _ valueobject.AccountID) ([]entity.HoldData, error) {
	return nil, nil
}

func (*postTxHolds) Update(_ context.Context, _ entity.HoldData, _ int64) error { return nil }

type postTxIdem struct {
	tx *postTxAdapter
}

func (s *postTxIdem) Reserve(_ context.Context, rec port.IdempotencyRecord) (port.ReserveOutcome, error) {
	if entry, ok := s.tx.uow.idem[rec.Key]; ok {
		if entry.fingerprint != rec.Fingerprint {
			return port.ReserveOutcome{}, entity.NewError("IDEMPOTENCY_CONFLICT", "idempotency key leased for a different request")
		}
		if entry.completed {
			return port.ReserveOutcome{Replay: true, Response: entry.response}, nil
		}
		return port.ReserveOutcome{}, nil
	}
	if entry, ok := s.tx.staged.idem[rec.Key]; ok {
		if entry.fingerprint != rec.Fingerprint {
			return port.ReserveOutcome{}, entity.NewError("IDEMPOTENCY_CONFLICT", "idempotency key leased for a different request")
		}
		return port.ReserveOutcome{}, nil
	}
	s.tx.staged.idem[rec.Key] = postIdemEntry{fingerprint: rec.Fingerprint}
	return port.ReserveOutcome{}, nil
}

func (s *postTxIdem) Complete(_ context.Context, key string, response []byte) error {
	entry := s.tx.staged.idem[key]
	entry.response = response
	entry.completed = true
	s.tx.staged.idem[key] = entry
	return nil
}

type postTxOutbox struct {
	tx *postTxAdapter
}

func (o *postTxOutbox) Append(_ context.Context, facts ...port.OutboxFact) error {
	o.tx.staged.outbox = append(o.tx.staged.outbox, facts...)
	return nil
}

type postClock struct{}

func (postClock) Now() time.Time { return postAt }

type postIDs struct {
	mu   sync.Mutex
	next []string
}

func (f *postIDs) NewID() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.next[0]
	f.next = f.next[1:]
	return id
}

type postAuthz struct {
	mu     sync.Mutex
	denied map[string]bool
	calls  int
}

func (a *postAuthz) Authorize(_ context.Context, subject port.Subject, action, resource string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	if a.denied[subject.ID+"|"+action+"|"+resource] {
		return entity.NewError("FORBIDDEN", "subject is not authorized for this action")
	}
	return nil
}

func postTestAccounts() map[valueobject.AccountID]entity.AccountData {
	return map[valueobject.AccountID]entity.AccountData{
		postAcctSrc: {ID: postAcctSrc, TenantID: postTenant, LedgerID: postLedger, Number: "4000", Name: "src", Class: valueobject.ClassLiability, AssetCode: postCurrency, Status: valueobject.StatusActive, Version: 1},
		postAcctDst: {ID: postAcctDst, TenantID: postTenant, LedgerID: postLedger, Number: "4001", Name: "dst", Class: valueobject.ClassLiability, AssetCode: postCurrency, Status: valueobject.StatusActive, Version: 1},
	}
}

func postTestIDs(n int) []string {
	ids := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		ids = append(ids, fmt.Sprintf("90000000-0000-4000-8000-%012d", i))
	}
	return ids
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

func newPostingService(uow *postUOW, authz *postAuthz, idCount int) *command.PostingService {
	return command.NewPostingService(command.PostingServiceParams{
		UoW:      uow,
		Accounts: &postAccounts{accounts: postTestAccounts()},
		Clock:    postClock{},
		IDs:      &postIDs{next: postTestIDs(idCount)},
		Authz:    authz,
	})
}

func TestPostingServiceExecute(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name               string
		cmd                port.PostPostingCommand
		preload            func(svc *command.PostingService, uow *postUOW)
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
			preload: func(_ *command.PostingService, _ *postUOW) {},
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
			preload: func(_ *command.PostingService, uow *postUOW) {
				cmd := postTestCommand()
				parts := []string{cmd.IdempotencyKey, string(cmd.TenantID), string(cmd.LedgerID), cmd.Operation, cmd.ExternalReference}
				for _, line := range cmd.Entries {
					parts = append(parts, string(line.AccountID), string(line.Side), fmt.Sprintf("%d", line.AmountMinor), string(line.AssetCode))
				}
				uow.idem = map[string]postIdemEntry{
					cmd.IdempotencyKey: {
						fingerprint: command.Fingerprint(parts...),
						response:    []byte("{corrupt-json"),
						completed:   true,
					},
				}
			},
			denied:             false,
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
			preload:            func(_ *command.PostingService, _ *postUOW) {},
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
			preload:            func(_ *command.PostingService, _ *postUOW) {},
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
			preload: func(svc *command.PostingService, _ *postUOW) {
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
			preload: func(svc *command.PostingService, _ *postUOW) {
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
			preload:            func(_ *command.PostingService, _ *postUOW) {},
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
			preload:            func(_ *command.PostingService, _ *postUOW) {},
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
			preload:            func(_ *command.PostingService, _ *postUOW) {},
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
			uow := &postUOW{}
			authz := &postAuthz{denied: map[string]bool{}}
			if tc.denied {
				authz.denied["u-1|ledger.post|ledger/"+string(postLedger)] = true
			}
			svc := newPostingService(uow, authz, 20)
			tc.preload(svc, uow)
			actualResult, err := svc.Execute(context.Background(), tc.cmd)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, tc.expectedCommits, uow.commits)
			assert.Len(t, uow.outbox, tc.expectedOutbox)
			assert.Equal(t, tc.expectedAuthzCalls, authz.calls)
		})
	}
}

func TestPostingServiceUnknownOutcome(t *testing.T) {
	t.Parallel()

	uow := &postUOW{failAfterCommit: true}
	authz := &postAuthz{denied: map[string]bool{}}
	svc := newPostingService(uow, authz, 20)

	actualResult, err := svc.Execute(context.Background(), postTestCommand())
	assert.Equal(t, port.PostingResult{}, actualResult)
	assert.Equal(t, entity.NewError("COMMIT_AMBIGUOUS", "commit outcome unknown"), err)
	assert.Equal(t, 1, uow.commits)

	uow.failAfterCommit = false
	actualResult, err = svc.Execute(context.Background(), postTestCommand())
	assert.NoError(t, err)
	assert.Equal(t, port.PostingResult{
		PostingID: postPosting1,
		TenantID:  postTenant,
		LedgerID:  postLedger,
		Cursor:    "cursor-7",
	}, actualResult)
	assert.Equal(t, 1, uow.commits)
}

func TestPostingServiceConcurrent(t *testing.T) {
	t.Parallel()

	uow := &postUOW{}
	authz := &postAuthz{denied: map[string]bool{}}
	svc := newPostingService(uow, authz, 100)

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
	assert.Equal(t, 1, uow.commits)
	assert.Len(t, uow.outbox, 1)
}
