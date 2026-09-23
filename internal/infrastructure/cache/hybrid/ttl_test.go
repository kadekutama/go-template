package hybrid_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/infrastructure/cache/hybrid"
)

func TestTTLSetWithDefaults(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		ttlSet         hybrid.TTLSet
		expectedResult hybrid.TTLSet
		expectedError  error
	}

	testCases := []testCase{
		{
			name:   "zero set keeps contract defaults",
			ttlSet: hybrid.TTLSet{},
			expectedResult: hybrid.TTLSet{
				Balance:         time.Minute,
				BalanceCursor:   5 * time.Minute,
				Config:          5 * time.Minute,
				ConfigLong:      30 * time.Minute,
				FX:              time.Hour,
				IdempotencyHint: 24 * time.Hour,
				RateLimit:       time.Minute,
				L1Populate:      time.Minute,
			},
			expectedError: nil,
		},
		{
			name: "partial config overlays defaults",
			ttlSet: hybrid.TTLSet{
				Balance: 2 * time.Minute,
			},
			expectedResult: func() hybrid.TTLSet {
				ttl := hybrid.DefaultTTLs()
				ttl.Balance = 2 * time.Minute
				return ttl
			}(),
			expectedError: nil,
		},
		{
			name: "negative explicit value is an error, not a silent default",
			ttlSet: hybrid.TTLSet{
				FX: -time.Second,
			},
			expectedResult: hybrid.TTLSet{},
			expectedError:  errors.New("cache: ttl fx must not be negative"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := tc.ttlSet.WithDefaults()
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tc.expectedError.Error())
				assert.Equal(t, tc.expectedResult, actualResult)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, actualResult)
			require.NoError(t, actualResult.Validate())
		})
	}
}

func TestTTLSetValidate(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		ttlSet        hybrid.TTLSet
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "defaults validate",
			ttlSet:        hybrid.DefaultTTLs(),
			expectedError: nil,
		},
		{
			name:          "zero set rejected",
			ttlSet:        hybrid.TTLSet{},
			expectedError: errors.New("cache: ttl balance must be positive"),
		},
		{
			name: "negative bound rejected",
			ttlSet: func() hybrid.TTLSet {
				ttl := hybrid.DefaultTTLs()
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
