package local_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/infrastructure/cache/local"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/store"
)

// Compile-time seam conformance for the adapter under test.
var _ store.Store = (*local.OtterCache)(nil)

func mustStore(t *testing.T) *local.OtterCache {
	t.Helper()

	cache, err := local.NewOtterCache(local.OtterParams{MaximumWeight: 1 << 20})
	require.NoError(t, err)

	return cache
}

func TestNewOtterCache(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		params         local.OtterParams
		expectedWeight uint64
		expectedTTL    time.Duration
	}

	testCases := []testCase{
		{
			name: "explicit sizing",
			params: local.OtterParams{
				MaximumWeight: 1024,
				DefaultTTL:    2 * time.Minute,
			},
			expectedWeight: 1024,
			expectedTTL:    2 * time.Minute,
		},
		{
			name:           "zero selects defaults",
			params:         local.OtterParams{},
			expectedWeight: local.DefaultMaximumWeight,
			expectedTTL:    local.DefaultTTL,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cache, err := local.NewOtterCache(tc.params)
			require.NoError(t, err)
			require.NotNil(t, cache)
			assert.Equal(t, tc.expectedWeight, cache.MaximumWeight())
			assert.Equal(t, tc.expectedTTL, cache.DefaultTTLValue())
		})
	}
}

func TestOtterCacheGet(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		cache          func() *local.OtterCache
		ctx            context.Context
		key            string
		expectedResult []byte
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "valid hit returns payload copy and isolates caller mutations",
			cache: func() *local.OtterCache {
				c := mustStore(t)
				val := []byte(`{"minor":100}`)
				_ = c.Set(context.Background(), "balance:t1:a1:USD", val, time.Minute)
				val[0] = 'X' // caller mutation should not affect cached data
				return c
			},
			ctx:            context.Background(),
			key:            "balance:t1:a1:USD",
			expectedResult: []byte(`{"minor":100}`),
			expectedError:  nil,
		},
		{
			name: "missing key returns store.ErrMiss",
			cache: func() *local.OtterCache {
				return mustStore(t)
			},
			ctx:            context.Background(),
			key:            "balance:t1:absent:USD",
			expectedResult: nil,
			expectedError:  store.ErrMiss,
		},
		{
			name: "nil value returns nil payload without error",
			cache: func() *local.OtterCache {
				c := mustStore(t)
				_ = c.Set(context.Background(), "negative-marker", nil, time.Minute)
				return c
			},
			ctx:            context.Background(),
			key:            "negative-marker",
			expectedResult: nil,
			expectedError:  nil,
		},
		{
			name: "empty key rejected",
			cache: func() *local.OtterCache {
				return mustStore(t)
			},
			ctx:            context.Background(),
			key:            "",
			expectedResult: nil,
			expectedError:  errors.New("otter: key is required"),
		},
		{
			name: "uninitialized receiver returns error",
			cache: func() *local.OtterCache {
				return nil
			},
			ctx:            context.Background(),
			key:            "any-key",
			expectedResult: nil,
			expectedError:  errors.New("otter: cache is not initialized"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.cache()
			actualResult, err := c.Get(tc.ctx, tc.key)
			if tc.expectedError != nil {
				if errors.Is(tc.expectedError, store.ErrMiss) {
					assert.ErrorIs(t, err, store.ErrMiss)
				} else {
					assert.EqualError(t, err, tc.expectedError.Error())
				}
				assert.Nil(t, actualResult)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestOtterCacheSet(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		cache         func() *local.OtterCache
		ctx           context.Context
		key           string
		value         []byte
		ttl           time.Duration
		expectedError error
	}

	testCases := []testCase{
		{
			name: "valid set stores payload copy and is synchronously visible",
			cache: func() *local.OtterCache {
				return mustStore(t)
			},
			ctx:           context.Background(),
			key:           "balance:t1:a1:USD",
			value:         []byte(`{"minor":100}`),
			ttl:           time.Minute,
			expectedError: nil,
		},
		{
			name: "nil value stores negative marker",
			cache: func() *local.OtterCache {
				return mustStore(t)
			},
			ctx:           context.Background(),
			key:           "negative-key",
			value:         nil,
			ttl:           time.Minute,
			expectedError: nil,
		},
		{
			name: "empty key rejected",
			cache: func() *local.OtterCache {
				return mustStore(t)
			},
			ctx:           context.Background(),
			key:           "",
			value:         []byte(`{"minor":100}`),
			ttl:           time.Minute,
			expectedError: errors.New("otter: key is required"),
		},
		{
			name: "zero ttl rejected",
			cache: func() *local.OtterCache {
				return mustStore(t)
			},
			ctx:           context.Background(),
			key:           "k",
			value:         []byte(`v`),
			ttl:           0,
			expectedError: errors.New("otter: ttl must be positive"),
		},
		{
			name: "negative ttl rejected",
			cache: func() *local.OtterCache {
				return mustStore(t)
			},
			ctx:           context.Background(),
			key:           "k",
			value:         []byte(`v`),
			ttl:           -time.Second,
			expectedError: errors.New("otter: ttl must be positive"),
		},
		{
			name: "uninitialized receiver returns error",
			cache: func() *local.OtterCache {
				return nil
			},
			ctx:           context.Background(),
			key:           "k",
			value:         []byte(`v`),
			ttl:           time.Minute,
			expectedError: errors.New("otter: cache is not initialized"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.cache()
			err := c.Set(tc.ctx, tc.key, tc.value, tc.ttl)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				return
			}
			require.NoError(t, err)

			// Verify synchronous visibility
			got, err := c.Get(tc.ctx, tc.key)
			require.NoError(t, err)
			assert.Equal(t, tc.value, got)
		})
	}
}

func TestOtterCacheDelete(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		cache         func() *local.OtterCache
		ctx           context.Context
		key           string
		expectedError error
	}

	testCases := []testCase{
		{
			name: "existing key deleted successfully",
			cache: func() *local.OtterCache {
				c := mustStore(t)
				_ = c.Set(context.Background(), "del-key", []byte(`data`), time.Minute)
				return c
			},
			ctx:           context.Background(),
			key:           "del-key",
			expectedError: nil,
		},
		{
			name: "missing key succeeds idempotently",
			cache: func() *local.OtterCache {
				return mustStore(t)
			},
			ctx:           context.Background(),
			key:           "non-existent-key",
			expectedError: nil,
		},
		{
			name: "empty key rejected",
			cache: func() *local.OtterCache {
				return mustStore(t)
			},
			ctx:           context.Background(),
			key:           "",
			expectedError: errors.New("otter: key is required"),
		},
		{
			name: "uninitialized receiver returns error",
			cache: func() *local.OtterCache {
				return nil
			},
			ctx:           context.Background(),
			key:           "any-key",
			expectedError: errors.New("otter: cache is not initialized"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.cache()
			err := c.Delete(tc.ctx, tc.key)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				return
			}
			require.NoError(t, err)

			// After deletion, Get should report a miss
			_, getErr := c.Get(tc.ctx, tc.key)
			assert.ErrorIs(t, getErr, store.ErrMiss)
		})
	}
}

func TestOtterCacheExpiry(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		ctx            context.Context
		key            string
		value          []byte
		ttl            time.Duration
		sleep          time.Duration
		expectedResult []byte
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "unexpired key returns value",
			ctx:            context.Background(),
			key:            "unexpired-key",
			value:          []byte(`fresh`),
			ttl:            500 * time.Millisecond,
			sleep:          0,
			expectedResult: []byte(`fresh`),
			expectedError:  nil,
		},
		{
			name:           "expired key returns miss",
			ctx:            context.Background(),
			key:            "expired-key",
			value:          []byte(`stale`),
			ttl:            20 * time.Millisecond,
			sleep:          60 * time.Millisecond,
			expectedResult: nil,
			expectedError:  store.ErrMiss,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cache := mustStore(t)
			require.NoError(t, cache.Set(tc.ctx, tc.key, tc.value, tc.ttl))

			if tc.sleep > 0 {
				time.Sleep(tc.sleep)
			}

			actualResult, err := cache.Get(tc.ctx, tc.key)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				assert.Nil(t, actualResult)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestOtterCacheClose(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		cache         func() *local.OtterCache
		expectedError error
		expectedSize  int
	}

	testCases := []testCase{
		{
			name: "active cache closes cleanly and idempotently",
			cache: func() *local.OtterCache {
				c := mustStore(t)
				_ = c.Set(context.Background(), "k", []byte(`v`), time.Minute)
				return c
			},
			expectedError: nil,
			expectedSize:  1,
		},
		{
			name: "nil cache closes safely without panic",
			cache: func() *local.OtterCache {
				return nil
			},
			expectedError: nil,
			expectedSize:  0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.cache()
			err := c.Close()
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				return
			}
			require.NoError(t, err)

			// Idempotent second close
			require.NoError(t, c.Close())
			assert.Equal(t, tc.expectedSize, c.Size())
		})
	}
}
