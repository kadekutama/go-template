package shutdown

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunDrainsOnCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	release := make(chan struct{})
	var drained atomic.Bool

	serve := func(ctx context.Context) error {
		<-ctx.Done()
		<-release // hold until the test frees us; proves Run chose the drain path
		return nil
	}
	drain := func(context.Context) error {
		drained.Store(true)
		return nil
	}

	time.AfterFunc(100*time.Millisecond, cancel)
	timeout := 5 * time.Second
	if err := Run(ctx, &timeout, serve, drain); err != nil {
		t.Fatalf("Run: %v", err)
	}
	close(release)
	if !drained.Load() {
		t.Error("drain did not run after cancel")
	}
}

func TestRun(t *testing.T) {
	t.Parallel()

	sentinelServeErr := errors.New("serve boom")
	sentinelDrainErr := errors.New("drain failure")
	shortTimeout := 20 * time.Millisecond

	type testCase struct {
		name          string
		ctx           func() context.Context
		timeout       *time.Duration
		serve         func(context.Context) error
		drain         func(context.Context) error
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "nil serve returns error",
			ctx:           context.Background,
			timeout:       nil,
			serve:         nil,
			drain:         nil,
			expectedError: errors.New("shutdown: serve func must not be nil"),
		},
		{
			name:    "serve returns error immediately",
			ctx:     context.Background,
			timeout: nil,
			serve: func(context.Context) error {
				return sentinelServeErr
			},
			drain: func(context.Context) error {
				return nil
			},
			expectedError: sentinelServeErr,
		},
		{
			name: "nil drain succeeds after cancel",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			timeout: nil,
			serve: func(ctx context.Context) error {
				<-ctx.Done()
				return nil
			},
			drain:         nil,
			expectedError: nil,
		},
		{
			name: "drain returns error",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			timeout: nil,
			serve: func(ctx context.Context) error {
				<-ctx.Done()
				return nil
			},
			drain: func(context.Context) error {
				return sentinelDrainErr
			},
			expectedError: sentinelDrainErr,
		},
		{
			name: "drain timeout returns DeadlineExceeded",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			timeout: &shortTimeout,
			serve: func(ctx context.Context) error {
				<-ctx.Done()
				return nil
			},
			drain: func(ctx context.Context) error {
				<-ctx.Done()
				return ctx.Err()
			},
			expectedError: context.DeadlineExceeded,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := Run(tc.ctx(), tc.timeout, tc.serve, tc.drain)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
