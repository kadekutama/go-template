package command_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/application/query"
	"github.com/kadekutama/go-template/internal/domain/entity"
)

type createAccountCmd struct {
	name        string
	subject     port.Subject
	key         string
	fingerprint string
}

type createAccountRes struct {
	id string
}

type createAccountQuery struct {
	id string
}

// createAccountHandler is the E06-T01 sample: it composes the shared stages
// (command contract, query contract asserted below, idempotency,
// translation at the caller, authorization) around trivial logic with fakes.
type createAccountHandler struct {
	authz      port.Authorizer
	store      *sampleIdemStore
	executions int
}

var _ command.CommandHandler[createAccountCmd, createAccountRes] = (*createAccountHandler)(nil)

func (h *createAccountHandler) Handle(ctx context.Context, cmd createAccountCmd) (createAccountRes, error) {
	if strings.TrimSpace(cmd.name) == "" {
		return createAccountRes{}, entity.NewError("ACCOUNT_NAME_REQUIRED", "account name is required")
	}
	if err := command.RequireAuthz(ctx, h.authz, cmd.subject, "account.open", "tenant/"+string(cmd.subject.TenantID)); err != nil {
		return createAccountRes{}, err
	}
	response, _, err := command.RunIdempotent(ctx, h.store, port.IdempotencyRecord{
		Key:         cmd.key,
		Fingerprint: cmd.fingerprint,
		TenantID:    cmd.subject.TenantID,
	}, func(context.Context) ([]byte, error) {
		h.executions++
		return []byte(`{"id":"a-1"}`), nil
	})
	if err != nil {
		return createAccountRes{}, err
	}
	var payload struct {
		ID string `json:"id"`
	}
	if unmarshalErr := json.Unmarshal(response, &payload); unmarshalErr != nil {
		return createAccountRes{}, unmarshalErr
	}
	return createAccountRes{id: payload.ID}, nil
}

type createAccountReader struct{}

var _ query.QueryHandler[createAccountQuery, createAccountRes] = (*createAccountReader)(nil)

func (*createAccountReader) Handle(_ context.Context, q createAccountQuery) (createAccountRes, error) {
	return createAccountRes(q), nil
}

type sampleIdemEntry struct {
	fingerprint string
	response    []byte
	completed   bool
}

type sampleIdemStore struct {
	entries map[string]sampleIdemEntry
}

func (s *sampleIdemStore) Reserve(_ context.Context, rec port.IdempotencyRecord) (port.ReserveOutcome, error) {
	if s.entries == nil {
		s.entries = map[string]sampleIdemEntry{}
	}
	entry, ok := s.entries[rec.Key]
	if !ok {
		s.entries[rec.Key] = sampleIdemEntry{fingerprint: rec.Fingerprint}
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

func (s *sampleIdemStore) Complete(_ context.Context, key string, response []byte) error {
	entry := s.entries[key]
	entry.response = response
	entry.completed = true
	s.entries[key] = entry
	return nil
}

type sampleAuthorizer struct {
	denied map[string]bool
	calls  int
}

func (a *sampleAuthorizer) Authorize(_ context.Context, subject port.Subject, action, resource string) error {
	a.calls++
	if a.denied[subject.ID+"|"+action+"|"+resource] {
		return entity.NewError("FORBIDDEN", "subject is not authorized for this action")
	}
	return nil
}

func TestCreateAccountPipeline(t *testing.T) {
	t.Parallel()

	allowedSubject := port.Subject{ID: "u-1", TenantID: "t-1"}
	deniedSubject := port.Subject{ID: "u-9", TenantID: "t-1"}

	type testCase struct {
		name               string
		subject            port.Subject
		key                string
		firstFingerprint   string
		fingerprint        string
		accountName        string
		submits            int
		expectedResult     createAccountRes
		expectedError      error
		expectedExecutions int
		expectedAuthzCalls int
	}

	testCases := []testCase{
		{
			name:               "single submit executes once",
			subject:            allowedSubject,
			key:                "k-1",
			firstFingerprint:   "",
			fingerprint:        "fp-a",
			accountName:        "operating",
			submits:            1,
			expectedResult:     createAccountRes{id: "a-1"},
			expectedError:      nil,
			expectedExecutions: 1,
			expectedAuthzCalls: 1,
		},
		{
			name:               "double submit replays without re-executing",
			subject:            allowedSubject,
			key:                "k-1",
			firstFingerprint:   "",
			fingerprint:        "fp-a",
			accountName:        "operating",
			submits:            2,
			expectedResult:     createAccountRes{id: "a-1"},
			expectedError:      nil,
			expectedExecutions: 1,
			expectedAuthzCalls: 2,
		},
		{
			name:               "same key with different fingerprint conflicts",
			subject:            allowedSubject,
			key:                "k-1",
			firstFingerprint:   "fp-a",
			fingerprint:        "fp-b",
			accountName:        "operating",
			submits:            1,
			expectedResult:     createAccountRes{},
			expectedError:      entity.NewError("IDEMPOTENCY_CONFLICT", "idempotency key leased for a different request"),
			expectedExecutions: 1,
			expectedAuthzCalls: 2,
		},
		{
			name:               "denied subject fails before execution",
			subject:            deniedSubject,
			key:                "k-2",
			firstFingerprint:   "",
			fingerprint:        "fp-a",
			accountName:        "operating",
			submits:            1,
			expectedResult:     createAccountRes{},
			expectedError:      entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
			expectedExecutions: 0,
			expectedAuthzCalls: 1,
		},
		{
			name: "empty subject identity fails closed without adapter call",
			subject: port.Subject{
				ID:       "",
				TenantID: "t-1",
			},
			key:                "k-3",
			firstFingerprint:   "",
			fingerprint:        "fp-a",
			accountName:        "operating",
			submits:            1,
			expectedResult:     createAccountRes{},
			expectedError:      entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
			expectedExecutions: 0,
			expectedAuthzCalls: 0,
		},
		{
			name:               "invalid command never executes",
			subject:            allowedSubject,
			key:                "k-4",
			firstFingerprint:   "",
			fingerprint:        "fp-a",
			accountName:        "   ",
			submits:            1,
			expectedResult:     createAccountRes{},
			expectedError:      entity.NewError("ACCOUNT_NAME_REQUIRED", "account name is required"),
			expectedExecutions: 0,
			expectedAuthzCalls: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			authorizer := &sampleAuthorizer{denied: map[string]bool{"u-9|account.open|tenant/t-1": true}}
			handler := &createAccountHandler{authz: authorizer, store: &sampleIdemStore{}}
			if tc.firstFingerprint != "" {
				_, firstErr := handler.Handle(context.Background(), createAccountCmd{
					name:        tc.accountName,
					subject:     tc.subject,
					key:         tc.key,
					fingerprint: tc.firstFingerprint,
				})
				require.NoError(t, firstErr)
			}
			var actualResult createAccountRes
			var err error
			for i := 0; i < tc.submits; i++ {
				actualResult, err = handler.Handle(context.Background(), createAccountCmd{
					name:        tc.accountName,
					subject:     tc.subject,
					key:         tc.key,
					fingerprint: tc.fingerprint,
				})
			}
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, tc.expectedExecutions, handler.executions)
			assert.Equal(t, tc.expectedAuthzCalls, authorizer.calls)
		})
	}
}
