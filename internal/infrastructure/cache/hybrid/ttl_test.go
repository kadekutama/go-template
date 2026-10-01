package hybrid_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/infrastructure/cache/hybrid"
)

func TestTTLSetValidate(t *testing.T) {
	t.Parallel()

	validSet := hybrid.TTLSet{
		Balance:         time.Minute,
		BalanceCursor:   5 * time.Minute,
		Config:          5 * time.Minute,
		ConfigLong:      30 * time.Minute,
		FX:              time.Hour,
		IdempotencyHint: 24 * time.Hour,
		RateLimit:       time.Minute,
		L1Populate:      time.Minute,
	}

	type testCase struct {
		name          string
		ttlSet        hybrid.TTLSet
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid set validates",
			ttlSet:        validSet,
			expectedError: nil,
		},
		{
			name:          "zero set rejected",
			ttlSet:        hybrid.TTLSet{},
			expectedError: errors.New("cache: ttl balance must be positive"),
		},
		{
			name: "missing balance cursor rejected",
			ttlSet: func() hybrid.TTLSet {
				ttl := validSet
				ttl.BalanceCursor = 0
				return ttl
			}(),
			expectedError: errors.New("cache: ttl balance_cursor must be positive"),
		},
		{
			name: "missing config rejected",
			ttlSet: func() hybrid.TTLSet {
				ttl := validSet
				ttl.Config = 0
				return ttl
			}(),
			expectedError: errors.New("cache: ttl config must be positive"),
		},
		{
			name: "missing config long rejected",
			ttlSet: func() hybrid.TTLSet {
				ttl := validSet
				ttl.ConfigLong = 0
				return ttl
			}(),
			expectedError: errors.New("cache: ttl config_long must be positive"),
		},
		{
			name: "missing fx rejected",
			ttlSet: func() hybrid.TTLSet {
				ttl := validSet
				ttl.FX = 0
				return ttl
			}(),
			expectedError: errors.New("cache: ttl fx must be positive"),
		},
		{
			name: "missing idempotency hint rejected",
			ttlSet: func() hybrid.TTLSet {
				ttl := validSet
				ttl.IdempotencyHint = 0
				return ttl
			}(),
			expectedError: errors.New("cache: ttl idempotency_hint must be positive"),
		},
		{
			name: "missing rate limit rejected",
			ttlSet: func() hybrid.TTLSet {
				ttl := validSet
				ttl.RateLimit = 0
				return ttl
			}(),
			expectedError: errors.New("cache: ttl rate_limit must be positive"),
		},
		{
			name: "missing l1 populate rejected",
			ttlSet: func() hybrid.TTLSet {
				ttl := validSet
				ttl.L1Populate = 0
				return ttl
			}(),
			expectedError: errors.New("cache: ttl l1_populate must be positive"),
		},
		{
			name: "negative bound rejected",
			ttlSet: func() hybrid.TTLSet {
				ttl := validSet
				ttl.RateLimit = -time.Second
				return ttl
			}(),
			expectedError: errors.New("cache: ttl rate_limit must be positive"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.ttlSet.Validate()
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tc.expectedError.Error())
				return
			}
			require.NoError(t, err)
		})
	}
}
