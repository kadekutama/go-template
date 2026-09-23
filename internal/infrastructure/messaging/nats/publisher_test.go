package nats_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	edgenats "github.com/kadekutama/go-template/internal/infrastructure/messaging/nats"
)

func TestNewPublisher(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		params        edgenats.PublisherParams
		expectedError error
	}

	testCases := []testCase{
		{
			name: "blank rejected",
			params: edgenats.PublisherParams{
				URL: "",
			},
			expectedError: errors.New("nats: invalid params (1 violation(s)): URL: rule \"required\" on value "),
		},
		{
			name: "whitespace url rejected",
			params: edgenats.PublisherParams{
				URL: "   ",
			},
			expectedError: errors.New("nats: url is required"),
		},
		{
			name: "invalid seed rejected before dialing",
			params: edgenats.PublisherParams{
				URL:      "nats://127.0.0.1:4222",
				NKeySeed: "SU-not-a-seed",
			},
			expectedError: errors.New(
				"nats: invalid nkey seed: illegal base32 data at input byte 2",
			),
		},
		{
			name: "account seed rejected (not a user key)",
			params: func() edgenats.PublisherParams {
				account := func() []byte {
					key, _ := nkeys.CreateAccount()
					seed, _ := key.Seed()

					return seed
				}()

				return edgenats.PublisherParams{
					URL:      "nats://127.0.0.1:4222",
					NKeySeed: string(account),
				}
			}(),
			expectedError: errors.New("nats: seed is not a user nkey"),
		},
		{
			name: "unreachable broker fails fast",
			params: edgenats.PublisherParams{
				URL:            "nats://127.0.0.1:1",
				ConnectTimeout: time.Second,
			},
			expectedError: errors.New("nats: connect: nats: no servers available for connection"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			publisher, err := edgenats.NewPublisher(tc.params)
			assert.EqualError(t, err, tc.expectedError.Error())
			assert.Nil(t, publisher)
		})
	}
}

func TestPublisherPublishValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		publisher     func() *edgenats.Publisher
		ctx           context.Context
		subject       string
		payload       []byte
		expectedError error
	}

	testCases := []testCase{
		{
			name: "nil receiver rejected",
			publisher: func() *edgenats.Publisher {
				return nil
			},
			ctx:           context.Background(),
			subject:       "ledger.t1.transfer.completed.v1",
			payload:       []byte(`{}`),
			expectedError: errors.New("nats: publisher is not initialized"),
		},
		{
			name: "uninitialized connection rejected",
			publisher: func() *edgenats.Publisher {
				return &edgenats.Publisher{}
			},
			ctx:           context.Background(),
			subject:       "ledger.t1.transfer.completed.v1",
			payload:       []byte(`{}`),
			expectedError: errors.New("nats: publisher is not initialized"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.publisher()
			err := p.Publish(tc.ctx, tc.subject, tc.payload)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestPublisherPublishEventValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		publisher     func() *edgenats.Publisher
		ctx           context.Context
		tenant        string
		eventType     string
		payload       []byte
		expectedError error
	}

	testCases := []testCase{
		{
			name: "nil receiver rejected",
			publisher: func() *edgenats.Publisher {
				return nil
			},
			ctx:           context.Background(),
			tenant:        "t1",
			eventType:     "transfer.completed.v1",
			payload:       []byte(`{}`),
			expectedError: errors.New("nats: publisher is not initialized"),
		},
		{
			name: "nil payload rejected",
			publisher: func() *edgenats.Publisher {
				return &edgenats.Publisher{}
			},
			ctx:           context.Background(),
			tenant:        "t1",
			eventType:     "transfer.completed.v1",
			payload:       nil,
			expectedError: errors.New("nats: payload is required"),
		},
		{
			name: "empty tenant rejected",
			publisher: func() *edgenats.Publisher {
				return &edgenats.Publisher{}
			},
			ctx:           context.Background(),
			tenant:        "",
			eventType:     "transfer.completed.v1",
			payload:       []byte(`{}`),
			expectedError: entity.NewError("ISOLATION_SUBJECT_INVALID", "subject tenant is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.publisher()
			err := p.PublishEvent(tc.ctx, tc.tenant, tc.eventType, tc.payload)
			if tc.expectedError != nil {
				assert.Equal(t, tc.expectedError, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestPublisherClose(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		publisher     func() *edgenats.Publisher
		expectedError error
	}

	testCases := []testCase{
		{
			name: "nil receiver close succeeds",
			publisher: func() *edgenats.Publisher {
				return nil
			},
			expectedError: nil,
		},
		{
			name: "uninitialized connection close succeeds",
			publisher: func() *edgenats.Publisher {
				return &edgenats.Publisher{}
			},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.publisher()
			err := p.Close()
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				return
			}
			assert.NoError(t, err)
		})
	}
}
