package port_test

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/repository"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

var (
	_ port.PostLedgerPosting = (*fakePoster)(nil)
	_ port.GetPosting        = (*fakePostingReader)(nil)
	_ port.GetBalance        = (*fakeBalanceReader)(nil)
	_ port.ListEntries       = (*fakeEntryLister)(nil)
	_ port.UnitOfWork        = (*fakeUnitOfWork)(nil)
	_ port.IdempotencyStore  = (*fakeIdempotencyStore)(nil)
	_ port.EventOutbox       = (*fakeOutbox)(nil)
	_ port.EventPublisher    = (*fakePublisher)(nil)
	_ port.Clock             = (*fakeClock)(nil)
	_ port.IDGenerator       = (*fakeIDs)(nil)
	_ port.Authorizer        = (*fakeAuthorizer)(nil)
)

type fakePoster struct{}

func (*fakePoster) Execute(_ context.Context, _ port.PostPostingCommand) (port.PostingResult, error) {
	return port.PostingResult{}, nil
}

type fakePostingReader struct{}

func (*fakePostingReader) Execute(_ context.Context, _ port.GetPostingQuery) (port.PostingView, error) {
	return port.PostingView{}, nil
}

type fakeBalanceReader struct{}

func (*fakeBalanceReader) Execute(_ context.Context, _ port.BalanceQuery) (port.BalanceView, error) {
	return port.BalanceView{}, nil
}

type fakeEntryLister struct{}

func (*fakeEntryLister) Execute(_ context.Context, _ port.EntriesQuery) (port.EntriesPage, error) {
	return port.EntriesPage{}, nil
}

type fakeClock struct{}

func (*fakeClock) Now() time.Time { return time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC) }

// fakePostingRepo buffers committed postings in memory.
type fakePostingRepo struct {
	staged    map[valueobject.PostingID]entity.PostingData
	committed map[valueobject.PostingID]entity.PostingData
}

func (f *fakePostingRepo) Commit(_ context.Context, posting entity.PostingData) error {
	if f.staged == nil {
		f.staged = map[valueobject.PostingID]entity.PostingData{}
	}
	f.staged[posting.ID] = posting
	return nil
}

func (f *fakePostingRepo) FindByID(_ context.Context, _ valueobject.TenantID, id valueobject.PostingID) (entity.PostingData, error) {
	posting, ok := f.committed[id]
	if !ok {
		return entity.PostingData{}, entity.NewError("POSTING_NOT_FOUND", "posting is unknown")
	}
	return posting, nil
}

func (f *fakePostingRepo) FindByExternalReference(_ context.Context, _ valueobject.TenantID, _ string) (entity.PostingData, error) {
	return entity.PostingData{}, entity.NewError("POSTING_NOT_FOUND", "posting is unknown")
}

func (f *fakePostingRepo) FindByAccount(_ context.Context, _ valueobject.TenantID, _ valueobject.AccountID, _ string, _ int) ([]entity.PostingData, string, error) {
	return nil, "", nil
}

// fakeHoldRepo buffers holds in memory.
type fakeHoldRepo struct {
	staged map[valueobject.HoldID]entity.HoldData
}

func (f *fakeHoldRepo) Create(_ context.Context, hold entity.HoldData) error {
	if f.staged == nil {
		f.staged = map[valueobject.HoldID]entity.HoldData{}
	}
	f.staged[hold.ID] = hold
	return nil
}

func (f *fakeHoldRepo) FindByID(_ context.Context, _ valueobject.TenantID, _ valueobject.HoldID) (entity.HoldData, error) {
	return entity.HoldData{}, entity.NewError("HOLD_NOT_FOUND", "hold is unknown")
}

func (f *fakeHoldRepo) FindActiveByAccount(_ context.Context, _ valueobject.TenantID, _ valueobject.AccountID) ([]entity.HoldData, error) {
	return nil, nil
}

func (f *fakeHoldRepo) Update(_ context.Context, _ entity.HoldData, _ int64) error {
	return nil
}

// fakeTx exposes tx-scoped stores backed by per-transaction buffers.
type fakeTx struct {
	postings *fakePostingRepo
	holds    *fakeHoldRepo
	idem     *fakeIdempotencyStore
	outbox   *fakeOutbox
	cursor   string
}

func (t *fakeTx) Postings() repository.PostingRepository { return t.postings }
func (t *fakeTx) Holds() repository.HoldRepository       { return t.holds }
func (t *fakeTx) Idempotency() port.IdempotencyStore     { return t.idem }
func (t *fakeTx) Outbox() port.EventOutbox               { return t.outbox }
func (t *fakeTx) Cursor() string                         { return t.cursor }

// fakeUnitOfWork merges staged buffers into committed state only when fn
// returns nil, modeling the atomicity contract adapters must honor.
type fakeUnitOfWork struct {
	postings map[valueobject.PostingID]entity.PostingData
	outbox   []port.OutboxFact
}

func (u *fakeUnitOfWork) Do(ctx context.Context, fn func(ctx context.Context, tx port.Tx) error) error {
	tx := &fakeTx{
		postings: &fakePostingRepo{},
		holds:    &fakeHoldRepo{},
		idem:     &fakeIdempotencyStore{},
		outbox:   &fakeOutbox{},
		cursor:   "cursor-1",
	}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	if u.postings == nil {
		u.postings = map[valueobject.PostingID]entity.PostingData{}
	}
	for id, posting := range tx.postings.staged {
		u.postings[id] = posting
	}
	u.outbox = append(u.outbox, tx.outbox.staged...)
	return nil
}

// fakeIdempotencyStore is a durable-result fake: completed keys replay on the
// same fingerprint and conflict on a different one.
type fakeIdempotencyStore struct {
	entries map[string]fakeIdemEntry
}

type fakeIdemEntry struct {
	fingerprint string
	response    []byte
	completed   bool
}

func (s *fakeIdempotencyStore) Reserve(_ context.Context, rec port.IdempotencyRecord) (port.ReserveOutcome, error) {
	if s.entries == nil {
		s.entries = map[string]fakeIdemEntry{}
	}
	entry, ok := s.entries[rec.Key]
	if !ok {
		s.entries[rec.Key] = fakeIdemEntry{fingerprint: rec.Fingerprint}
		return port.ReserveOutcome{}, nil
	}
	if entry.fingerprint != rec.Fingerprint {
		return port.ReserveOutcome{}, entity.NewError("IDEMPOTENCY_CONFLICT", "idempotency key leased for a different request")
	}
	if entry.completed {
		return port.ReserveOutcome{Replay: true, Response: entry.response}, nil
	}
	return port.ReserveOutcome{}, nil
}

func (s *fakeIdempotencyStore) Complete(_ context.Context, key string, response []byte) error {
	entry := s.entries[key]
	entry.response = response
	entry.completed = true
	s.entries[key] = entry
	return nil
}

type fakeOutbox struct {
	staged []port.OutboxFact
}

func (o *fakeOutbox) Append(_ context.Context, facts ...port.OutboxFact) error {
	o.staged = append(o.staged, facts...)
	return nil
}

type fakePublisher struct {
	published []port.OutboxFact
}

func (p *fakePublisher) Publish(_ context.Context, facts ...port.OutboxFact) error {
	p.published = append(p.published, facts...)
	return nil
}

type fakeIDs struct {
	next []string
}

func (f *fakeIDs) NewID() string {
	id := f.next[0]
	f.next = f.next[1:]
	return id
}

type fakeAuthorizer struct {
	denied map[string]bool
}

func (a *fakeAuthorizer) Authorize(_ context.Context, subject port.Subject, action, resource string) error {
	if a.denied[subject.ID+"|"+action+"|"+resource] {
		return entity.NewError("FORBIDDEN", "subject is not authorized for this action")
	}
	return nil
}

func TestUnitOfWorkAtomicity(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                string
		callbackError       error
		expectedPostings    int
		expectedOutboxFacts int
	}

	testCases := []testCase{
		{
			name:                "commit makes writes visible",
			callbackError:       nil,
			expectedPostings:    1,
			expectedOutboxFacts: 1,
		},
		{
			name:                "mid-transaction failure persists nothing",
			callbackError:       errors.New("boom"),
			expectedPostings:    0,
			expectedOutboxFacts: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := &fakeUnitOfWork{}
			err := uow.Do(context.Background(), func(_ context.Context, tx port.Tx) error {
				commitErr := tx.Postings().Commit(context.Background(), entity.PostingData{ID: "p-1"})
				assert.NoError(t, commitErr)
				outboxErr := tx.Outbox().Append(context.Background(), port.OutboxFact{EventType: "transfer.completed.v1"})
				assert.NoError(t, outboxErr)
				return tc.callbackError
			})
			assert.Equal(t, tc.callbackError, err)
			assert.Len(t, uow.postings, tc.expectedPostings)
			assert.Len(t, uow.outbox, tc.expectedOutboxFacts)
		})
	}
}

func TestIdempotencyReserve(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name               string
		preload            func(store *fakeIdempotencyStore)
		key                string
		fingerprint        string
		expectedOutcome    port.ReserveOutcome
		expectedError      error
		expectedExecutions int
	}

	testCases := []testCase{
		{
			name: "unknown key leases without replay",
			preload: func(_ *fakeIdempotencyStore) {
			},
			key:         "k-1",
			fingerprint: "fp-a",
			expectedOutcome: port.ReserveOutcome{
				Replay:   false,
				Response: nil,
			},
			expectedError:      nil,
			expectedExecutions: 1,
		},
		{
			name: "same fingerprint replays stored response",
			preload: func(store *fakeIdempotencyStore) {
				reserveOutcome, reserveErr := store.Reserve(context.Background(), port.IdempotencyRecord{Key: "k-1", Fingerprint: "fp-a"})
				assert.Equal(t, port.ReserveOutcome{}, reserveOutcome)
				assert.NoError(t, reserveErr)
				assert.NoError(t, store.Complete(context.Background(), "k-1", []byte(`{"ok":true}`)))
			},
			key:         "k-1",
			fingerprint: "fp-a",
			expectedOutcome: port.ReserveOutcome{
				Replay:   true,
				Response: []byte(`{"ok":true}`),
			},
			expectedError:      nil,
			expectedExecutions: 0,
		},
		{
			name: "different fingerprint conflicts without execution",
			preload: func(store *fakeIdempotencyStore) {
				_, reserveErr := store.Reserve(context.Background(), port.IdempotencyRecord{Key: "k-1", Fingerprint: "fp-a"})
				assert.NoError(t, reserveErr)
			},
			key:                "k-1",
			fingerprint:        "fp-b",
			expectedOutcome:    port.ReserveOutcome{},
			expectedError:      entity.NewError("IDEMPOTENCY_CONFLICT", "idempotency key leased for a different request"),
			expectedExecutions: 0,
		},
		{
			name: "empty key leases without replay",
			preload: func(_ *fakeIdempotencyStore) {
			},
			key:         "",
			fingerprint: "fp-a",
			expectedOutcome: port.ReserveOutcome{
				Replay:   false,
				Response: nil,
			},
			expectedError:      nil,
			expectedExecutions: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeIdempotencyStore{}
			tc.preload(store)
			executions := 0
			outcome, err := store.Reserve(context.Background(), port.IdempotencyRecord{Key: tc.key, Fingerprint: tc.fingerprint})
			assert.Equal(t, tc.expectedOutcome, outcome)
			assert.Equal(t, tc.expectedError, err)
			if err == nil && !outcome.Replay {
				executions++
			}
			assert.Equal(t, tc.expectedExecutions, executions)
		})
	}
}

func TestAuthorizerBoundary(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		subject       port.Subject
		action        string
		resource      string
		expectedError error
	}

	testCases := []testCase{
		{
			name: "allowed subject passes",
			subject: port.Subject{
				ID:       "u-1",
				TenantID: "t-1",
			},
			action:        "ledger.post",
			resource:      "ledger/l-1",
			expectedError: nil,
		},
		{
			name: "denied subject fails before execution",
			subject: port.Subject{
				ID:       "u-2",
				TenantID: "t-1",
			},
			action:        "ledger.post",
			resource:      "ledger/l-1",
			expectedError: entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
		{
			name: "empty subject identity fails closed",
			subject: port.Subject{
				ID:       "",
				TenantID: "t-1",
			},
			action:        "ledger.post",
			resource:      "ledger/l-1",
			expectedError: entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			authz := &fakeAuthorizer{denied: map[string]bool{
				"u-2|ledger.post|ledger/l-1": true,
				"|ledger.post|ledger/l-1":    true,
			}}
			err := authz.Authorize(context.Background(), tc.subject, tc.action, tc.resource)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestPortImportBoundary(t *testing.T) {
	t.Parallel()

	allowedPrefixes := []string{
		`"context"`,
		`"errors"`,
		`"math"`,
		`"strconv"`,
		`"strings"`,
		`"time"`,
		`"github.com/kadekutama/go-template/internal/domain/`,
		`"github.com/kadekutama/go-template/internal/application/port"`,
	}

	entries, err := os.ReadDir(".")
	require.NoError(t, err)
	checked := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		checked++
		path := filepath.Join(".", entry.Name())
		src, readErr := os.ReadFile(filepath.Clean(path)) // #nosec G304 -- test inspects package source files
		require.NoError(t, readErr)
		parsed, parseErr := parser.ParseFile(token.NewFileSet(), path, src, parser.ImportsOnly)
		require.NoError(t, parseErr)
		for _, imp := range parsed.Imports {
			importPath, unquoteErr := strconv.Unquote(imp.Path.Value)
			require.NoError(t, unquoteErr)
			quoted := strconv.Quote(importPath)
			allowed := false
			for _, prefix := range allowedPrefixes {
				if strings.HasPrefix(quoted, strings.TrimSuffix(prefix, `"`)) {
					allowed = true
					break
				}
			}
			assert.True(t, allowed, "port file %s imports %s", entry.Name(), importPath)
		}
	}
	assert.Greater(t, checked, 0)
}
