package consumer_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda/consumer"
	mockconsumer "github.com/kadekutama/go-template/test/mock/consumer"
)

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
				TTL:    time.Hour,
			},
			expectedError: errors.New("consumer: invalid receipt params (1 violation(s)): Client: rule \"required\" on value <nil>"),
		},
		{
			name: "non-positive ttl rejected",
			params: consumer.ValkeyReceiptParams{
				Client: mockconsumer.NewMockReceiptKV(t),
				TTL:    0,
			},
			expectedError: errors.New("consumer: invalid receipt params (1 violation(s)): TTL: rule \"required\" on value 0s"),
		},
		{
			name: "valid client accepted",
			params: consumer.ValkeyReceiptParams{
				Client: mockconsumer.NewMockReceiptKV(t),
				TTL:    time.Hour,
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

func newMockReceiptKV(t *testing.T, fail error) *mockconsumer.MockReceiptKV {
	t.Helper()
	m := mockconsumer.NewMockReceiptKV(t)
	var data sync.Map
	m.EXPECT().SetNX(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, key string, _ []byte, _ time.Duration) (bool, error) {
			if fail != nil {
				return false, fail
			}
			_, loaded := data.LoadOrStore(key, struct{}{})
			return !loaded, nil
		}).Maybe()
	m.EXPECT().Delete(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, key string) error {
			if fail != nil {
				return fail
			}
			data.Delete(key)
			return nil
		}).Maybe()
	return m
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
			kv := newMockReceiptKV(t, tc.kvFail)
			store, err := consumer.NewValkeyReceiptStore(consumer.ValkeyReceiptParams{Client: kv, TTL: time.Hour})
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
			kv := newMockReceiptKV(t, tc.kvFail)
			store, err := consumer.NewValkeyReceiptStore(consumer.ValkeyReceiptParams{Client: kv, TTL: time.Hour})
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
