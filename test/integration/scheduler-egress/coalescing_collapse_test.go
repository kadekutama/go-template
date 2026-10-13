package scheduleregress_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/infrastructure/cache/coalesce"
)

// errCoalescingOverflow guards test fake provider counter wrap.
var errCoalescingOverflow = errors.New("coalescing provider call counter overflowed")

// TestCoalescingCollapse proves scoped collapse through the real Coalescer
// (E09.1-T05-S04): concurrent identical scoped reads run one provider call
// with independent result slices, a different scope runs separately, and
// canceling one waiter spares the rest.
func TestCoalescingCollapse(t *testing.T) {
	t.Parallel()

	t.Run("identical scoped reads collapse", func(t *testing.T) {
		base, cancel := context.WithCancel(t.Context())
		defer cancel()

		coalescer, err := coalesce.NewCoalescer(coalesce.Params{BaseContext: base, SharedTimeout: time.Minute})
		require.NoError(t, err)

		key := coalesce.Key{Provider: "fxprov", Scope: "tenant-a", RequestDigest: "digest-1", Kind: coalesce.CacheableFXRate}

		var calls atomic.Int32

		work := func(context.Context) ([]byte, error) {
			if n := calls.Add(1); n <= 0 {
				return nil, errCoalescingOverflow
			}

			time.Sleep(300 * time.Millisecond)

			return []byte("rate"), nil
		}

		const waiters = 20

		results := make([][]byte, waiters)
		errs := make([]error, waiters)

		var begin sync.WaitGroup
		begin.Add(waiters)

		var done sync.WaitGroup
		done.Add(waiters)

		for i := range results {
			go func() {
				defer done.Done()

				begin.Done()
				begin.Wait()

				res, err := coalescer.Do(context.Background(), key, work)
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
			assert.Equal(t, []byte("rate"), res)
		}

		results[0][0] = 'X'
		for _, res := range results[1:] {
			assert.Equal(t, []byte("rate"), res)
		}

		otherScope := key
		otherScope.Scope = "tenant-b"
		_, err = coalescer.Do(context.Background(), otherScope, work)
		require.NoError(t, err)
		assert.Equal(t, int32(2), calls.Load())
	})
}

func TestCoalescingCancelIsolation(t *testing.T) {
	t.Parallel()

	t.Run("canceled waiter spares the rest", func(t *testing.T) {
		coalescer, err := coalesce.NewCoalescer(coalesce.Params{BaseContext: context.Background(), SharedTimeout: time.Minute})
		require.NoError(t, err)

		key := coalesce.Key{Provider: "fxprov", Scope: "tenant-a", RequestDigest: "digest-9", Kind: coalesce.CacheableFXRate}
		release := make(chan struct{})
		started := make(chan struct{})

		var (
			startOnce sync.Once
			calls     atomic.Int32
		)

		work := func(ctx context.Context) ([]byte, error) {
			calls.Add(1)
			startOnce.Do(func() { close(started) })

			select {
			case <-release:
				return []byte("rate"), nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		first, cancelFirst := context.WithCancel(context.Background())
		firstErr := make(chan error, 1)

		go func() {
			_, err := coalescer.Do(first, key, work)
			firstErr <- err
		}()

		select {
		case <-started:
		case <-time.After(10 * time.Second):
			t.Fatal("shared work never started")
		}

		othersErr := make(chan error, 3)

		for range 3 {
			go func() {
				_, err := coalescer.Do(context.Background(), key, work)
				othersErr <- err
			}()
		}

		time.Sleep(200 * time.Millisecond)
		cancelFirst()

		select {
		case err := <-firstErr:
			assert.ErrorIs(t, err, context.Canceled)
		case <-time.After(10 * time.Second):
			t.Fatal("canceled waiter never returned")
		}

		close(release)

		for range 3 {
			select {
			case err := <-othersErr:
				assert.NoError(t, err)
			case <-time.After(10 * time.Second):
				t.Fatal("remaining waiter never returned")
			}
		}

		assert.Equal(t, int32(1), calls.Load())
	})
}
