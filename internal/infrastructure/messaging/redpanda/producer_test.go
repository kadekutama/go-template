package redpanda_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda"
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
				Seeds: nil,
			},
			expectedError: errors.New("redpanda: at least one seed is required"),
		},
		{
			name: "whitespace seeds rejected",
			params: redpanda.ProducerParams{
				Seeds: []string{"  ", ""},
			},
			expectedError: errors.New("redpanda: at least one seed is required"),
		},
		{
			name: "unknown sasl mechanism rejected",
			params: redpanda.ProducerParams{
				Seeds:         []string{"127.0.0.1:9092"},
				SASLUser:      "app",
				SASLPass:      "secret",
				SASLMechanism: "oauthbearer",
			},
			expectedError: errors.New(`redpanda: unknown sasl mechanism "oauthbearer"`),
		},
		{
			name: "negative dial timeout rejected",
			params: redpanda.ProducerParams{
				Seeds:       []string{"127.0.0.1:9092"},
				DialTimeout: -time.Second,
			},
			expectedError: errors.New(`redpanda: invalid producer params (1 violation(s)): DialTimeout: rule "gt" on value -1s`),
		},
		{
			name: "negative buffer rejected",
			params: redpanda.ProducerParams{
				Seeds:            []string{"127.0.0.1:9092"},
				MaxBufferedBytes: -1,
			},
			expectedError: errors.New(`redpanda: invalid producer params (1 violation(s)): MaxBufferedBytes: rule "gt" on value -1`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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
			expectedError: errors.New("redpanda: producer is not initialized"),
		},
		{
			name: "blank topic rejected",
			producer: func() *redpanda.Producer {
				p, err := redpanda.NewProducer(redpanda.ProducerParams{Seeds: []string{"127.0.0.1:1"}})
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
			expectedError: errors.New("redpanda: topic is required"),
		},
		{
			name: "canceled context aborts",
			producer: func() *redpanda.Producer {
				p, err := redpanda.NewProducer(redpanda.ProducerParams{Seeds: []string{"127.0.0.1:1"}})
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
				p, err := redpanda.NewProducer(redpanda.ProducerParams{Seeds: []string{"127.0.0.1:1"}})
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
			expectedError: errors.New("redpanda: publish ledger.events.v1: client closed"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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
				p, err := redpanda.NewProducer(redpanda.ProducerParams{Seeds: []string{"127.0.0.1:1"}})
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
