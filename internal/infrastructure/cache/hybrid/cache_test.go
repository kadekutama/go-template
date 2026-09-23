package hybrid_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/hybrid"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/store"
	"github.com/kadekutama/go-template/test/fakes"
)

func TestNew(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		params         hybrid.Params
		expectedResult bool
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "valid params with l1 only",
			params: hybrid.Params{
				L1: fakes.NewCacheStore(),
				L2: nil,
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "valid params with l1 and l2",
			params: hybrid.Params{
				L1: fakes.NewCacheStore(),
				L2: fakes.NewCacheStore(),
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "nil l1 rejected",
			params: hybrid.Params{
				L1: nil,
				L2: nil,
			},
			expectedResult: false,
			expectedError:  hybrid.ErrL1Required,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			engine, err := hybrid.New(tc.params)
			assert.ErrorIs(t, err, tc.expectedError)
			assert.Equal(t, tc.expectedResult, engine != nil)
		})
	}
}

func TestEnginePortConformance(t *testing.T) {
	t.Parallel()

	var _ appport.Cache = (*hybrid.Cache)(nil)
}

func TestHybridCacheSet(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		engine        *hybrid.Cache
		ctx           context.Context
		key           string
		value         []byte
		ttl           time.Duration
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "nil receiver rejected",
			engine:        nil,
			ctx:           context.Background(),
			key:           "balance:t1:a1:USD",
			value:         []byte(`{"minor":42}`),
			ttl:           time.Minute,
			expectedError: hybrid.ErrNotInitialized,
		},
		{
			name:          "uninitialized engine rejected",
			engine:        &hybrid.Cache{},
			ctx:           context.Background(),
			key:           "balance:t1:a1:USD",
			value:         []byte(`{"minor":42}`),
			ttl:           time.Minute,
			expectedError: hybrid.ErrNotInitialized,
		},
		{
			name: "valid set both layers",
			engine: func() *hybrid.Cache {
				e, err := hybrid.New(hybrid.Params{
					L1: fakes.NewCacheStore(),
					L2: fakes.NewCacheStore(),
				})
				if err != nil {
					panic(err)
				}
				return e
			}(),
			ctx:           context.Background(),
			key:           "balance:t1:a1:USD",
			value:         []byte(`{"minor":42}`),
			ttl:           time.Minute,
			expectedError: nil,
		},
		{
			name: "valid set l1 only",
			engine: func() *hybrid.Cache {
				e, err := hybrid.New(hybrid.Params{
					L1: fakes.NewCacheStore(),
				})
				if err != nil {
					panic(err)
				}
				return e
			}(),
			ctx:           context.Background(),
			key:           "balance:t1:a1:USD",
			value:         []byte(`{"minor":42}`),
			ttl:           time.Minute,
			expectedError: nil,
		},
		{
			name: "empty key rejected",
			engine: func() *hybrid.Cache {
				e, err := hybrid.New(hybrid.Params{
					L1: fakes.NewCacheStore(),
					L2: fakes.NewCacheStore(),
				})
				if err != nil {
					panic(err)
				}
				return e
			}(),
			ctx:           context.Background(),
			key:           "",
			value:         []byte(`x`),
			ttl:           time.Minute,
			expectedError: hybrid.ErrKeyRequired,
		},
		{
			name: "nil value stores negative-cache marker",
			engine: func() *hybrid.Cache {
				e, err := hybrid.New(hybrid.Params{
					L1: fakes.NewCacheStore(),
					L2: fakes.NewCacheStore(),
				})
				if err != nil {
					panic(err)
				}
				return e
			}(),
			ctx:           context.Background(),
			key:           "balance:t1:a1:USD",
			value:         nil,
			ttl:           time.Minute,
			expectedError: nil,
		},
		{
			name: "zero ttl rejected",
			engine: func() *hybrid.Cache {
				e, err := hybrid.New(hybrid.Params{
					L1: fakes.NewCacheStore(),
					L2: fakes.NewCacheStore(),
				})
				if err != nil {
					panic(err)
				}
				return e
			}(),
			ctx:           context.Background(),
			key:           "balance:t1:a1:USD",
			value:         []byte(`x`),
			ttl:           0,
			expectedError: hybrid.ErrTTLPositive,
		},
		{
			name: "negative ttl rejected",
			engine: func() *hybrid.Cache {
				e, err := hybrid.New(hybrid.Params{
					L1: fakes.NewCacheStore(),
					L2: fakes.NewCacheStore(),
				})
				if err != nil {
					panic(err)
				}
				return e
			}(),
			ctx:           context.Background(),
			key:           "balance:t1:a1:USD",
			value:         []byte(`x`),
			ttl:           -time.Second,
			expectedError: hybrid.ErrTTLPositive,
		},
		{
			name: "canceled context rejected",
			engine: func() *hybrid.Cache {
				e, err := hybrid.New(hybrid.Params{
					L1: fakes.NewCacheStore(),
					L2: fakes.NewCacheStore(),
				})
				if err != nil {
					panic(err)
				}
				return e
			}(),
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			key:           "balance:t1:a1:USD",
			value:         []byte(`x`),
			ttl:           time.Minute,
			expectedError: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.engine.Set(tc.ctx, tc.key, tc.value, tc.ttl)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHybridCacheGetValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		engine         *hybrid.Cache
		ctx            context.Context
		key            string
		expectedResult []byte
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "nil receiver rejected",
			engine:         nil,
			ctx:            context.Background(),
			key:            "balance:t1:a1:USD",
			expectedResult: nil,
			expectedError:  hybrid.ErrNotInitialized,
		},
		{
			name:           "uninitialized engine rejected",
			engine:         &hybrid.Cache{},
			ctx:            context.Background(),
			key:            "balance:t1:a1:USD",
			expectedResult: nil,
			expectedError:  hybrid.ErrNotInitialized,
		},
		{
			name: "empty key rejected",
			engine: func() *hybrid.Cache {
				e, err := hybrid.New(hybrid.Params{
					L1: fakes.NewCacheStore(),
				})
				if err != nil {
					panic(err)
				}
				return e
			}(),
			ctx:            context.Background(),
			key:            "",
			expectedResult: nil,
			expectedError:  hybrid.ErrKeyRequired,
		},
		{
			name: "canceled context rejected",
			engine: func() *hybrid.Cache {
				e, err := hybrid.New(hybrid.Params{
					L1: fakes.NewCacheStore(),
				})
				if err != nil {
					panic(err)
				}
				return e
			}(),
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			key:            "balance:t1:a1:USD",
			expectedResult: nil,
			expectedError:  context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := tc.engine.Get(tc.ctx, tc.key)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				assert.Equal(t, tc.expectedResult, actualResult)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestHybridCacheDelete(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		engine        *hybrid.Cache
		ctx           context.Context
		key           string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "nil receiver rejected",
			engine:        nil,
			ctx:           context.Background(),
			key:           "balance:t1:a1:USD",
			expectedError: hybrid.ErrNotInitialized,
		},
		{
			name:          "uninitialized engine rejected",
			engine:        &hybrid.Cache{},
			ctx:           context.Background(),
			key:           "balance:t1:a1:USD",
			expectedError: hybrid.ErrNotInitialized,
		},
		{
			name: "valid delete both layers",
			engine: func() *hybrid.Cache {
				e, err := hybrid.New(hybrid.Params{
					L1: fakes.NewCacheStore(),
					L2: fakes.NewCacheStore(),
				})
				if err != nil {
					panic(err)
				}
				return e
			}(),
			ctx:           context.Background(),
			key:           "balance:t1:a1:USD",
			expectedError: nil,
		},
		{
			name: "valid delete l1 only",
			engine: func() *hybrid.Cache {
				e, err := hybrid.New(hybrid.Params{
					L1: fakes.NewCacheStore(),
				})
				if err != nil {
					panic(err)
				}
				return e
			}(),
			ctx:           context.Background(),
			key:           "balance:t1:a1:USD",
			expectedError: nil,
		},
		{
			name: "empty key rejected",
			engine: func() *hybrid.Cache {
				e, err := hybrid.New(hybrid.Params{
					L1: fakes.NewCacheStore(),
				})
				if err != nil {
					panic(err)
				}
				return e
			}(),
			ctx:           context.Background(),
			key:           "",
			expectedError: hybrid.ErrKeyRequired,
		},
		{
			name: "canceled context rejected",
			engine: func() *hybrid.Cache {
				e, err := hybrid.New(hybrid.Params{
					L1: fakes.NewCacheStore(),
				})
				if err != nil {
					panic(err)
				}
				return e
			}(),
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			key:           "balance:t1:a1:USD",
			expectedError: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.engine.Delete(tc.ctx, tc.key)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHybridCacheDeleteOrder(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		l2DeleteErr   error
		expectedOrder []string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "l2 delete lands before l1 delete",
			l2DeleteErr:   nil,
			expectedOrder: []string{"l2.delete", "l1.delete"},
			expectedError: nil,
		},
		{
			name:          "failed l2 delete leaves l1 untouched",
			l2DeleteErr:   errors.New("l2 down"),
			expectedOrder: []string{"l2.delete"},
			expectedError: errors.New("l2 down"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			shared := &fakes.Journal{}

			l1 := fakes.NewCacheStore().Share(shared, "l1")
			l2 := fakes.NewCacheStore().Share(shared, "l2").WithDeleteFailure(tc.l2DeleteErr)

			engine, err := hybrid.New(hybrid.Params{L1: l1, L2: l2})
			require.NoError(t, err)

			require.NoError(t, engine.Set(context.Background(), "balance:t1:a1:USD", []byte(`v`), time.Minute))
			shared.Record("setup")

			err = engine.Delete(context.Background(), "balance:t1:a1:USD")
			if tc.expectedError != nil {
				assert.Contains(t, err.Error(), tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expectedOrder, fakes.AfterMarker(shared.Snapshot(), "setup"))
		})
	}
}

type getObservation struct {
	value       []byte
	l2Read      bool
	l1Populated bool
}

func countOp(ops []string, op string) int {
	count := 0
	for _, entry := range ops {
		if entry == op {
			count++
		}
	}
	return count
}

func TestHybridCacheGetFallback(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		key           string
		l1Value       []byte
		l1Present     bool
		l2Value       []byte
		l2Remaining   time.Duration
		l2Present     bool
		l2Error       error
		l2TTLError    error
		expectedValue []byte
		expectedTTL   time.Duration
		expectedError error
		expectL2Read  bool
	}

	testCases := []testCase{
		{
			name:          "l1 hit skips l2",
			key:           "balance:t1:get:USD",
			l1Value:       []byte(`{"minor":1}`),
			l1Present:     true,
			expectedValue: []byte(`{"minor":1}`),
			expectedError: nil,
			expectL2Read:  false,
		},
		{
			name:          "l2 hit populates l1 with remaining ttl",
			key:           "balance:t1:get:USD",
			l2Value:       []byte(`{"minor":2}`),
			l2Remaining:   30 * time.Second,
			l2Present:     true,
			expectedValue: []byte(`{"minor":2}`),
			expectedTTL:   30 * time.Second,
			expectedError: nil,
			expectL2Read:  true,
		},
		{
			name:          "l2 hit without expiry keeps l1 bound",
			key:           "balance:t1:get:USD",
			l2Value:       []byte(`{"minor":3}`),
			l2Remaining:   store.NoExpiry,
			l2Present:     true,
			expectedValue: []byte(`{"minor":3}`),
			expectedTTL:   hybrid.DefaultTTLs().L1Populate,
			expectedError: nil,
			expectL2Read:  true,
		},
		{
			name:          "l2 hit expiring now is not copied to l1",
			key:           "balance:t1:get:USD",
			l2Value:       []byte(`{"minor":4}`),
			l2Remaining:   0,
			l2Present:     true,
			expectedValue: []byte(`{"minor":4}`),
			expectedError: nil,
			expectL2Read:  true,
		},
		{
			name:          "both layers miss",
			key:           "balance:t1:get:USD",
			expectedError: hybrid.ErrCacheMiss,
			expectL2Read:  true,
		},
		{
			name:          "degraded l2 fails open as miss",
			key:           "balance:t1:get:USD",
			l2Error:       errors.New("connection refused"),
			expectedError: hybrid.ErrCacheMiss,
			expectL2Read:  true,
		},
		{
			name:          "ttl read failure skips l1 copy but serves l2 value",
			key:           "balance:t1:get:USD",
			l2Value:       []byte(`{"minor":5}`),
			l2Remaining:   30 * time.Second,
			l2Present:     true,
			l2TTLError:    errors.New("pttl timeout"),
			expectedValue: []byte(`{"minor":5}`),
			expectedError: nil,
			expectL2Read:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			l1 := fakes.NewCacheStore()
			l2 := fakes.NewCacheStore()

			if tc.l1Present {
				require.NoError(t, l1.Set(ctx, tc.key, tc.l1Value, time.Minute))
			}

			if tc.l2Present {
				require.NoError(t, l2.Set(ctx, tc.key, tc.l2Value, time.Minute))
				l2.SetRemaining(tc.key, tc.l2Remaining)
			}

			if tc.l2Error != nil {
				l2.WithGetFailure(tc.l2Error)
			}

			if tc.l2TTLError != nil {
				l2.WithTTLFailure(tc.l2TTLError)
			}

			engine, err := hybrid.New(hybrid.Params{L1: l1, L2: l2})
			require.NoError(t, err)

			l1.Mark("setup")
			l2.Mark("setup")

			got, err := engine.Get(ctx, tc.key)

			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, getObservation{
				value:       tc.expectedValue,
				l2Read:      tc.expectL2Read,
				l1Populated: tc.expectedTTL != 0,
			}, getObservation{
				value:       got,
				l2Read:      len(fakes.AfterMarker(l2.Journal(), "setup")) > 0,
				l1Populated: countOp(fakes.AfterMarker(l1.Journal(), "setup"), "set") > 0,
			})

			if tc.expectedTTL != 0 {
				remaining, err := l1.TTL(ctx, tc.key)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedTTL, remaining)
			}
		})
	}
}

type balance struct {
	Minor int64    `json:"minor"`
	Tags  []string `json:"tags"`
}

func TestHybridCacheTypedValue(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		key            string
		typedValue     balance
		expectedJSON   []byte
		expectedStruct balance
	}

	testCases := []testCase{
		{
			name:           "structured value round trip through shared bytes",
			key:            "balance:t1:a1:USD",
			typedValue:     balance{Minor: 4200, Tags: []string{"settlement"}},
			expectedJSON:   []byte(`{"minor":4200,"tags":["settlement"]}`),
			expectedStruct: balance{Minor: 4200, Tags: []string{"settlement"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			l1 := fakes.NewCacheStore()
			l2 := fakes.NewCacheStore()

			engine, err := hybrid.New(hybrid.Params{L1: l1, L2: l2})
			require.NoError(t, err)

			view := hybrid.Typed[balance](engine)

			require.NoError(t, view.Set(ctx, tc.key, tc.typedValue, time.Minute))

			stored, err := l2.Get(ctx, tc.key)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedJSON, stored)

			l2.Mark("setup")

			got, err := view.Get(ctx, tc.key)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedStruct, got)
			assert.Empty(t, fakes.AfterMarker(l2.Journal(), "setup"))

			got.Tags[0] = "mutated"

			clean, err := view.Get(ctx, tc.key)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedStruct, clean)
		})
	}
}

func TestTypedCacheValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		view          *hybrid.TypedCache[string]
		ctx           context.Context
		key           string
		value         string
		ttl           time.Duration
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "nil typed cache receiver rejected on get",
			view:          nil,
			ctx:           context.Background(),
			key:           "key-1",
			value:         "val-1",
			ttl:           time.Minute,
			expectedError: hybrid.ErrNotInitialized,
		},
		{
			name:          "uninitialized engine rejected on get",
			view:          hybrid.Typed[string](nil),
			ctx:           context.Background(),
			key:           "key-1",
			value:         "val-1",
			ttl:           time.Minute,
			expectedError: hybrid.ErrNotInitialized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.view.Get(tc.ctx, tc.key)
			assert.ErrorIs(t, err, tc.expectedError)

			err = tc.view.Set(tc.ctx, tc.key, tc.value, tc.ttl)
			assert.ErrorIs(t, err, tc.expectedError)

			err = tc.view.Delete(tc.ctx, tc.key)
			assert.ErrorIs(t, err, tc.expectedError)
		})
	}
}

func TestTypedCacheUnsupportedTypes(t *testing.T) {
	t.Parallel()

	engine, err := hybrid.New(hybrid.Params{
		L1: fakes.NewCacheStore(),
	})
	require.NoError(t, err)

	type testCase struct {
		name          string
		action        func() error
		expectedError error
	}

	testCases := []testCase{
		{
			name: "function kind rejected",
			action: func() error {
				view := hybrid.Typed[func()](engine)
				return view.Set(context.Background(), "fn-key", func() {}, time.Minute)
			},
			expectedError: hybrid.ErrUnsupportedValueType,
		},
		{
			name: "channel kind rejected",
			action: func() error {
				view := hybrid.Typed[chan int](engine)
				ch := make(chan int)
				return view.Set(context.Background(), "chan-key", ch, time.Minute)
			},
			expectedError: hybrid.ErrUnsupportedValueType,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.action()
			assert.ErrorIs(t, err, tc.expectedError)
		})
	}
}
