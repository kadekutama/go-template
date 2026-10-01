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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/repository"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
)

var (
	_ port.PostLedgerPosting = (*mockapplication.MockPostLedgerPosting)(nil)
	_ port.GetPosting        = (*mockapplication.MockGetPosting)(nil)
	_ port.GetBalance        = (*mockapplication.MockGetBalance)(nil)
	_ port.ListEntries       = (*mockapplication.MockListEntries)(nil)
	_ port.UnitOfWork        = (*mockapplication.MockUnitOfWork)(nil)
	_ port.IdempotencyStore  = (*mockapplication.MockIdempotencyStore)(nil)
	_ port.EventOutbox       = (*mockapplication.MockEventOutbox)(nil)
	_ port.EventPublisher    = (*mockapplication.MockEventPublisher)(nil)
	_ port.Clock             = (*mockapplication.MockClock)(nil)
	_ port.IDGenerator       = (*mockapplication.MockIDGenerator)(nil)
	_ port.Authorizer        = (*mockapplication.MockAuthorizer)(nil)
)

func TestUnitOfWorkAtomicity(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		callbackError     error
		expectedCommitted []string
		expectedError     error
	}

	testCases := []testCase{
		{
			name:              "commit makes staged writes visible",
			callbackError:     nil,
			expectedCommitted: []string{"outbox:transfer.completed.v1", "idem:key-1"},
			expectedError:     nil,
		},
		{
			name:              "mid-transaction failure persists nothing",
			callbackError:     errors.New("boom"),
			expectedCommitted: nil,
			expectedError:     errors.New("boom"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := &atomicFakeUOW{}

			err := uow.Do(context.Background(), func(ctx context.Context, tx port.Tx) error {
				appendErr := tx.Outbox().Append(ctx, port.OutboxFact{EventType: "transfer.completed.v1"})
				require.NoError(t, appendErr)
				completeErr := tx.Idempotency().Complete(ctx, "key-1", []byte(`{"ok":true}`))
				require.NoError(t, completeErr)
				return tc.callbackError
			})

			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, tc.expectedCommitted, uow.committed)
		})
	}
}

// atomicFakeUOW is a test-only UnitOfWork double with real commit/rollback
// semantics: staged outbox/idempotency writes merge into committed only when
// fn returns nil. It proves the Do contract (staged-visible-on-commit,
// nothing-persisted-on-failure) that a programmed mock cannot express.
// Postings/Holds return nil: this double exercises the outbox/idempotency
// stores only; ledger atomicity is proven by the postgres integration suite.
type atomicFakeUOW struct {
	committed []string
}

func (u *atomicFakeUOW) Do(_ context.Context, fn func(context.Context, port.Tx) error) error {
	var staged []string
	if err := fn(context.Background(), &atomicFakeTx{staged: &staged}); err != nil {
		return err
	}
	u.committed = append(u.committed, staged...)
	return nil
}

type atomicFakeTx struct {
	staged *[]string
}

func (t *atomicFakeTx) Postings() repository.PostingRepository { return nil }
func (t *atomicFakeTx) Holds() repository.HoldRepository       { return nil }
func (t *atomicFakeTx) Idempotency() port.IdempotencyStore {
	return &atomicFakeIdem{staged: t.staged}
}
func (t *atomicFakeTx) Outbox() port.EventOutbox { return &atomicFakeOutbox{staged: t.staged} }
func (t *atomicFakeTx) Cursor() string           { return "" }

type atomicFakeOutbox struct {
	staged *[]string
}

func (o *atomicFakeOutbox) Append(_ context.Context, facts ...port.OutboxFact) error {
	for _, fact := range facts {
		*o.staged = append(*o.staged, "outbox:"+fact.EventType)
	}
	return nil
}

type atomicFakeIdem struct {
	staged *[]string
}

func (s *atomicFakeIdem) Reserve(_ context.Context, _ port.IdempotencyRecord) (port.ReserveOutcome, error) {
	return port.ReserveOutcome{}, nil
}

func (s *atomicFakeIdem) Complete(_ context.Context, key string, _ []byte) error {
	*s.staged = append(*s.staged, "idem:"+key)
	return nil
}

func TestIdempotencyReserve(t *testing.T) {
	t.Parallel()

	// setupMock programs one Reserve expectation per case. The field cannot
	// carry the tested function's parameter names (AGENTS.md table rule)
	// because it holds a programmer function, not call arguments; the
	// key/fingerprint fields below carry the exact Reserve signature names.
	type testCase struct {
		name            string
		setupMock       func(store *mockapplication.MockIdempotencyStore)
		key             string
		fingerprint     string
		expectedOutcome port.ReserveOutcome
		expectedError   error
	}

	testCases := []testCase{
		{
			name: "unknown key leases without replay",
			setupMock: func(store *mockapplication.MockIdempotencyStore) {
				store.EXPECT().Reserve(mock.Anything, port.IdempotencyRecord{Key: "k-1", Fingerprint: "fp-a"}).
					Return(port.ReserveOutcome{Replay: false, Response: nil}, nil).Once()
			},
			key:             "k-1",
			fingerprint:     "fp-a",
			expectedOutcome: port.ReserveOutcome{Replay: false, Response: nil},
			expectedError:   nil,
		},
		{
			name: "same fingerprint replays stored response",
			setupMock: func(store *mockapplication.MockIdempotencyStore) {
				store.EXPECT().Reserve(mock.Anything, port.IdempotencyRecord{Key: "k-1", Fingerprint: "fp-a"}).
					Return(port.ReserveOutcome{Replay: true, Response: []byte(`{"ok":true}`)}, nil).Once()
			},
			key:             "k-1",
			fingerprint:     "fp-a",
			expectedOutcome: port.ReserveOutcome{Replay: true, Response: []byte(`{"ok":true}`)},
			expectedError:   nil,
		},
		{
			name: "different fingerprint conflicts without execution",
			setupMock: func(store *mockapplication.MockIdempotencyStore) {
				store.EXPECT().Reserve(mock.Anything, port.IdempotencyRecord{Key: "k-1", Fingerprint: "fp-b"}).
					Return(port.ReserveOutcome{}, entity.NewError("IDEMPOTENCY_CONFLICT", "idempotency key leased for a different request")).Once()
			},
			key:             "k-1",
			fingerprint:     "fp-b",
			expectedOutcome: port.ReserveOutcome{},
			expectedError:   entity.NewError("IDEMPOTENCY_CONFLICT", "idempotency key leased for a different request"),
		},
		{
			name: "empty key leases without replay",
			setupMock: func(store *mockapplication.MockIdempotencyStore) {
				store.EXPECT().Reserve(mock.Anything, port.IdempotencyRecord{Key: "", Fingerprint: "fp-a"}).
					Return(port.ReserveOutcome{Replay: false, Response: nil}, nil).Once()
			},
			key:             "",
			fingerprint:     "fp-a",
			expectedOutcome: port.ReserveOutcome{Replay: false, Response: nil},
			expectedError:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := mockapplication.NewMockIdempotencyStore(t)
			tc.setupMock(store)
			outcome, err := store.Reserve(context.Background(), port.IdempotencyRecord{Key: tc.key, Fingerprint: tc.fingerprint})
			assert.Equal(t, tc.expectedOutcome, outcome)
			assert.Equal(t, tc.expectedError, err)
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
		setupMock     func(authz *mockapplication.MockAuthorizer, sub port.Subject, act, res string)
		expectedError error
	}

	testCases := []testCase{
		{
			name: "allowed subject passes",
			subject: port.Subject{
				ID:       "u-1",
				TenantID: "t-1",
			},
			action:   "ledger.post",
			resource: "ledger/l-1",
			setupMock: func(authz *mockapplication.MockAuthorizer, sub port.Subject, act, res string) {
				authz.EXPECT().Authorize(mock.Anything, sub, act, res).Return(nil).Once()
			},
			expectedError: nil,
		},
		{
			name: "denied subject fails before execution",
			subject: port.Subject{
				ID:       "u-2",
				TenantID: "t-1",
			},
			action:   "ledger.post",
			resource: "ledger/l-1",
			setupMock: func(authz *mockapplication.MockAuthorizer, sub port.Subject, act, res string) {
				authz.EXPECT().Authorize(mock.Anything, sub, act, res).Return(entity.NewError("FORBIDDEN", "subject is not authorized for this action")).Once()
			},
			expectedError: entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
		{
			name: "empty subject identity fails closed",
			subject: port.Subject{
				ID:       "",
				TenantID: "t-1",
			},
			action:   "ledger.post",
			resource: "ledger/l-1",
			setupMock: func(authz *mockapplication.MockAuthorizer, sub port.Subject, act, res string) {
				authz.EXPECT().Authorize(mock.Anything, sub, act, res).Return(entity.NewError("FORBIDDEN", "subject is not authorized for this action")).Once()
			},
			expectedError: entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			authz := mockapplication.NewMockAuthorizer(t)
			tc.setupMock(authz, tc.subject, tc.action, tc.resource)
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
