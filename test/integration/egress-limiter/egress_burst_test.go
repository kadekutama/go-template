package egresslimiter_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/valkey"
	"github.com/kadekutama/go-template/internal/infrastructure/egress"
	"github.com/kadekutama/go-template/internal/shared/kernel"
	"github.com/kadekutama/go-template/internal/shared/kernel/sleep"
	testcontainers "github.com/kadekutama/go-template/test/testcontainers"
)

// TestEgressBurst proves bounded provider-scoped admission against real
// Valkey (E09.1-T03-S01): with a fresh bucket of capacity 10 and a maxWait
// shorter than one refill interval, exactly the initial capacity is
// admitted and every excess caller fails closed without over-issuing.
func TestEgressBurst(t *testing.T) {
	t.Parallel()

	t.Run("bounded burst admits exactly capacity", func(t *testing.T) {
		testcontainers.SkipIfNoDocker(t)

		handle, err := testcontainers.StartValkey(t)
		require.NoError(t, err)

		ctx := context.Background()
		client, err := valkey.NewValkeyClient(valkey.Params{
			Addr:         handle.Addr(),
			PoolSize:     4,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		})
		require.NoError(t, err)
		t.Cleanup(func() { _ = client.Close() })
		require.NoError(t, client.Ping(ctx))

		var (
			allows atomic.Int64
			denies atomic.Int64
		)

		adapter, err := valkey.NewRateLimiter(valkey.LimiterParams{
			Client:  client,
			OnAllow: func(context.Context, string) { allows.Add(1) },
			OnDeny:  func(context.Context, string) { denies.Add(1) },
		})
		require.NoError(t, err)

		waiter, err := egress.NewEgressLimiter(egress.Params{
			Limiter: adapter,
			Policies: []egress.Policy{
				{Provider: "burstprov", Operation: "burstop", Budget: 10, Window: time.Minute, MaxWait: 2 * time.Second},
			},
			Clock:   kernel.SystemClock{},
			Jitter:  egress.NewCappedJitter(),
			Sleeper: sleep.TimerSleeper{},
		})
		require.NoError(t, err)

		const callers = 20

		results := make([]error, callers)

		var started sync.WaitGroup
		started.Add(callers)

		var done sync.WaitGroup
		done.Add(callers)

		for i := range results {
			go func() {
				defer done.Done()

				started.Done()
				started.Wait()
				results[i] = waiter.Wait(context.Background(), "burstprov", "burstop")
			}()
		}

		done.Wait()

		var (
			admitted int
			exceeded int
		)

		for _, result := range results {
			if result == nil {
				admitted++

				continue
			}

			var egressErr *entity.Error
			if errors.As(result, &egressErr) && egressErr.Code == egress.CodeWaitExceeded {
				exceeded++
			} else {
				t.Fatalf("unexpected waiter result: %v", result)
			}
		}

		assert.Equal(t, 10, admitted)
		assert.Equal(t, 10, exceeded)
		assert.Equal(t, int64(10), allows.Load())
		assert.Equal(t, int64(10), denies.Load())
	})
}
