package outbox_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres/outbox"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
)

func TestNewWriterRequiresDB(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		params         outbox.WriterParams
		expectedResult *outbox.Writer
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "nil DB rejected",
			params: outbox.WriterParams{
				DB: nil,
			},
			expectedResult: nil,
			expectedError:  outbox.ErrOutboxWriterNeedsDB,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			writer, err := outbox.NewWriter(tc.params)
			assert.Equal(t, tc.expectedResult, writer)

			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAppendTxRequiresTx(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		ctx           context.Context
		tx            *gorm.DB
		facts         []appport.OutboxFact
		expectedError error
	}

	testCases := []testCase{
		{
			name: "nil transaction rejected before touching the database",
			ctx:  context.Background(),
			tx:   nil,
			facts: []appport.OutboxFact{
				{
					EventType:   "transfer.completed.v1",
					AggregateID: "agg-01",
				},
			},
			expectedError: outbox.ErrOutboxTransactionIsRequired,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			writer, err := outbox.NewWriter(outbox.WriterParams{DB: &gorm.DB{}})
			require.NoError(t, err)
			require.NotNil(t, writer)

			appendErr := writer.AppendTx(tc.ctx, tc.tx, tc.facts...)
			require.Error(t, appendErr)
			assert.Equal(t, tc.expectedError.Error(), appendErr.Error())
		})
	}
}

func TestNewPollerRequiresPublisher(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                 string
		params               outbox.PollerParams
		expectedMaxBatch     int
		expectedClaimTimeout time.Duration
		expectedError        error
	}

	testCases := []testCase{
		{
			name: "nil publisher rejected",
			params: outbox.PollerParams{
				Publisher:    nil,
				MaxBatch:     10,
				BaseBackoff:  time.Second,
				ClaimTimeout: time.Minute,
			},
			expectedMaxBatch:     0,
			expectedClaimTimeout: 0,
			expectedError:        outbox.ErrOutboxPollerNeedsAPublisher,
		},
		{
			name: "non-positive max batch rejected",
			params: outbox.PollerParams{
				Publisher:    mockapplication.NewMockEventPublisher(t),
				MaxBatch:     0,
				BaseBackoff:  time.Second,
				ClaimTimeout: time.Minute,
			},
			expectedMaxBatch:     0,
			expectedClaimTimeout: 0,
			expectedError:        outbox.ErrOutboxMaxBatchMustBePositive,
		},
		{
			name: "non-positive base backoff rejected",
			params: outbox.PollerParams{
				Publisher:    mockapplication.NewMockEventPublisher(t),
				MaxBatch:     10,
				BaseBackoff:  0,
				ClaimTimeout: time.Minute,
			},
			expectedMaxBatch:     0,
			expectedClaimTimeout: 0,
			expectedError:        outbox.ErrOutboxBaseBackoffMustBePositive,
		},
		{
			name: "non-positive claim timeout rejected",
			params: outbox.PollerParams{
				Publisher:    mockapplication.NewMockEventPublisher(t),
				MaxBatch:     10,
				BaseBackoff:  time.Second,
				ClaimTimeout: 0,
			},
			expectedMaxBatch:     0,
			expectedClaimTimeout: 0,
			expectedError:        outbox.ErrOutboxClaimTimeoutMustBePositive,
		},
		{
			name: "valid explicit params accepted",
			params: outbox.PollerParams{
				Publisher:    mockapplication.NewMockEventPublisher(t),
				MaxBatch:     10,
				BaseBackoff:  time.Second,
				ClaimTimeout: 30 * time.Second,
			},
			expectedMaxBatch:     10,
			expectedClaimTimeout: 30 * time.Second,
			expectedError:        nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			poller, err := outbox.NewPoller(tc.params)
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError.Error(), err.Error())
				assert.Nil(t, poller)
			} else {
				require.NoError(t, err)
				require.NotNil(t, poller)
				assert.Equal(t, tc.expectedMaxBatch, poller.MaxBatch())
				assert.Equal(t, tc.expectedClaimTimeout, poller.ClaimTimeout())
			}
		})
	}
}

func TestClaimSQLShape(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name     string
		fragment string
	}

	testCases := []testCase{
		{
			name:     "skip locked present",
			fragment: "FOR UPDATE SKIP LOCKED",
		},
		{
			name:     "undelivered filter present",
			fragment: "delivered_at IS NULL",
		},
		{
			name:     "lease timeout filter present",
			fragment: "claimed_at < now() - (? * INTERVAL '1 second')",
		},
		{
			name:     "insertion order defines relay order",
			fragment: "ORDER BY id",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, strings.ToUpper(outbox.ClaimSQL), strings.ToUpper(tc.fragment))
		})
	}
}

func TestBackoffGrowsWithJitterBound(t *testing.T) {
	t.Parallel()

	baseBackoff := 500 * time.Millisecond

	type testCase struct {
		name        string
		failures    int
		minExpected time.Duration
	}

	testCases := []testCase{
		{
			name:        "first retry near base",
			failures:    0,
			minExpected: baseBackoff,
		},
		{
			name:        "third retry at least quadruple base",
			failures:    2,
			minExpected: baseBackoff * 4,
		},
		{
			name:        "negative clamps to base",
			failures:    -3,
			minExpected: baseBackoff,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			poller, err := outbox.NewPoller(outbox.PollerParams{
				Publisher:    mockapplication.NewMockEventPublisher(t),
				MaxBatch:     100,
				BaseBackoff:  baseBackoff,
				ClaimTimeout: 60 * time.Second,
			})
			require.NoError(t, err)

			wait := poller.Backoff(tc.failures)
			assert.GreaterOrEqual(t, wait, tc.minExpected)
			assert.Less(t, wait, tc.minExpected*2+baseBackoff)
		})
	}
}

func TestRunOnceRequiresDB(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		ctx               context.Context
		db                *gorm.DB
		expectedDelivered int
		expectedError     error
	}

	testCases := []testCase{
		{
			name:              "nil DB rejected before publish",
			ctx:               context.Background(),
			db:                nil,
			expectedDelivered: 0,
			expectedError:     outbox.ErrOutboxDBIsRequired,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			poller, err := outbox.NewPoller(outbox.PollerParams{
				Publisher:    mockapplication.NewMockEventPublisher(t),
				MaxBatch:     100,
				BaseBackoff:  500 * time.Millisecond,
				ClaimTimeout: 60 * time.Second,
			})
			require.NoError(t, err)

			delivered, runErr := poller.RunOnce(tc.ctx, tc.db)
			require.Error(t, runErr)
			assert.Equal(t, tc.expectedError.Error(), runErr.Error())
			assert.Equal(t, tc.expectedDelivered, delivered)
		})
	}
}

func TestMockPublisherContract(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		ctx           context.Context
		facts         []appport.OutboxFact
		fail          error
		expectedCalls int
		expectedError error
	}

	testCases := []testCase{
		{
			name: "records one batch",
			ctx:  context.Background(),
			facts: []appport.OutboxFact{
				{EventType: "transfer.completed.v1"},
			},
			fail:          nil,
			expectedCalls: 1,
			expectedError: nil,
		},
		{
			name: "surfaces failure",
			ctx:  context.Background(),
			facts: []appport.OutboxFact{
				{EventType: "transfer.completed.v1"},
			},
			fail:          outbox.ErrBrokerDown,
			expectedCalls: 0,
			expectedError: outbox.ErrBrokerDown,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pub := mockapplication.NewMockEventPublisher(t)
			if tc.fail != nil {
				pub.EXPECT().Publish(tc.ctx, mock.Anything).Return(tc.fail).Once()
			} else {
				pub.EXPECT().Publish(tc.ctx, mock.Anything).Return(nil).Once()
			}

			err := pub.Publish(tc.ctx, tc.facts...)
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
