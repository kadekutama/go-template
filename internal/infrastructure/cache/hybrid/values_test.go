package hybrid_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/infrastructure/cache/hybrid"
	"github.com/kadekutama/go-template/test/fakes"
)

// valuesBalance is a structured value for the kind-check table.
type valuesBalance struct {
	Minor int64
}

// TestHybridCacheValueKinds proves the Set-time type check on typed views:
// non-cacheable Go kinds are rejected with the sentinel, while nil values of
// nil-able kinds remain legal negative-cache markers.
func TestHybridCacheValueKinds(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		value         any
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "function rejected",
			value:         func() {},
			expectedError: hybrid.ErrUnsupportedValueType,
		},
		{
			name:          "channel rejected",
			value:         make(chan int),
			expectedError: hybrid.ErrUnsupportedValueType,
		},
		{
			name:          "complex rejected",
			value:         complex(1, 2),
			expectedError: hybrid.ErrUnsupportedValueType,
		},
		{
			name:          "nil interface allowed",
			value:         nil,
			expectedError: nil,
		},
		{
			name:          "nil slice allowed",
			value:         []string(nil),
			expectedError: nil,
		},
		{
			name:          "nil map allowed",
			value:         map[string]int(nil),
			expectedError: nil,
		},
		{
			name:          "typed nil pointer allowed",
			value:         (*int)(nil),
			expectedError: nil,
		},
		{
			name:          "nil interface holding function rejected",
			value:         any(func() {}),
			expectedError: hybrid.ErrUnsupportedValueType,
		},
		{
			name:          "plain string allowed",
			value:         "value",
			expectedError: nil,
		},
		{
			name:          "struct allowed",
			value:         valuesBalance{Minor: 7},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			engine, err := hybrid.New(hybrid.Params{L1: fakes.NewCacheStore()})
			require.NoError(t, err)

			view := hybrid.Typed[any](engine)
			ctx := context.Background()

			err = view.Set(ctx, "k", tc.value, time.Minute)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)

				_, getErr := engine.Get(ctx, "k")
				assert.ErrorIs(t, getErr, hybrid.ErrCacheMiss)

				return
			}

			require.NoError(t, err)

			_, err = view.Get(ctx, "k")
			require.NoError(t, err)
		})
	}
}

func TestTypedCacheDelete(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		view          func(engine *hybrid.Cache) *hybrid.TypedCache[valuesBalance]
		seed          bool
		expectedError error
	}

	testCases := []testCase{
		{
			name: "delete removes the typed entry",
			view: func(engine *hybrid.Cache) *hybrid.TypedCache[valuesBalance] {
				return hybrid.Typed[valuesBalance](engine)
			},
			seed:          true,
			expectedError: nil,
		},
		{
			name: "delete missing key succeeds",
			view: func(engine *hybrid.Cache) *hybrid.TypedCache[valuesBalance] {
				return hybrid.Typed[valuesBalance](engine)
			},
			seed:          false,
			expectedError: nil,
		},
		{
			name: "nil engine view reports not initialized",
			view: func(_ *hybrid.Cache) *hybrid.TypedCache[valuesBalance] {
				return hybrid.Typed[valuesBalance](nil)
			},
			seed:          false,
			expectedError: hybrid.ErrNotInitialized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			engine, err := hybrid.New(hybrid.Params{L1: fakes.NewCacheStore()})
			require.NoError(t, err)

			view := tc.view(engine)
			ctx := context.Background()

			if tc.seed {
				require.NoError(t, view.Set(ctx, "k", valuesBalance{Minor: 7}, time.Minute))
			}

			err = view.Delete(ctx, "k")
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				return
			}
			require.NoError(t, err)

			_, err = view.Get(ctx, "k")
			assert.ErrorIs(t, err, hybrid.ErrCacheMiss)
		})
	}
}

// unencodable carries a channel so the codec fails at encode time even
// though the struct kind itself passes the value check.
type unencodable struct {
	Ch chan int `json:"ch"`
}

func TestTypedCacheCodecFailures(t *testing.T) {
	t.Parallel()

	t.Run("encode failure surfaces with key context", func(t *testing.T) {
		engine, err := hybrid.New(hybrid.Params{L1: fakes.NewCacheStore()})
		require.NoError(t, err)

		view := hybrid.Typed[unencodable](engine)

		err = view.Set(context.Background(), "k", unencodable{Ch: make(chan int)}, time.Minute)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cache: encode k:")
	})

	t.Run("decode failure surfaces with key context", func(t *testing.T) {
		l1 := fakes.NewCacheStore()
		engine, err := hybrid.New(hybrid.Params{L1: l1})
		require.NoError(t, err)

		// Seed the shared bytes layer with garbage no codec can decode.
		require.NoError(t, l1.Set(context.Background(), "k", []byte(`not-json{{`), time.Minute))

		view := hybrid.Typed[valuesBalance](engine)

		_, err = view.Get(context.Background(), "k")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cache: decode k:")
	})
}
