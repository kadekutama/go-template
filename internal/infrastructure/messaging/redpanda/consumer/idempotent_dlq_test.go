package consumer_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda/consumer"
	fakes "github.com/kadekutama/go-template/test/fakes"
)

func TestReceiptStoreFake(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		consumer      string
		eventID       string
		expectedError bool
	}

	testCases := []testCase{
		{
			name:          "first claim wins",
			consumer:      "g",
			eventID:       "e-1",
			expectedError: false,
		},
		{
			name:          "blank consumer rejected",
			consumer:      "",
			eventID:       "e-1",
			expectedError: true,
		},
		{
			name:          "blank event rejected",
			consumer:      "g",
			eventID:       "",
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := fakes.NewReceiptStore()
			duplicate, err := store.Claim(context.Background(), tc.consumer, tc.eventID)
			if tc.expectedError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.False(t, duplicate)

			duplicateAgain, err := store.Claim(context.Background(), tc.consumer, tc.eventID)
			require.NoError(t, err)
			assert.True(t, duplicateAgain)
		})
	}
}

func TestMessageDLQFake(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		msg           appport.Message
		expectedError error
	}

	testCases := []testCase{
		{
			name: "valid message recorded",
			msg: appport.Message{
				ID: "dlq-1",
				TenantID: func() valueobject.TenantID {
					t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000040")
					return t
				}(),
				Subject:     "ledger.t1.transfer.completed.v1",
				Payload:     []byte(`{"minor":1}`),
				Redelivered: 5,
			},
			expectedError: nil,
		},
		{
			name: "empty id rejected",
			msg: appport.Message{
				ID: "",
				TenantID: func() valueobject.TenantID {
					t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000040")
					return t
				}(),
				Subject:     "ledger.t1.transfer.completed.v1",
				Payload:     []byte(`{"minor":1}`),
				Redelivered: 5,
			},
			expectedError: errors.New("dlq: message id is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dlq := fakes.NewMessageDLQ()
			err := dlq.Record(context.Background(), tc.msg)
			if tc.expectedError != nil {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, 1, dlq.Len())
			assert.Len(t, dlq.List(), 1)
		})
	}
}

func TestValkeyReceiptStoreValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		params        consumer.ValkeyReceiptParams
		expectedError error
	}

	testCases := []testCase{
		{
			name: "nil client rejected",
			params: consumer.ValkeyReceiptParams{
				Client: nil,
			},
			expectedError: errors.New("consumer: valkey client is required"),
		},
		{
			name: "valid client accepted",
			params: consumer.ValkeyReceiptParams{
				Client: &fakeKV{},
			},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store, err := consumer.NewValkeyReceiptStore(tc.params)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				assert.Nil(t, store)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, store)
		})
	}
}

// fakeKV is an in-memory SetNX seam for receipt tests.
type fakeKV struct {
	mu   sync.Mutex
	data map[string]struct{}
	fail error
}

func (f *fakeKV) SetNX(_ context.Context, key string, _ []byte, _ time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.fail != nil {
		return false, f.fail
	}

	if f.data == nil {
		f.data = make(map[string]struct{})
	}

	if _, ok := f.data[key]; ok {
		return false, nil
	}

	f.data[key] = struct{}{}

	return true, nil
}

func (f *fakeKV) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.fail != nil {
		return f.fail
	}

	delete(f.data, key)

	return nil
}

func TestReceiptStoreFakeRelease(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		consumer      string
		eventID       string
		preClaim      bool
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "successful release",
			consumer:      "g",
			eventID:       "e-1",
			preClaim:      true,
			expectedError: nil,
		},
		{
			name:          "release missing receipt succeeds",
			consumer:      "g",
			eventID:       "e-missing",
			preClaim:      false,
			expectedError: nil,
		},
		{
			name:          "blank consumer rejected",
			consumer:      "",
			eventID:       "e-1",
			preClaim:      false,
			expectedError: errors.New("receipt: consumer is required"),
		},
		{
			name:          "blank event rejected",
			consumer:      "g",
			eventID:       "",
			preClaim:      false,
			expectedError: errors.New("receipt: event id is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := fakes.NewReceiptStore()
			ctx := context.Background()

			if tc.preClaim {
				dup, err := store.Claim(ctx, tc.consumer, tc.eventID)
				require.NoError(t, err)
				assert.False(t, dup)
			}

			err := store.Release(ctx, tc.consumer, tc.eventID)
			if tc.expectedError != nil {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValkeyReceiptStoreClaim(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		kvFail            error
		consumer          string
		eventID           string
		preClaim          bool
		expectedDuplicate bool
		expectedError     error
	}

	testCases := []testCase{
		{
			name:              "first claim wins",
			kvFail:            nil,
			consumer:          "g",
			eventID:           "e-1",
			preClaim:          false,
			expectedDuplicate: false,
			expectedError:     nil,
		},
		{
			name:              "second claim is duplicate",
			kvFail:            nil,
			consumer:          "g",
			eventID:           "e-1",
			preClaim:          true,
			expectedDuplicate: true,
			expectedError:     nil,
		},
		{
			name:              "store error surfaces",
			kvFail:            errors.New("down"),
			consumer:          "g",
			eventID:           "e-1",
			preClaim:          false,
			expectedDuplicate: false,
			expectedError:     errors.New("down"),
		},
		{
			name:              "blank consumer rejected",
			kvFail:            nil,
			consumer:          "",
			eventID:           "e-1",
			preClaim:          false,
			expectedDuplicate: false,
			expectedError:     errors.New("receipt: consumer is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kv := &fakeKV{fail: tc.kvFail}
			store, err := consumer.NewValkeyReceiptStore(consumer.ValkeyReceiptParams{Client: kv})
			require.NoError(t, err)

			if tc.preClaim {
				_, err := store.Claim(context.Background(), tc.consumer, tc.eventID)
				require.NoError(t, err)
			}

			duplicate, err := store.Claim(context.Background(), tc.consumer, tc.eventID)
			if tc.expectedError != nil {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedDuplicate, duplicate)
		})
	}
}

func TestValkeyReceiptStoreRelease(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		kvFail        error
		consumer      string
		eventID       string
		preClaim      bool
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "successful release",
			kvFail:        nil,
			consumer:      "g",
			eventID:       "e-1",
			preClaim:      true,
			expectedError: nil,
		},
		{
			name:          "release store error surfaces",
			kvFail:        errors.New("down"),
			consumer:      "g",
			eventID:       "e-1",
			preClaim:      false,
			expectedError: errors.New("down"),
		},
		{
			name:          "blank consumer rejected",
			kvFail:        nil,
			consumer:      "",
			eventID:       "e-1",
			preClaim:      false,
			expectedError: errors.New("receipt: consumer is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kv := &fakeKV{fail: tc.kvFail}
			store, err := consumer.NewValkeyReceiptStore(consumer.ValkeyReceiptParams{Client: kv})
			require.NoError(t, err)

			if tc.preClaim {
				_, err := store.Claim(context.Background(), tc.consumer, tc.eventID)
				require.NoError(t, err)
			}

			err = store.Release(context.Background(), tc.consumer, tc.eventID)
			if tc.expectedError != nil {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
