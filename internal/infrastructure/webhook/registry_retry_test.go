package webhook_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/infrastructure/webhook"
	fakes "github.com/kadekutama/go-template/test/fakes"
)

func TestEndpointNormalized(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		endpoint       webhook.Endpoint
		expectedResult webhook.Endpoint
	}

	testCases := []testCase{
		{
			name: "trims whitespace on all fields",
			endpoint: webhook.Endpoint{
				ID:     "  ep-1  ",
				Tenant: "  tenant-a  ",
				URL:    "  https://example.com/webhook  ",
				Events: []string{"  transfer.completed.v1  ", "  payment.settled.v1  "},
				Secret: "  s1  ",
				Active: true,
			},
			expectedResult: webhook.Endpoint{
				ID:     "ep-1",
				Tenant: "tenant-a",
				URL:    "https://example.com/webhook",
				Events: []string{"transfer.completed.v1", "payment.settled.v1"},
				Secret: "s1",
				Active: true,
			},
		},
		{
			name: "clean fields remain unmodified",
			endpoint: webhook.Endpoint{
				ID:     "ep-2",
				Tenant: "tenant-b",
				URL:    "https://example.com/hook",
				Events: []string{"transfer.completed.v1"},
				Secret: "sec-2",
				Active: false,
			},
			expectedResult: webhook.Endpoint{
				ID:     "ep-2",
				Tenant: "tenant-b",
				URL:    "https://example.com/hook",
				Events: []string{"transfer.completed.v1"},
				Secret: "sec-2",
				Active: false,
			},
		},
		{
			name: "empty events slice",
			endpoint: webhook.Endpoint{
				ID:     "ep-3",
				Tenant: "tenant-c",
				URL:    "https://example.com/hook",
				Events: []string{},
				Secret: "sec-3",
			},
			expectedResult: webhook.Endpoint{
				ID:     "ep-3",
				Tenant: "tenant-c",
				URL:    "https://example.com/hook",
				Events: []string{},
				Secret: "sec-3",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := tc.endpoint.Normalized()
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestEndpointValidate(t *testing.T) {
	t.Parallel()

	base := webhook.Endpoint{
		ID:     "ep-1",
		Tenant: "t1",
		URL:    "https://m.example.com/hook",
		Events: []string{"transfer.completed.v1"},
		Secret: "s1",
	}

	type testCase struct {
		name          string
		endpoint      webhook.Endpoint
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid endpoint",
			endpoint:      base,
			expectedError: nil,
		},
		{
			name: "blank id rejected",
			endpoint: func() webhook.Endpoint {
				e := base
				e.ID = ""
				return e
			}(),
			expectedError: errors.New("webhook: invalid endpoint (1 violation(s)): ID: rule \"required\" on value "),
		},
		{
			name: "blank tenant rejected",
			endpoint: func() webhook.Endpoint {
				e := base
				e.Tenant = ""
				return e
			}(),
			expectedError: errors.New("webhook: invalid endpoint (1 violation(s)): Tenant: rule \"required\" on value "),
		},
		{
			name: "non-http url rejected",
			endpoint: func() webhook.Endpoint {
				e := base
				e.URL = "ftp://x"
				return e
			}(),
			expectedError: errors.New("webhook: invalid endpoint (1 violation(s)): URL: rule \"http_url\" on value ftp://x"),
		},
		{
			name: "blank url rejected",
			endpoint: func() webhook.Endpoint {
				e := base
				e.URL = ""
				return e
			}(),
			expectedError: errors.New("webhook: invalid endpoint (1 violation(s)): URL: rule \"required\" on value "),
		},
		{
			name: "nil events rejected by required rule",
			endpoint: func() webhook.Endpoint {
				e := base
				e.Events = nil
				return e
			}(),
			expectedError: errors.New("webhook: invalid endpoint (1 violation(s)): Events: rule \"required\" on value []"),
		},
		{
			name: "empty events slice rejected by min rule",
			endpoint: func() webhook.Endpoint {
				e := base
				e.Events = []string{}
				return e
			}(),
			expectedError: errors.New("webhook: invalid endpoint (1 violation(s)): Events: rule \"min\" on value []"),
		},
		{
			name: "blank event in list rejected",
			endpoint: func() webhook.Endpoint {
				e := base
				e.Events = []string{""}
				return e
			}(),
			expectedError: errors.New("webhook: invalid endpoint (1 violation(s)): Events[0]: rule \"required\" on value "),
		},
		{
			name: "blank secret rejected",
			endpoint: func() webhook.Endpoint {
				e := base
				e.Secret = ""
				return e
			}(),
			expectedError: errors.New("webhook: invalid endpoint (1 violation(s)): Secret: rule \"required\" on value "),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.endpoint.Validate()
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegistryUpsert(t *testing.T) {
	t.Parallel()

	base := webhook.Endpoint{
		ID:     "ep-1",
		Tenant: "t1",
		URL:    "https://m.example.com/hook",
		Events: []string{"transfer.completed.v1"},
		Secret: "s1",
	}

	type testCase struct {
		name          string
		r             *webhook.Registry
		ctx           context.Context
		endpoint      webhook.Endpoint
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "nil receiver rejected",
			r:             nil,
			ctx:           context.Background(),
			endpoint:      base,
			expectedError: errors.New("webhook: registry is not initialized"),
		},
		{
			name:          "valid endpoint stored successfully",
			r:             webhook.NewRegistry(webhook.RegistryParams{}),
			ctx:           context.Background(),
			endpoint:      base,
			expectedError: nil,
		},
		{
			name: "endpoint with surrounding whitespace normalized and stored",
			r:    webhook.NewRegistry(webhook.RegistryParams{}),
			ctx:  context.Background(),
			endpoint: func() webhook.Endpoint {
				e := base
				e.ID = "  ep-ws  "
				e.Tenant = "  t1  "
				e.URL = "  https://m.example.com/hook  "
				e.Events = []string{"  transfer.completed.v1  "}
				e.Secret = "  s1  "
				return e
			}(),
			expectedError: nil,
		},
		{
			name: "blank id rejected",
			r:    webhook.NewRegistry(webhook.RegistryParams{}),
			ctx:  context.Background(),
			endpoint: func() webhook.Endpoint {
				e := base
				e.ID = ""
				return e
			}(),
			expectedError: errors.New("webhook: invalid endpoint (1 violation(s)): ID: rule \"required\" on value "),
		},
		{
			name: "whitespace id rejected after normalization",
			r:    webhook.NewRegistry(webhook.RegistryParams{}),
			ctx:  context.Background(),
			endpoint: func() webhook.Endpoint {
				e := base
				e.ID = "   "
				return e
			}(),
			expectedError: errors.New("webhook: invalid endpoint (1 violation(s)): ID: rule \"required\" on value "),
		},
		{
			name: "blank tenant rejected",
			r:    webhook.NewRegistry(webhook.RegistryParams{}),
			ctx:  context.Background(),
			endpoint: func() webhook.Endpoint {
				e := base
				e.Tenant = ""
				return e
			}(),
			expectedError: errors.New("webhook: invalid endpoint (1 violation(s)): Tenant: rule \"required\" on value "),
		},
		{
			name: "whitespace tenant rejected after normalization",
			r:    webhook.NewRegistry(webhook.RegistryParams{}),
			ctx:  context.Background(),
			endpoint: func() webhook.Endpoint {
				e := base
				e.Tenant = "   "
				return e
			}(),
			expectedError: errors.New("webhook: invalid endpoint (1 violation(s)): Tenant: rule \"required\" on value "),
		},
		{
			name: "non-http url rejected",
			r:    webhook.NewRegistry(webhook.RegistryParams{}),
			ctx:  context.Background(),
			endpoint: func() webhook.Endpoint {
				e := base
				e.URL = "ftp://x"
				return e
			}(),
			expectedError: errors.New("webhook: invalid endpoint (1 violation(s)): URL: rule \"http_url\" on value ftp://x"),
		},
		{
			name: "blank url rejected",
			r:    webhook.NewRegistry(webhook.RegistryParams{}),
			ctx:  context.Background(),
			endpoint: func() webhook.Endpoint {
				e := base
				e.URL = ""
				return e
			}(),
			expectedError: errors.New("webhook: invalid endpoint (1 violation(s)): URL: rule \"required\" on value "),
		},
		{
			name: "no events rejected",
			r:    webhook.NewRegistry(webhook.RegistryParams{}),
			ctx:  context.Background(),
			endpoint: func() webhook.Endpoint {
				e := base
				e.Events = nil
				return e
			}(),
			expectedError: errors.New("webhook: invalid endpoint (1 violation(s)): Events: rule \"min\" on value []"),
		},
		{
			name: "blank event in list rejected after normalization",
			r:    webhook.NewRegistry(webhook.RegistryParams{}),
			ctx:  context.Background(),
			endpoint: func() webhook.Endpoint {
				e := base
				e.Events = []string{"   "}
				return e
			}(),
			expectedError: errors.New("webhook: invalid endpoint (1 violation(s)): Events[0]: rule \"required\" on value "),
		},
		{
			name: "blank secret rejected",
			r:    webhook.NewRegistry(webhook.RegistryParams{}),
			ctx:  context.Background(),
			endpoint: func() webhook.Endpoint {
				e := base
				e.Secret = ""
				return e
			}(),
			expectedError: errors.New("webhook: invalid endpoint (1 violation(s)): Secret: rule \"required\" on value "),
		},
		{
			name: "whitespace secret rejected after normalization",
			r:    webhook.NewRegistry(webhook.RegistryParams{}),
			ctx:  context.Background(),
			endpoint: func() webhook.Endpoint {
				e := base
				e.Secret = "   "
				return e
			}(),
			expectedError: errors.New("webhook: invalid endpoint (1 violation(s)): Secret: rule \"required\" on value "),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.r.Upsert(tc.ctx, tc.endpoint)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegistryRemove(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		r             *webhook.Registry
		ctx           context.Context
		id            string
		setup         func(r *webhook.Registry)
		expectedError error
		verify        func(t *testing.T, r *webhook.Registry)
	}

	testCases := []testCase{
		{
			name:          "nil receiver rejected",
			r:             nil,
			ctx:           context.Background(),
			id:            "ep-1",
			setup:         nil,
			expectedError: errors.New("webhook: registry is not initialized"),
			verify:        nil,
		},
		{
			name: "remove non-existent id succeeds",
			r:    webhook.NewRegistry(webhook.RegistryParams{}),
			ctx:  context.Background(),
			id:   "non-existent",
			setup: func(r *webhook.Registry) {
				_ = r.Upsert(context.Background(), webhook.Endpoint{
					ID:     "ep-1",
					Tenant: "t1",
					URL:    "https://m.example.com/hook",
					Events: []string{"transfer.completed.v1"},
					Secret: "s1",
				})
			},
			expectedError: nil,
			verify: func(t *testing.T, r *webhook.Registry) {
				found := r.Find(context.Background(), "t1", "transfer.completed.v1")
				assert.Len(t, found, 1)
			},
		},
		{
			name: "remove existing id deletes endpoint",
			r:    webhook.NewRegistry(webhook.RegistryParams{}),
			ctx:  context.Background(),
			id:   "ep-1",
			setup: func(r *webhook.Registry) {
				_ = r.Upsert(context.Background(), webhook.Endpoint{
					ID:     "ep-1",
					Tenant: "t1",
					URL:    "https://m.example.com/hook",
					Events: []string{"transfer.completed.v1"},
					Secret: "s1",
				})
			},
			expectedError: nil,
			verify: func(t *testing.T, r *webhook.Registry) {
				found := r.Find(context.Background(), "t1", "transfer.completed.v1")
				assert.Empty(t, found)
			},
		},
		{
			name: "remove with whitespace id trims and deletes",
			r:    webhook.NewRegistry(webhook.RegistryParams{}),
			ctx:  context.Background(),
			id:   "  ep-2  ",
			setup: func(r *webhook.Registry) {
				_ = r.Upsert(context.Background(), webhook.Endpoint{
					ID:     "ep-2",
					Tenant: "t1",
					URL:    "https://m.example.com/hook",
					Events: []string{"transfer.completed.v1"},
					Secret: "s1",
				})
			},
			expectedError: nil,
			verify: func(t *testing.T, r *webhook.Registry) {
				found := r.Find(context.Background(), "t1", "transfer.completed.v1")
				assert.Empty(t, found)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(tc.r)
			}
			err := tc.r.Remove(tc.ctx, tc.id)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
			if tc.verify != nil {
				tc.verify(t, tc.r)
			}
		})
	}
}

func TestRegistryFind(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		r              *webhook.Registry
		ctx            context.Context
		tenant         string
		event          string
		setup          func(r *webhook.Registry)
		expectedResult []webhook.Endpoint
	}

	testCases := []testCase{
		{
			name:           "nil receiver returns nil",
			r:              nil,
			ctx:            context.Background(),
			tenant:         "t1",
			event:          "transfer.completed.v1",
			setup:          nil,
			expectedResult: nil,
		},
		{
			name:   "blank tenant returns nil",
			r:      webhook.NewRegistry(webhook.RegistryParams{}),
			ctx:    context.Background(),
			tenant: "",
			event:  "transfer.completed.v1",
			setup: func(r *webhook.Registry) {
				_ = r.Upsert(context.Background(), webhook.Endpoint{
					ID:     "ep-1",
					Tenant: "t1",
					URL:    "https://m.example.com/hook",
					Events: []string{"transfer.completed.v1"},
					Secret: "s1",
				})
			},
			expectedResult: nil,
		},
		{
			name:   "whitespace tenant returns nil",
			r:      webhook.NewRegistry(webhook.RegistryParams{}),
			ctx:    context.Background(),
			tenant: "   ",
			event:  "transfer.completed.v1",
			setup: func(r *webhook.Registry) {
				_ = r.Upsert(context.Background(), webhook.Endpoint{
					ID:     "ep-1",
					Tenant: "t1",
					URL:    "https://m.example.com/hook",
					Events: []string{"transfer.completed.v1"},
					Secret: "s1",
				})
			},
			expectedResult: nil,
		},
		{
			name:   "blank event returns nil",
			r:      webhook.NewRegistry(webhook.RegistryParams{}),
			ctx:    context.Background(),
			tenant: "t1",
			event:  "",
			setup: func(r *webhook.Registry) {
				_ = r.Upsert(context.Background(), webhook.Endpoint{
					ID:     "ep-1",
					Tenant: "t1",
					URL:    "https://m.example.com/hook",
					Events: []string{"transfer.completed.v1"},
					Secret: "s1",
				})
			},
			expectedResult: nil,
		},
		{
			name:   "whitespace event returns nil",
			r:      webhook.NewRegistry(webhook.RegistryParams{}),
			ctx:    context.Background(),
			tenant: "t1",
			event:  "   ",
			setup: func(r *webhook.Registry) {
				_ = r.Upsert(context.Background(), webhook.Endpoint{
					ID:     "ep-1",
					Tenant: "t1",
					URL:    "https://m.example.com/hook",
					Events: []string{"transfer.completed.v1"},
					Secret: "s1",
				})
			},
			expectedResult: nil,
		},
		{
			name:   "cross-tenant lookup returns nothing",
			r:      webhook.NewRegistry(webhook.RegistryParams{}),
			ctx:    context.Background(),
			tenant: "t2",
			event:  "transfer.completed.v1",
			setup: func(r *webhook.Registry) {
				_ = r.Upsert(context.Background(), webhook.Endpoint{
					ID:     "ep-1",
					Tenant: "t1",
					URL:    "https://m.example.com/hook",
					Events: []string{"transfer.completed.v1"},
					Secret: "s1",
				})
			},
			expectedResult: nil,
		},
		{
			name:   "unregistered event returns nothing",
			r:      webhook.NewRegistry(webhook.RegistryParams{}),
			ctx:    context.Background(),
			tenant: "t1",
			event:  "dispute.opened.v1",
			setup: func(r *webhook.Registry) {
				_ = r.Upsert(context.Background(), webhook.Endpoint{
					ID:     "ep-1",
					Tenant: "t1",
					URL:    "https://m.example.com/hook",
					Events: []string{"transfer.completed.v1"},
					Secret: "s1",
				})
			},
			expectedResult: nil,
		},
		{
			name:   "matching tenant and event returns endpoint",
			r:      webhook.NewRegistry(webhook.RegistryParams{}),
			ctx:    context.Background(),
			tenant: "t1",
			event:  "transfer.completed.v1",
			setup: func(r *webhook.Registry) {
				_ = r.Upsert(context.Background(), webhook.Endpoint{
					ID:     "ep-1",
					Tenant: "t1",
					URL:    "https://m.example.com/hook",
					Events: []string{"transfer.completed.v1", "payment.settled.v1"},
					Secret: "s1",
				})
			},
			expectedResult: []webhook.Endpoint{
				{
					ID:     "ep-1",
					Tenant: "t1",
					URL:    "https://m.example.com/hook",
					Events: []string{"transfer.completed.v1", "payment.settled.v1"},
					Secret: "s1",
					Active: true,
				},
			},
		},
		{
			name:   "lookup trims leading and trailing whitespace",
			r:      webhook.NewRegistry(webhook.RegistryParams{}),
			ctx:    context.Background(),
			tenant: "  t1  ",
			event:  "  transfer.completed.v1  ",
			setup: func(r *webhook.Registry) {
				_ = r.Upsert(context.Background(), webhook.Endpoint{
					ID:     "ep-1",
					Tenant: "t1",
					URL:    "https://m.example.com/hook",
					Events: []string{"transfer.completed.v1"},
					Secret: "s1",
				})
			},
			expectedResult: []webhook.Endpoint{
				{
					ID:     "ep-1",
					Tenant: "t1",
					URL:    "https://m.example.com/hook",
					Events: []string{"transfer.completed.v1"},
					Secret: "s1",
					Active: true,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(tc.r)
			}
			actualResult := tc.r.Find(tc.ctx, tc.tenant, tc.event)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestNewRetryPolicy(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		steps            []time.Duration
		expectedAttempts int
		expectedError    error
	}

	testCases := []testCase{
		{
			name:             "valid custom two-step policy",
			steps:            []time.Duration{time.Second, 2 * time.Second},
			expectedAttempts: 3,
			expectedError:    nil,
		},
		{
			name: "valid seven-step policy",
			steps: []time.Duration{
				time.Minute,
				5 * time.Minute,
				15 * time.Minute,
				time.Hour,
				6 * time.Hour,
				24 * time.Hour,
				48 * time.Hour,
			},
			expectedAttempts: 8,
			expectedError:    nil,
		},
		{
			name:             "nil steps rejected",
			steps:            nil,
			expectedAttempts: 0,
			expectedError:    errors.New("webhook: at least one retry step is required"),
		},
		{
			name:             "empty steps slice rejected",
			steps:            []time.Duration{},
			expectedAttempts: 0,
			expectedError:    errors.New("webhook: at least one retry step is required"),
		},
		{
			name:             "zero duration step rejected",
			steps:            []time.Duration{time.Second, 0},
			expectedAttempts: 0,
			expectedError:    errors.New("webhook: retry steps must be positive"),
		},
		{
			name:             "negative step duration rejected",
			steps:            []time.Duration{-time.Second},
			expectedAttempts: 0,
			expectedError:    errors.New("webhook: retry steps must be positive"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			policy, err := webhook.NewRetryPolicy(tc.steps)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				assert.Nil(t, policy)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, policy)
				assert.Equal(t, tc.expectedAttempts, policy.MaxAttempts())
			}
		})
	}
}

func TestRetryPolicyNextDelay(t *testing.T) {
	t.Parallel()

	policy, err := webhook.NewRetryPolicy([]time.Duration{
		time.Minute,
		5 * time.Minute,
		15 * time.Minute,
		time.Hour,
		6 * time.Hour,
		24 * time.Hour,
		48 * time.Hour,
	})
	require.NoError(t, err)

	type testCase struct {
		name          string
		p             *webhook.RetryPolicy
		attempt       int
		expectedDelay time.Duration
		expectedOk    bool
	}

	testCases := []testCase{
		{
			name:          "nil receiver returns false",
			p:             nil,
			attempt:       0,
			expectedDelay: 0,
			expectedOk:    false,
		},
		{
			name:          "negative attempt returns false",
			p:             policy,
			attempt:       -1,
			expectedDelay: 0,
			expectedOk:    false,
		},
		{
			name:          "first retry attempt (attempt 0) is one minute",
			p:             policy,
			attempt:       0,
			expectedDelay: 1 * time.Minute,
			expectedOk:    true,
		},
		{
			name:          "second retry attempt (attempt 1) is five minutes",
			p:             policy,
			attempt:       1,
			expectedDelay: 5 * time.Minute,
			expectedOk:    true,
		},
		{
			name:          "third retry attempt (attempt 2) is fifteen minutes",
			p:             policy,
			attempt:       2,
			expectedDelay: 15 * time.Minute,
			expectedOk:    true,
		},
		{
			name:          "attempt 6 is 48 hours",
			p:             policy,
			attempt:       6,
			expectedDelay: 48 * time.Hour,
			expectedOk:    true,
		},
		{
			name:          "attempt 7 exhausts policy",
			p:             policy,
			attempt:       7,
			expectedDelay: 0,
			expectedOk:    false,
		},
		{
			name:          "large attempt value exhausts policy",
			p:             policy,
			attempt:       100,
			expectedDelay: 0,
			expectedOk:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualDelay, actualOk := tc.p.NextDelay(tc.attempt)
			assert.Equal(t, tc.expectedDelay, actualDelay)
			assert.Equal(t, tc.expectedOk, actualOk)
		})
	}
}

func TestRetryPolicyProperties(t *testing.T) {
	t.Parallel()

	sevenStepPolicy, err := webhook.NewRetryPolicy([]time.Duration{
		time.Minute,
		5 * time.Minute,
		15 * time.Minute,
		time.Hour,
		6 * time.Hour,
		24 * time.Hour,
		48 * time.Hour,
	})
	require.NoError(t, err)

	twoStepPolicy, err := webhook.NewRetryPolicy([]time.Duration{
		time.Second,
		2 * time.Second,
	})
	require.NoError(t, err)

	type testCase struct {
		name             string
		p                *webhook.RetryPolicy
		expectedRetries  int
		expectedAttempts int
		expectedHorizon  time.Duration
		expectedSchedule []time.Duration
	}

	testCases := []testCase{
		{
			name:             "nil receiver safe defaults",
			p:                nil,
			expectedRetries:  0,
			expectedAttempts: 1,
			expectedHorizon:  0,
			expectedSchedule: nil,
		},
		{
			name:             "seven step policy properties",
			p:                sevenStepPolicy,
			expectedRetries:  7,
			expectedAttempts: 8,
			expectedHorizon:  79*time.Hour + 21*time.Minute,
			expectedSchedule: []time.Duration{
				time.Minute,
				5 * time.Minute,
				15 * time.Minute,
				time.Hour,
				6 * time.Hour,
				24 * time.Hour,
				48 * time.Hour,
			},
		},
		{
			name:             "two step policy properties",
			p:                twoStepPolicy,
			expectedRetries:  2,
			expectedAttempts: 3,
			expectedHorizon:  3 * time.Second,
			expectedSchedule: []time.Duration{
				time.Second,
				2 * time.Second,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedRetries, tc.p.MaxRetries())
			assert.Equal(t, tc.expectedAttempts, tc.p.MaxAttempts())
			assert.Equal(t, tc.expectedHorizon, tc.p.Horizon())
			assert.Equal(t, tc.expectedSchedule, tc.p.Schedule())
		})
	}
}

func TestRetryableError(t *testing.T) {
	t.Parallel()

	causeErr := errors.New("underlying network timeout")

	type testCase struct {
		name            string
		err             *webhook.RetryableError
		expectedMessage string
		expectedUnwrap  error
	}

	testCases := []testCase{
		{
			name:            "nil receiver safe error",
			err:             nil,
			expectedMessage: "webhook: retryable delivery",
			expectedUnwrap:  nil,
		},
		{
			name: "error with endpoint attempt nextdelay and cause",
			err: &webhook.RetryableError{
				EndpointID: "ep-101",
				Attempt:    2,
				NextDelay:  5 * time.Minute,
				Cause:      causeErr,
			},
			expectedMessage: "webhook: retry endpoint ep-101 attempt 2 after 5m0s",
			expectedUnwrap:  causeErr,
		},
		{
			name: "error without cause",
			err: &webhook.RetryableError{
				EndpointID: "ep-202",
				Attempt:    0,
				NextDelay:  1 * time.Minute,
				Cause:      nil,
			},
			expectedMessage: "webhook: retry endpoint ep-202 attempt 0 after 1m0s",
			expectedUnwrap:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedMessage, tc.err.Error())
			assert.Equal(t, tc.expectedUnwrap, tc.err.Unwrap())
		})
	}
}

func TestWebhookDLQRecord(t *testing.T) {
	t.Parallel()

	baseMsg := webhook.DLQMessage{
		EndpointID: "ep-1",
		Tenant:     "t1",
		Event:      "transfer.completed.v1",
		Payload:    []byte(`{"id":"e-100"}`),
		Attempts:   7,
	}

	type testCase struct {
		name          string
		dlq           *fakes.WebhookDLQ
		ctx           context.Context
		msg           webhook.DLQMessage
		expectedError error
		verify        func(t *testing.T, dlq *fakes.WebhookDLQ)
	}

	testCases := []testCase{
		{
			name:          "valid record stored and retrievable",
			dlq:           fakes.NewWebhookDLQ(),
			ctx:           context.Background(),
			msg:           baseMsg,
			expectedError: nil,
			verify: func(t *testing.T, dlq *fakes.WebhookDLQ) {
				assert.Equal(t, 1, dlq.Len())
				list := dlq.List()
				require.Len(t, list, 1)
				assert.Equal(t, "ep-1", list[0].EndpointID)
				assert.Equal(t, "t1", list[0].Tenant)
				assert.Equal(t, "transfer.completed.v1", list[0].Event)
				assert.Equal(t, []byte(`{"id":"e-100"}`), list[0].Payload)
				assert.Equal(t, 7, list[0].Attempts)
			},
		},
		{
			name: "blank endpoint id rejected",
			dlq:  fakes.NewWebhookDLQ(),
			ctx:  context.Background(),
			msg: func() webhook.DLQMessage {
				m := baseMsg
				m.EndpointID = ""
				return m
			}(),
			expectedError: errors.New("webhook: endpoint id is required"),
			verify: func(t *testing.T, dlq *fakes.WebhookDLQ) {
				assert.Equal(t, 0, dlq.Len())
			},
		},
		{
			name: "whitespace endpoint id rejected",
			dlq:  fakes.NewWebhookDLQ(),
			ctx:  context.Background(),
			msg: func() webhook.DLQMessage {
				m := baseMsg
				m.EndpointID = "   "
				return m
			}(),
			expectedError: errors.New("webhook: endpoint id is required"),
			verify: func(t *testing.T, dlq *fakes.WebhookDLQ) {
				assert.Equal(t, 0, dlq.Len())
			},
		},
		{
			name: "nil payload rejected",
			dlq:  fakes.NewWebhookDLQ(),
			ctx:  context.Background(),
			msg: func() webhook.DLQMessage {
				m := baseMsg
				m.Payload = nil
				return m
			}(),
			expectedError: errors.New("webhook: payload is required"),
			verify: func(t *testing.T, dlq *fakes.WebhookDLQ) {
				assert.Equal(t, 0, dlq.Len())
			},
		},
		{
			name: "blank tenant allowed and stored",
			dlq:  fakes.NewWebhookDLQ(),
			ctx:  context.Background(),
			msg: func() webhook.DLQMessage {
				m := baseMsg
				m.Tenant = ""
				return m
			}(),
			expectedError: nil,
			verify: func(t *testing.T, dlq *fakes.WebhookDLQ) {
				assert.Equal(t, 1, dlq.Len())
			},
		},
		{
			name: "payload is deeply copied and isolated",
			dlq:  fakes.NewWebhookDLQ(),
			ctx:  context.Background(),
			msg: func() webhook.DLQMessage {
				buf := []byte(`{"id":"immutable"}`)
				m := baseMsg
				m.Payload = buf
				return m
			}(),
			expectedError: nil,
			verify: func(t *testing.T, dlq *fakes.WebhookDLQ) {
				list := dlq.List()
				require.Len(t, list, 1)
				// Modify the returned payload
				list[0].Payload[0] = 'X'
				// Next List() should remain unaffected because List copies the slice
				list2 := dlq.List()
				assert.Equal(t, byte('{'), list2[0].Payload[0])
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.dlq.Record(tc.ctx, tc.msg)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
			if tc.verify != nil {
				tc.verify(t, tc.dlq)
			}
		})
	}
}
