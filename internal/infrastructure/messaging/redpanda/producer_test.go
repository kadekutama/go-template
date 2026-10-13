package redpanda_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda"
	"github.com/kadekutama/go-template/internal/shared/kernel/validate"
)

func TestNewProducer(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		params        redpanda.ProducerParams
		expectedError error
	}

	testCases := []testCase{
		{
			name: "blank seeds rejected without dialing",
			params: redpanda.ProducerParams{
				Seeds:       nil,
				DialTimeout: 5 * time.Second,
			},
			expectedError: redpanda.ErrRedpandaAtLeastOneSeedIsRequired,
		},
		{
			name: "whitespace seeds rejected",
			params: redpanda.ProducerParams{
				Seeds:       []string{"  ", ""},
				DialTimeout: 5 * time.Second,
			},
			expectedError: redpanda.ErrRedpandaAtLeastOneSeedIsRequired,
		},
		{
			name: "unknown sasl mechanism rejected",
			params: redpanda.ProducerParams{
				Seeds:         []string{"127.0.0.1:9092"},
				SASLUser:      "app",
				SASLPass:      "secret",
				SASLMechanism: "oauthbearer",
				DialTimeout:   5 * time.Second,
			},
			expectedError: fmt.Errorf("redpanda: unknown sasl mechanism %q: %w", "oauthbearer", redpanda.ErrRedpandaUnknownSaslMechanism),
		},
		{
			name: "missing sasl mechanism rejected when user is provided",
			params: redpanda.ProducerParams{
				Seeds:         []string{"127.0.0.1:9092"},
				SASLUser:      "app",
				SASLPass:      "secret",
				SASLMechanism: "",
				DialTimeout:   5 * time.Second,
			},
			expectedError: redpanda.ErrRedpandaSaslMechanismIsRequiredWhenSaslCredentialsAreProv,
		},
		{
			name: "missing dial timeout rejected",
			params: redpanda.ProducerParams{
				Seeds:       []string{"127.0.0.1:9092"},
				DialTimeout: 0,
			},
			expectedError: fmt.Errorf(`redpanda: invalid producer params (1 violation(s)): DialTimeout: rule "required" on value 0s: %w`, validate.ErrValidateViolation),
		},
		{
			name: "negative dial timeout rejected",
			params: redpanda.ProducerParams{
				Seeds:       []string{"127.0.0.1:9092"},
				DialTimeout: -time.Second,
			},
			expectedError: fmt.Errorf(`redpanda: invalid producer params (1 violation(s)): DialTimeout: rule "gt" on value -1s: %w`, validate.ErrValidateViolation),
		},
		{
			name: "negative buffer rejected",
			params: redpanda.ProducerParams{
				Seeds:            []string{"127.0.0.1:9092"},
				DialTimeout:      5 * time.Second,
				MaxBufferedBytes: -1,
			},
			expectedError: fmt.Errorf(`redpanda: invalid producer params (1 violation(s)): MaxBufferedBytes: rule "gt" on value -1: %w`, validate.ErrValidateViolation),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			producer, err := redpanda.NewProducer(tc.params)
			assert.EqualError(t, err, tc.expectedError.Error())
			assert.Nil(t, producer)
		})
	}
}

func TestProducerPublishValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		producer      func() *redpanda.Producer
		ctx           context.Context
		topic         string
		key           string
		headers       map[string]string
		payload       []byte
		expectedError error
	}

	testCases := []testCase{
		{
			name: "nil producer rejected",
			producer: func() *redpanda.Producer {
				return nil
			},
			ctx:           context.Background(),
			topic:         "ledger.events.v1",
			key:           "k",
			headers:       map[string]string{},
			payload:       []byte(`{}`),
			expectedError: redpanda.ErrRedpandaProducerIsNotInitialized,
		},
		{
			name: "blank topic rejected",
			producer: func() *redpanda.Producer {
				p, err := redpanda.NewProducer(redpanda.ProducerParams{
					Seeds:       []string{"127.0.0.1:1"},
					DialTimeout: 5 * time.Second,
				})
				if err != nil {
					panic(err)
				}

				return p
			},
			ctx:           context.Background(),
			topic:         "   ",
			key:           "k",
			headers:       map[string]string{},
			payload:       []byte(`{}`),
			expectedError: redpanda.ErrRedpandaTopicIsRequired,
		},
		{
			name: "canceled context aborts",
			producer: func() *redpanda.Producer {
				p, err := redpanda.NewProducer(redpanda.ProducerParams{
					Seeds:       []string{"127.0.0.1:1"},
					DialTimeout: 5 * time.Second,
				})
				if err != nil {
					panic(err)
				}

				return p
			},
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx
			}(),
			topic:         "ledger.events.v1",
			key:           "k",
			headers:       map[string]string{},
			payload:       []byte(`{}`),
			expectedError: context.Canceled,
		},
		{
			name: "publish after close returns client closed error",
			producer: func() *redpanda.Producer {
				p, err := redpanda.NewProducer(redpanda.ProducerParams{
					Seeds:       []string{"127.0.0.1:1"},
					DialTimeout: 5 * time.Second,
				})
				if err != nil {
					panic(err)
				}

				_ = p.Close()

				return p
			},
			ctx:           context.Background(),
			topic:         "ledger.events.v1",
			key:           "k",
			headers:       map[string]string{},
			payload:       []byte(`{}`),
			expectedError: redpanda.ErrRedpandaPublishLedgerEventsV1ClientClosed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			producer := tc.producer()
			if producer != nil {
				defer func() { _ = producer.Close() }()
			}

			err := producer.Publish(tc.ctx, tc.topic, tc.key, tc.headers, tc.payload)
			if errors.Is(tc.expectedError, context.Canceled) {
				assert.ErrorIs(t, err, context.Canceled)
			} else {
				assert.EqualError(t, err, tc.expectedError.Error())
			}
		})
	}
}

func TestProducerClose(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		producer      func() *redpanda.Producer
		expectedError error
	}

	testCases := []testCase{
		{
			name: "nil producer close succeeds",
			producer: func() *redpanda.Producer {
				return nil
			},
			expectedError: nil,
		},
		{
			name: "constructed producer close succeeds",
			producer: func() *redpanda.Producer {
				p, err := redpanda.NewProducer(redpanda.ProducerParams{
					Seeds:       []string{"127.0.0.1:1"},
					DialTimeout: 5 * time.Second,
				})
				if err != nil {
					panic(err)
				}

				return p
			},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p := tc.producer()

			err := p.Close()
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())

				return
			}

			require.NoError(t, err)
		})
	}
}
