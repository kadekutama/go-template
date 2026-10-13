package scheduleregress_test

import (
	"context"
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

type retryBreaker struct {
	failures int
}

func (b *retryBreaker) Execute(fn func() error) error {
	err := fn()
	if err != nil {
		b.failures++
	}

	return err
}

type retryProvider struct {
	calls int
	err   error
}

func (p *retryProvider) Do() error {
	p.calls++

	return p.err
}

// TestEgressBurstAndRetry proves retry-permit composition against real
// Valkey (E09.1-T05-S03): each retry acquires one permit, and limiter or
// store failures never reach the breaker or the provider.
func TestEgressBurstAndRetry(t *testing.T) {
	t.Parallel()

	t.Run("retries reacquire permits and failures stay isolated", func(t *testing.T) {
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

		adapter, err := valkey.NewRateLimiter(valkey.LimiterParams{Client: client})
		require.NoError(t, err)

		waiter, err := egress.NewEgressLimiter(egress.Params{
			Limiter: adapter,
			Policies: []egress.Policy{
				{Provider: "retryprov", Operation: "retryop", Budget: 2, Window: time.Minute, MaxWait: 100 * time.Millisecond},
			},
			Clock:   kernel.SystemClock{},
			Jitter:  egress.NewCappedJitter(),
			Sleeper: sleep.TimerSleeper{},
		})
		require.NoError(t, err)

		breaker := &retryBreaker{}
		provider := &retryProvider{}

		attempt := func() error {
			if err := waiter.Wait(ctx, "retryprov", "retryop"); err != nil {
				return err
			}

			return breaker.Execute(provider.Do)
		}

		require.NoError(t, attempt())
		require.NoError(t, attempt())

		retryErr := attempt()
		require.Error(t, retryErr)

		var egressErr *entity.Error
		require.ErrorAs(t, retryErr, &egressErr)
		assert.Equal(t, egress.CodeWaitExceeded, egressErr.Code)

		assert.Equal(t, 2, provider.calls)
		assert.Equal(t, 0, breaker.failures)
	})
}

func TestEgressStoreErrorIsolation(t *testing.T) {
	t.Parallel()

	t.Run("store errors never reach breaker or provider", func(t *testing.T) {
		unreachable, err := valkey.NewValkeyClient(valkey.Params{
			Addr:         "127.0.0.1:1",
			PoolSize:     1,
			DialTimeout:  time.Second,
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
		})
		require.NoError(t, err)
		t.Cleanup(func() { _ = unreachable.Close() })

		adapter, err := valkey.NewRateLimiter(valkey.LimiterParams{Client: unreachable})
		require.NoError(t, err)

		waiter, err := egress.NewEgressLimiter(egress.Params{
			Limiter: adapter,
			Policies: []egress.Policy{
				{Provider: "downprov", Operation: "downop", Budget: 10, Window: time.Minute, MaxWait: 100 * time.Millisecond},
			},
		})
		require.NoError(t, err)

		breaker := &retryBreaker{}
		provider := &retryProvider{}

		waitErr := waiter.Wait(context.Background(), "downprov", "downop")
		require.Error(t, waitErr)

		var egressErr *entity.Error
		require.ErrorAs(t, waitErr, &egressErr)
		assert.Equal(t, egress.CodeStoreUnavailable, egressErr.Code)

		assert.Equal(t, 0, provider.calls)
		assert.Equal(t, 0, breaker.failures)
	})
}
