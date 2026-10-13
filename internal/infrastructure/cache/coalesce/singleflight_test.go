package coalesce_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/coalesce"
)

func testKey() coalesce.Key {
	return coalesce.Key{
		Provider:      "fxprov",
		Scope:         "tenant-a",
		RequestDigest: "digest-1",
		Kind:          coalesce.CacheableFXRate,
	}
}

func testCoalescer(base context.Context, t *testing.T, timeout time.Duration) *coalesce.Coalescer {
	t.Helper()

	c, err := coalesce.NewCoalescer(coalesce.Params{BaseContext: base, SharedTimeout: timeout})
	require.NoError(t, err)

	return c
}

func errorCode(t *testing.T, err error) string {
	t.Helper()

	var coalesceErr *entity.Error
	require.ErrorAs(t, err, &coalesceErr)

	return coalesceErr.Code
}

func TestCoalescerValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		key               coalesce.Key
		expectedErrorCode string
	}

	testCases := []testCase{
		{
			name: "empty provider rejected",
			key: func() coalesce.Key {
				k := testKey()
				k.Provider = ""

				return k
			}(),
			expectedErrorCode: coalesce.CodeInvalidKey,
		},
		{
			name: "empty scope rejected",
			key: func() coalesce.Key {
				k := testKey()
				k.Scope = ""

				return k
			}(),
			expectedErrorCode: coalesce.CodeInvalidKey,
		},
		{
			name: "empty digest rejected",
			key: func() coalesce.Key {
				k := testKey()
				k.RequestDigest = ""

				return k
			}(),
			expectedErrorCode: coalesce.CodeInvalidKey,
		},
		{
			name: "unknown kind rejected",
			key: func() coalesce.Key {
				k := testKey()
				k.Kind = "unknown"

				return k
			}(),
			expectedErrorCode: coalesce.CodeInvalidKind,
		},
		{
			name: "money-moving kind rejected",
			key: func() coalesce.Key {
				k := testKey()
				k.Kind = "payment_capture"

				return k
			}(),
			expectedErrorCode: coalesce.CodeInvalidKind,
		},
		{
			name: "NUL in scope rejected before sharing",
			key: func() coalesce.Key {
				k := testKey()
				k.Scope = "b\x00s"

				return k
			}(),
			expectedErrorCode: coalesce.CodeInvalidKey,
		},
		{
			name: "NUL in provider rejected before sharing",
			key: func() coalesce.Key {
				k := testKey()
				k.Provider = "a\x00b"

				return k
			}(),
			expectedErrorCode: coalesce.CodeInvalidKey,
		},
		{
			name: "DEL in digest rejected before sharing",
			key: func() coalesce.Key {
				k := testKey()
				k.RequestDigest = "d\x7f"

				return k
			}(),
			expectedErrorCode: coalesce.CodeInvalidKey,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := testCoalescer(context.Background(), t, time.Minute)

			var calls atomic.Int32

			_, err := c.Do(context.Background(), tc.key, func(context.Context) ([]byte, error) {
				calls.Add(1)

				return []byte("x"), nil
			})
			require.Error(t, err)
			assert.Equal(t, tc.expectedErrorCode, errorCode(t, err))
			assert.Equal(t, int32(0), calls.Load())
		})
	}
}

func TestCoalescerParams(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		params            coalesce.Params
		expectedErrorCode string
	}

	testCases := []testCase{
		{
			name:              "nil base context rejected",
			params:            coalesce.Params{BaseContext: nil, SharedTimeout: time.Minute},
			expectedErrorCode: coalesce.CodeInvalidParams,
		},
		{
			name:              "zero timeout rejected",
			params:            coalesce.Params{BaseContext: context.Background(), SharedTimeout: 0},
			expectedErrorCode: coalesce.CodeInvalidParams,
		},
		{
			name:              "negative timeout rejected",
			params:            coalesce.Params{BaseContext: context.Background(), SharedTimeout: -time.Second},
			expectedErrorCode: coalesce.CodeInvalidParams,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c, err := coalesce.NewCoalescer(tc.params)
			assert.Nil(t, c)
			require.Error(t, err)
			assert.Equal(t, tc.expectedErrorCode, errorCode(t, err))
		})
	}

	t.Run("nil work function rejected", func(t *testing.T) {
		t.Parallel()
		c := testCoalescer(context.Background(), t, time.Minute)
		_, err := c.Do(context.Background(), testKey(), nil)
		require.Error(t, err)
		assert.Equal(t, coalesce.CodeInvalidParams, errorCode(t, err))
	})

	t.Run("nil coalescer rejected", func(t *testing.T) {
		t.Parallel()

		var c *coalesce.Coalescer

		_, err := c.Do(context.Background(), testKey(), func(context.Context) ([]byte, error) {
			return []byte("x"), nil
		})
		require.Error(t, err)
		assert.Equal(t, coalesce.CodeInvalidParams, errorCode(t, err))
	})
}

func TestCoalescerCollapse(t *testing.T) {
	t.Parallel()

	t.Run("fifty-way identical read executes once", func(t *testing.T) {
		t.Parallel()
		c := testCoalescer(context.Background(), t, time.Minute)

		var calls atomic.Int32

		const waiters = 50

		results := make([][]byte, waiters)
		errs := make([]error, waiters)

		// All waiters start together while the shared call is slow, so every
		// call overlaps it and collapses: scheduling jitter (microseconds) can
		// never exceed the shared work (hundreds of milliseconds).
		var begin sync.WaitGroup
		begin.Add(waiters)

		var done sync.WaitGroup
		done.Add(waiters)

		for i := range results {
			go func() {
				defer done.Done()

				begin.Done()
				begin.Wait()

				res, err := c.Do(context.Background(), testKey(), func(context.Context) ([]byte, error) {
					calls.Add(1)
					time.Sleep(500 * time.Millisecond)

					return []byte("payload"), nil
				})
				results[i] = res
				errs[i] = err
			}()
		}

		done.Wait()

		assert.Equal(t, int32(1), calls.Load())

		for _, err := range errs {
			assert.NoError(t, err)
		}

		for _, res := range results {
			assert.Equal(t, []byte("payload"), res)
		}

		results[0][0] = 'X'
		for _, res := range results[1:] {
			assert.Equal(t, []byte("payload"), res)
		}
	})
}

func TestCoalescerNoRetention(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		fail          bool
		expectedCalls int32
	}

	testCases := []testCase{
		{
			name:          "errors are not retained",
			fail:          true,
			expectedCalls: 2,
		},
		{
			name:          "results are not retained",
			fail:          false,
			expectedCalls: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := testCoalescer(context.Background(), t, time.Minute)

			var calls atomic.Int32

			work := func(context.Context) ([]byte, error) {
				calls.Add(1)

				if tc.fail {
					return nil, coalesce.ErrProviderDown
				}

				return []byte("ok"), nil
			}
			_, firstErr := c.Do(context.Background(), testKey(), work)
			_, secondErr := c.Do(context.Background(), testKey(), work)

			if tc.fail {
				assert.Error(t, firstErr)
				assert.Error(t, secondErr)
			} else {
				assert.NoError(t, firstErr)
				assert.NoError(t, secondErr)
			}

			assert.Equal(t, tc.expectedCalls, calls.Load())
		})
	}
}

func TestCoalescerCallerCancel(t *testing.T) {
	t.Parallel()

	t.Run("first caller cancel spares remaining waiters", func(t *testing.T) {
		t.Parallel()
		c := testCoalescer(context.Background(), t, time.Minute)

		var calls atomic.Int32

		release := make(chan struct{})
		started := make(chan struct{})

		var startOnce sync.Once

		const waiters = 5

		results := make([][]byte, waiters)
		errs := make([]error, waiters)
		contexts := make([]context.Context, 0, waiters)

		cancels := make([]context.CancelFunc, 0, waiters)

		for range waiters {
			ctx, cancel := context.WithCancel(context.Background())
			contexts = append(contexts, ctx)
			cancels = append(cancels, cancel)
		}

		var begin sync.WaitGroup
		begin.Add(waiters)

		var done sync.WaitGroup
		done.Add(waiters)

		for i := range results {
			go func() {
				defer done.Done()

				begin.Done()
				begin.Wait()

				res, err := c.Do(contexts[i], testKey(), func(ctx context.Context) ([]byte, error) {
					calls.Add(1)
					startOnce.Do(func() { close(started) })

					select {
					case <-release:
						return []byte("payload"), nil
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				})
				results[i] = res
				errs[i] = err
			}()
		}

		select {
		case <-started:
		case <-time.After(10 * time.Second):
			t.Fatal("shared work never started")
		}

		time.Sleep(200 * time.Millisecond)
		cancels[0]()
		close(release)
		done.Wait()

		assert.ErrorIs(t, errs[0], context.Canceled)
		assert.Equal(t, int32(1), calls.Load())

		for _, res := range results[1:] {
			assert.Equal(t, []byte("payload"), res)
		}
	})
}

func TestCoalescerTimeout(t *testing.T) {
	t.Parallel()

	t.Run("shared deadline bounds the call", func(t *testing.T) {
		t.Parallel()
		c := testCoalescer(context.Background(), t, 100*time.Millisecond)
		sawCancel := make(chan struct{})
		_, err := c.Do(context.Background(), testKey(), func(ctx context.Context) ([]byte, error) {
			select {
			case <-ctx.Done():
				close(sawCancel)

				return nil, ctx.Err()
			case <-time.After(time.Minute):
				return []byte("too late"), nil
			}
		})
		assert.ErrorIs(t, err, context.DeadlineExceeded)

		select {
		case <-sawCancel:
		case <-time.After(10 * time.Second):
			t.Fatal("shared work ignored its deadline")
		}
	})
}

func TestCoalescerScopeIsolation(t *testing.T) {
	t.Parallel()

	t.Run("scopes isolate in-flight work", func(t *testing.T) {
		t.Parallel()
		c := testCoalescer(context.Background(), t, time.Minute)

		var calls atomic.Int32

		work := func(context.Context) ([]byte, error) {
			calls.Add(1)

			return []byte("payload"), nil
		}
		tenantA := testKey()
		tenantB := testKey()
		tenantB.Scope = "tenant-b"
		otherProvider := testKey()
		otherProvider.Provider = "otherprov"

		_, err := c.Do(context.Background(), tenantA, work)
		require.NoError(t, err)
		_, err = c.Do(context.Background(), tenantB, work)
		require.NoError(t, err)
		_, err = c.Do(context.Background(), otherProvider, work)
		require.NoError(t, err)
		assert.Equal(t, int32(3), calls.Load())
	})
}

func TestCoalescerShutdown(t *testing.T) {
	t.Parallel()

	t.Run("service shutdown cancels shared work", func(t *testing.T) {
		t.Parallel()

		base, cancel := context.WithCancel(context.Background())
		c := testCoalescer(base, t, time.Minute)
		sawCancel := make(chan struct{})
		done := make(chan error, 1)

		go func() {
			_, err := c.Do(context.Background(), testKey(), func(ctx context.Context) ([]byte, error) {
				select {
				case <-ctx.Done():
					close(sawCancel)

					return nil, ctx.Err()
				case <-time.After(time.Minute):
					return []byte("too late"), nil
				}
			})
			done <- err
		}()

		time.Sleep(200 * time.Millisecond)
		cancel()

		select {
		case err := <-done:
			require.Error(t, err)
		case <-time.After(10 * time.Second):
			t.Fatal("waiter survived service shutdown")
		}

		select {
		case <-sawCancel:
		case <-time.After(10 * time.Second):
			t.Fatal("shared work ignored service shutdown")
		}
	})
}
