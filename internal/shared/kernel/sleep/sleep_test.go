package sleep_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/shared/kernel/sleep"
)

func TestTimerSleeper(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		ctx             context.Context
		duration        time.Duration
		expectError     bool
		maxElapsed      time.Duration
		expectElapsedAt bool
	}

	testCases := []testCase{
		{
			name:            "short sleep succeeds",
			ctx:             context.Background(),
			duration:        20 * time.Millisecond,
			expectError:     false,
			maxElapsed:      5 * time.Second,
			expectElapsedAt: true,
		},
		{
			name:            "zero duration resolves immediately",
			ctx:             context.Background(),
			duration:        0,
			expectError:     false,
			maxElapsed:      time.Second,
			expectElapsedAt: false,
		},
		{
			name: "pre-canceled context fails",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx
			}(),
			duration:        time.Minute,
			expectError:     true,
			maxElapsed:      time.Second,
			expectElapsedAt: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			start := time.Now()
			err := sleep.TimerSleeper{}.Sleep(tc.ctx, tc.duration)
			elapsed := time.Since(start)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Less(t, elapsed, tc.maxElapsed)

			if tc.expectElapsedAt {
				assert.GreaterOrEqual(t, elapsed, tc.duration)
			}
		})
	}

	t.Run("mid-wait cancellation interrupts", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan error, 1)
		go func() {
			done <- sleep.TimerSleeper{}.Sleep(ctx, time.Minute)
		}()

		cancel()

		select {
		case err := <-done:
			assert.ErrorIs(t, err, context.Canceled)
		case <-time.After(10 * time.Second):
			t.Fatal("sleep ignored cancellation")
		}
	})
}
