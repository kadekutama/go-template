package command_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
)

func TestFingerprint(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		partsA        []string
		partsB        []string
		expectedEqual bool
	}

	testCases := []testCase{
		{
			name:          "identical parts produce identical fingerprints",
			partsA:        []string{"tenant-1", "account-1", "USD", "1000"},
			partsB:        []string{"tenant-1", "account-1", "USD", "1000"},
			expectedEqual: true,
		},
		{
			name:          "ambiguity guard differentiates concatenated boundaries",
			partsA:        []string{"ab", "c"},
			partsB:        []string{"a", "bc"},
			expectedEqual: false,
		},
		{
			name:          "different values produce different fingerprints",
			partsA:        []string{"tenant-1", "account-1"},
			partsB:        []string{"tenant-1", "account-2"},
			expectedEqual: false,
		},
		{
			name:          "empty part vs non-empty part",
			partsA:        []string{"a", "", "b"},
			partsB:        []string{"a", "b"},
			expectedEqual: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fpA := command.Fingerprint(tc.partsA...)
			fpB := command.Fingerprint(tc.partsB...)
			if tc.expectedEqual {
				assert.Equal(t, fpA, fpB)
			} else {
				assert.NotEqual(t, fpA, fpB)
			}
		})
	}
}

func TestMapParts(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		prefix        string
		inputMap      map[string]string
		expectedParts []string
	}

	testCases := []testCase{
		{
			name:          "empty map returns only prefix",
			prefix:        "meta",
			inputMap:      map[string]string{},
			expectedParts: []string{"meta"},
		},
		{
			name:   "keys are sorted deterministically",
			prefix: "headers",
			inputMap: map[string]string{
				"zebra": "1",
				"apple": "2",
				"mango": "3",
			},
			expectedParts: []string{"headers", "apple", "2", "mango", "3", "zebra", "1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := command.MapParts(tc.prefix, tc.inputMap)
			assert.Equal(t, tc.expectedParts, actual)
		})
	}
}

type fakeIdempotencyStore struct {
	reserveFunc  func(ctx context.Context, rec port.IdempotencyRecord) (port.ReserveOutcome, error)
	completeFunc func(ctx context.Context, key string, response []byte) error
}

func (s *fakeIdempotencyStore) Reserve(ctx context.Context, rec port.IdempotencyRecord) (port.ReserveOutcome, error) {
	if s.reserveFunc != nil {
		return s.reserveFunc(ctx, rec)
	}
	return port.ReserveOutcome{}, nil
}

func (s *fakeIdempotencyStore) Complete(ctx context.Context, key string, response []byte) error {
	if s.completeFunc != nil {
		return s.completeFunc(ctx, key, response)
	}
	return nil
}

func TestRunIdempotent(t *testing.T) {
	t.Parallel()

	baseRec := port.IdempotencyRecord{
		Key:         "key-1",
		Fingerprint: "fp-1",
		TenantID:    "t-1",
	}

	type testCase struct {
		name             string
		store            port.IdempotencyStore
		rec              port.IdempotencyRecord
		execute          func(ctx context.Context) ([]byte, error)
		expectedResponse []byte
		expectedReplayed bool
		expectedError    error
	}

	testCases := []testCase{
		{
			name: "fresh execution completes and returns response",
			store: &fakeIdempotencyStore{
				reserveFunc: func(context.Context, port.IdempotencyRecord) (port.ReserveOutcome, error) {
					return port.ReserveOutcome{Replay: false}, nil
				},
				completeFunc: func(context.Context, string, []byte) error {
					return nil
				},
			},
			rec: baseRec,
			execute: func(context.Context) ([]byte, error) {
				return []byte(`{"status":"ok"}`), nil
			},
			expectedResponse: []byte(`{"status":"ok"}`),
			expectedReplayed: false,
			expectedError:    nil,
		},
		{
			name: "replayed execution returns stored response without executing",
			store: &fakeIdempotencyStore{
				reserveFunc: func(context.Context, port.IdempotencyRecord) (port.ReserveOutcome, error) {
					return port.ReserveOutcome{Replay: true, Response: []byte(`{"replayed":true}`)}, nil
				},
			},
			rec: baseRec,
			execute: func(context.Context) ([]byte, error) {
				return nil, errors.New("should not execute on replay")
			},
			expectedResponse: []byte(`{"replayed":true}`),
			expectedReplayed: true,
			expectedError:    nil,
		},
		{
			name: "reserve conflict returns error immediately without executing",
			store: &fakeIdempotencyStore{
				reserveFunc: func(context.Context, port.IdempotencyRecord) (port.ReserveOutcome, error) {
					return port.ReserveOutcome{}, entity.NewError("IDEMPOTENCY_CONFLICT", "fingerprint mismatch")
				},
			},
			rec: baseRec,
			execute: func(context.Context) ([]byte, error) {
				return nil, errors.New("should not execute on conflict")
			},
			expectedResponse: nil,
			expectedReplayed: false,
			expectedError:    entity.NewError("IDEMPOTENCY_CONFLICT", "fingerprint mismatch"),
		},
		{
			name: "execution error propagates and does not complete store",
			store: &fakeIdempotencyStore{
				reserveFunc: func(context.Context, port.IdempotencyRecord) (port.ReserveOutcome, error) {
					return port.ReserveOutcome{Replay: false}, nil
				},
				completeFunc: func(context.Context, string, []byte) error {
					return errors.New("should not complete on failure")
				},
			},
			rec: baseRec,
			execute: func(context.Context) ([]byte, error) {
				return nil, entity.NewError("EXECUTION_FAILED", "business rule failed")
			},
			expectedResponse: nil,
			expectedReplayed: false,
			expectedError:    entity.NewError("EXECUTION_FAILED", "business rule failed"),
		},
		{
			name: "store complete error propagates",
			store: &fakeIdempotencyStore{
				reserveFunc: func(context.Context, port.IdempotencyRecord) (port.ReserveOutcome, error) {
					return port.ReserveOutcome{Replay: false}, nil
				},
				completeFunc: func(context.Context, string, []byte) error {
					return entity.NewError("STORE_ERROR", "failed to commit idempotency")
				},
			},
			rec: baseRec,
			execute: func(context.Context) ([]byte, error) {
				return []byte(`{"data":"valid"}`), nil
			},
			expectedResponse: nil,
			expectedReplayed: false,
			expectedError:    entity.NewError("STORE_ERROR", "failed to commit idempotency"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res, replayed, err := command.RunIdempotent(context.Background(), tc.store, tc.rec, tc.execute)
			assert.Equal(t, tc.expectedResponse, res)
			assert.Equal(t, tc.expectedReplayed, replayed)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
