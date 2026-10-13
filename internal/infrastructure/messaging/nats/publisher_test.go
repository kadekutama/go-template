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

// errInvalidNkeySeed mirrors nkeys library detail for the fixed bad-seed input.
var errInvalidNkeySeed = errors.New("nats: invalid nkey seed: illegal base32 data at input byte 2")

func TestNewPublisher(t *testing.T) {
	t.Parallel()

	baseParams := edgenats.Params{
		URL:            "nats://127.0.0.1:4222",
		ConnectTimeout: 5 * time.Second,
		RequestTimeout: 3 * time.Second,
	}

	type testCase struct {
		name          string
		params        edgenats.Params
		expectedError error
	}

	testCases := []testCase{
		{
			name: "blank url rejected",
			params: func() edgenats.Params {
				p := baseParams
				p.URL = ""

				return p
			}(),
			expectedError: edgenats.ErrNatsInvalidParams1ViolationSURLRuleRequiredOnValue,
		},
		{
			name: "whitespace url rejected",
			params: func() edgenats.Params {
				p := baseParams
				p.URL = "   "

				return p
			}(),
			expectedError: edgenats.ErrNatsURLIsRequired,
		},
		{
			name: "missing connect timeout rejected",
			params: func() edgenats.Params {
				p := baseParams
				p.ConnectTimeout = 0

				return p
			}(),
			expectedError: edgenats.ErrNatsInvalidParams1ViolationSConnectTimeoutRuleRequiredOnV,
		},
		{
			name: "negative connect timeout rejected",
			params: func() edgenats.Params {
				p := baseParams
				p.ConnectTimeout = -time.Second

				return p
			}(),
			expectedError: edgenats.ErrNatsInvalidParams1ViolationSConnectTimeoutRuleGtOnValue1s,
		},
		{
			name: "missing request timeout rejected",
			params: func() edgenats.Params {
				p := baseParams
				p.RequestTimeout = 0

				return p
			}(),
			expectedError: edgenats.ErrNatsInvalidParams1ViolationSRequestTimeoutRuleRequiredOnV,
		},
		{
			name: "negative request timeout rejected",
			params: func() edgenats.Params {
				p := baseParams
				p.RequestTimeout = -time.Second

				return p
			}(),
			expectedError: edgenats.ErrNatsInvalidParams1ViolationSRequestTimeoutRuleGtOnValue1s,
		},
		{
			name: "invalid seed rejected before dialing",
			params: func() edgenats.Params {
				p := baseParams
				p.NKeySeed = "SU-not-a-seed"

				return p
			}(),
			expectedError: errInvalidNkeySeed,
		},
		{
			name: "account seed rejected (not a user key)",
			params: func() edgenats.Params {
				account := func() []byte {
					key, _ := nkeys.CreateAccount()
					seed, _ := key.Seed()

					return seed
				}()

				p := baseParams
				p.NKeySeed = string(account)

				return p
			}(),
			expectedError: edgenats.ErrNatsSeedIsNotAUserNkey,
		},
		{
			name: "unreachable broker fails fast",
			params: func() edgenats.Params {
				p := baseParams
				p.URL = "nats://127.0.0.1:1"
				p.ConnectTimeout = time.Second

				return p
			}(),
			expectedError: edgenats.ErrNatsConnectNatsNoServersAvailableForConnection,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

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
			expectedError: edgenats.ErrNatsPublisherIsNotInitialized,
		},
		{
			name: "uninitialized connection rejected",
			publisher: func() *edgenats.Publisher {
				return &edgenats.Publisher{}
			},
			ctx:           context.Background(),
			subject:       "ledger.t1.transfer.completed.v1",
			payload:       []byte(`{}`),
			expectedError: edgenats.ErrNatsPublisherIsNotInitialized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

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
			expectedError: edgenats.ErrNatsPublisherIsNotInitialized,
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
			expectedError: edgenats.ErrNatsPayloadIsRequired,
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
			t.Parallel()

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
			t.Parallel()

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
