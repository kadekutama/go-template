package featureflag_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/infrastructure/featureflag"
)

func TestFallbackResolve(t *testing.T) {
	t.Parallel()

	now := time.Now()

	type testCase struct {
		name            string
		observed        map[string]bool
		defaults        map[string]bool
		advance         time.Duration
		key             string
		def             bool
		expectedResult  bool
		expectedResult1 bool
		expectedResult2 bool
	}

	testCases := []testCase{
		{
			name:            "unconfigured key returns call default fresh",
			observed:        map[string]bool{},
			defaults:        nil,
			advance:         0,
			key:             "payMethod.card.enabled",
			def:             false,
			expectedResult:  false,
			expectedResult1: false,
			expectedResult2: false,
		},
		{
			name:            "configured default wins over call default",
			observed:        map[string]bool{},
			defaults:        map[string]bool{"payMethod.card.enabled": true},
			advance:         0,
			key:             "payMethod.card.enabled",
			def:             false,
			expectedResult:  true,
			expectedResult1: false,
			expectedResult2: false,
		},
		{
			name:            "last known wins over configured default when fresh",
			observed:        map[string]bool{"payMethod.card.enabled": false},
			defaults:        map[string]bool{"payMethod.card.enabled": true},
			advance:         time.Second,
			key:             "payMethod.card.enabled",
			def:             true,
			expectedResult:  false,
			expectedResult1: true,
			expectedResult2: false,
		},
		{
			name:            "last known past refresh bound is stale",
			observed:        map[string]bool{"payMethod.card.enabled": true},
			defaults:        nil,
			advance:         11 * time.Second,
			key:             "payMethod.card.enabled",
			def:             false,
			expectedResult:  true,
			expectedResult1: true,
			expectedResult2: true,
		},
		{
			name:            "last known at refresh bound is fresh",
			observed:        map[string]bool{"kill.switch": true},
			defaults:        nil,
			advance:         10 * time.Second,
			key:             "kill.switch",
			def:             false,
			expectedResult:  true,
			expectedResult1: true,
			expectedResult2: false,
		},
		{
			name:            "empty key returns call default",
			observed:        map[string]bool{"kill.switch": true},
			defaults:        nil,
			advance:         0,
			key:             "",
			def:             true,
			expectedResult:  true,
			expectedResult1: false,
			expectedResult2: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			clock := testClock(now, now.Add(tc.advance))
			fallback, err := featureflag.NewFallback(featureflag.FallbackParams{
				Clock:           clock,
				RefreshInterval: 10 * time.Second,
				CacheTTL:        time.Hour,
				Defaults:        tc.defaults,
			})
			require.NoError(t, err)

			ctx := context.Background()
			for key, value := range tc.observed {
				fallback.Observe(ctx, key, value)
			}

			actualResult, actualResult1, actualResult2 := fallback.Resolve(tc.key, tc.def)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedResult1, actualResult1)
			assert.Equal(t, tc.expectedResult2, actualResult2)
		})
	}
}

func TestFallbackObserveOverrides(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		first           bool
		second          bool
		expectedResult  bool
		expectedResult1 bool
		expectedResult2 bool
	}

	testCases := []testCase{
		{
			name:            "second observation wins",
			first:           false,
			second:          true,
			expectedResult:  true,
			expectedResult1: true,
			expectedResult2: false,
		},
		{
			name:            "flip back to false wins",
			first:           true,
			second:          false,
			expectedResult:  false,
			expectedResult1: true,
			expectedResult2: false,
		},
		{
			name:            "same value stays fresh",
			first:           true,
			second:          true,
			expectedResult:  true,
			expectedResult1: true,
			expectedResult2: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			clock := testClock(time.Now())
			fallback, err := featureflag.NewFallback(featureflag.FallbackParams{
				Clock:           clock,
				RefreshInterval: 10 * time.Second,
				CacheTTL:        time.Hour,
			})
			require.NoError(t, err)

			ctx := context.Background()
			fallback.Observe(ctx, "payMethod.card.enabled", tc.first)
			fallback.Observe(ctx, "payMethod.card.enabled", tc.second)

			actualResult, actualResult1, actualResult2 := fallback.Resolve("payMethod.card.enabled", false)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedResult1, actualResult1)
			assert.Equal(t, tc.expectedResult2, actualResult2)
		})
	}
}

func TestFallbackCacheBestEffort(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		cacheSetErr     error
		key             string
		value           bool
		expectedResult  bool
		expectedResult1 bool
		expectedResult2 bool
	}

	testCases := []testCase{
		{
			name:            "cache write failure still records memory",
			cacheSetErr:     errCacheMiss,
			key:             "payMethod.card.enabled",
			value:           true,
			expectedResult:  true,
			expectedResult1: true,
			expectedResult2: false,
		},
		{
			name:            "cache success records memory",
			cacheSetErr:     nil,
			key:             "currency.EUR.enabled",
			value:           false,
			expectedResult:  false,
			expectedResult1: true,
			expectedResult2: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			clock := testClock(time.Now())
			cache := newMockCache(map[string][]byte{}, tc.cacheSetErr)
			fallback, err := featureflag.NewFallback(featureflag.FallbackParams{
				Clock:           clock,
				RefreshInterval: 10 * time.Second,
				CacheTTL:        time.Hour,
				Cache:           cache,
			})
			require.NoError(t, err)

			fallback.Observe(context.Background(), tc.key, tc.value)

			actualResult, actualResult1, actualResult2 := fallback.Resolve(tc.key, !tc.value)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedResult1, actualResult1)
			assert.Equal(t, tc.expectedResult2, actualResult2)
		})
	}
}

func TestFallbackResolveWithContext(t *testing.T) {
	t.Parallel()

	now := time.Now()

	type testCase struct {
		name            string
		cached          map[string][]byte
		key             string
		def             bool
		expectedResult  bool
		expectedResult1 bool
		expectedResult2 bool
	}

	testCases := []testCase{
		{
			name:            "l2 hit serves last known marked stale",
			cached:          map[string][]byte{"flag:payMethod.card.enabled": {1}},
			key:             "payMethod.card.enabled",
			def:             false,
			expectedResult:  true,
			expectedResult1: true,
			expectedResult2: true,
		},
		{
			name:            "l2 miss falls through to call default",
			cached:          map[string][]byte{},
			key:             "payMethod.card.enabled",
			def:             true,
			expectedResult:  true,
			expectedResult1: false,
			expectedResult2: false,
		},
		{
			name:            "malformed l2 entry falls through to call default",
			cached:          map[string][]byte{"flag:payMethod.card.enabled": {7}},
			key:             "payMethod.card.enabled",
			def:             false,
			expectedResult:  false,
			expectedResult1: false,
			expectedResult2: false,
		},
		{
			name:            "empty key returns call default",
			cached:          map[string][]byte{"flag:": {1}},
			key:             "",
			def:             true,
			expectedResult:  true,
			expectedResult1: false,
			expectedResult2: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cache := newMockCache(tc.cached, nil)
			fallback, err := featureflag.NewFallback(featureflag.FallbackParams{
				Clock:           testClock(now),
				RefreshInterval: 10 * time.Second,
				CacheTTL:        time.Hour,
				Cache:           cache,
			})
			require.NoError(t, err)

			actualResult, actualResult1, actualResult2 := fallback.ResolveWithContext(context.Background(), tc.key, tc.def)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedResult1, actualResult1)
			assert.Equal(t, tc.expectedResult2, actualResult2)
		})
	}
}

func TestFallbackCacheTTLHonored(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name        string
		cacheTTL    time.Duration
		expectedTTL time.Duration
	}

	testCases := []testCase{
		{
			name:        "custom ttl reaches the shared cache",
			cacheTTL:    30 * time.Minute,
			expectedTTL: 30 * time.Minute,
		},
		{
			name:        "hour ttl reaches the shared cache",
			cacheTTL:    time.Hour,
			expectedTTL: time.Hour,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cache := newMockCache(map[string][]byte{}, nil)
			fallback, err := featureflag.NewFallback(featureflag.FallbackParams{
				Clock:           testClock(time.Now()),
				RefreshInterval: 10 * time.Second,
				Cache:           cache,
				CacheTTL:        tc.cacheTTL,
			})
			require.NoError(t, err)

			fallback.Observe(context.Background(), "payMethod.card.enabled", true)

			actualTTL, _ := cacheSetTTL(cache, "flag:payMethod.card.enabled")
			assert.Equal(t, tc.expectedTTL, actualTTL)
		})
	}
}
