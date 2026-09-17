package outbox_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres/outbox"
)

// stubPublisher records published facts and fails on demand.
type stubPublisher struct {
	published [][]appport.OutboxFact
	fail      error
}

func (s *stubPublisher) Publish(_ context.Context, facts ...appport.OutboxFact) error {
	if s.fail != nil {
		return s.fail
	}

	s.published = append(s.published, facts)

	return nil
}

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
			expectedError:  errors.New("outbox: writer needs DB"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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
			expectedError: errors.New("outbox: transaction is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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
				Publisher: nil,
				MaxBatch:  0,
			},
			expectedMaxBatch:     0,
			expectedClaimTimeout: 0,
			expectedError:        errors.New("outbox: poller needs a publisher"),
		},
		{
			name: "default batch and claim timeout applied",
			params: outbox.PollerParams{
				Publisher: &stubPublisher{},
				MaxBatch:  0,
			},
			expectedMaxBatch:     outbox.DefaultMaxBatch,
			expectedClaimTimeout: outbox.DefaultClaimTimeout,
			expectedError:        nil,
		},
		{
			name: "explicit batch and claim timeout kept",
			params: outbox.PollerParams{
				Publisher:    &stubPublisher{},
				MaxBatch:     10,
				ClaimTimeout: 30 * time.Second,
			},
			expectedMaxBatch:     10,
			expectedClaimTimeout: 30 * time.Second,
			expectedError:        nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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
			assert.Contains(t, strings.ToUpper(outbox.ClaimSQL), strings.ToUpper(tc.fragment))
		})
	}
}

func TestBackoffGrowsWithJitterBound(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name        string
		failures    int
		minExpected time.Duration
	}

	testCases := []testCase{
		{
			name:        "first retry near base",
			failures:    0,
			minExpected: outbox.DefaultBaseBackoff,
		},
		{
			name:        "third retry at least quadruple base",
			failures:    2,
			minExpected: outbox.DefaultBaseBackoff * 4,
		},
		{
			name:        "negative clamps to base",
			failures:    -3,
			minExpected: outbox.DefaultBaseBackoff,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			poller, err := outbox.NewPoller(outbox.PollerParams{Publisher: &stubPublisher{}})
			require.NoError(t, err)

			wait := poller.Backoff(tc.failures)
			assert.GreaterOrEqual(t, wait, tc.minExpected)
			assert.Less(t, wait, tc.minExpected*2+outbox.DefaultBaseBackoff)
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
			expectedError:     errors.New("outbox: DB is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			poller, err := outbox.NewPoller(outbox.PollerParams{Publisher: &stubPublisher{}})
			require.NoError(t, err)

			delivered, runErr := poller.RunOnce(tc.ctx, tc.db)
			require.Error(t, runErr)
			assert.Equal(t, tc.expectedError.Error(), runErr.Error())
			assert.Equal(t, tc.expectedDelivered, delivered)
		})
	}
}

func TestStubPublisherContract(t *testing.T) {
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
			fail:          errors.New("broker down"),
			expectedCalls: 0,
			expectedError: errors.New("broker down"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubPublisher{fail: tc.fail}
			err := stub.Publish(tc.ctx, tc.facts...)
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}

			assert.Len(t, stub.published, tc.expectedCalls)
		})
	}
}
