package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/kadekutama/go-template/internal/infrastructure/logging"
	"github.com/kadekutama/go-template/internal/shared/di"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

func TestSeedGraphValidates(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		expectedError bool
	}

	testCases := []testCase{
		{
			name:          "graph validates with no database present",
			expectedError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var runner *seedRunner

			err := fx.ValidateApp(seedGraphOptions(&runner)...)
			if tc.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Nil(t, runner, "validation must not construct anything")
		})
	}
}

func TestSeedAppRequiresDSN(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("APP_FX_START_TIMEOUT", "30s")
	t.Setenv("APP_FX_STOP_TIMEOUT", "30s")
	t.Setenv("APP_LOG_LEVEL", "info")

	var output testLogBuffer

	logger, err := testLogger(&output)
	require.NoError(t, err)

	var runner *seedRunner

	work := func(context.Context) error {
		t.Error("work must not run when the pool cannot be built")
		return nil
	}

	code := di.RunApp(logger, "seed", "v0-test", work, seedGraphOptions(&runner)...)
	assert.Equal(t, 1, code, "missing DSN must exit 1 without touching docker")
	assert.Contains(t, output.lines(), "app failed to start")
}

// testLogBuffer captures logger output for assertions.
type testLogBuffer struct {
	buf atomic.Pointer[[]byte]
}

func (b *testLogBuffer) Write(p []byte) (int, error) {
	for {
		old := b.buf.Load()
		var next []byte
		if old != nil {
			next = make([]byte, len(*old)+len(p))
			copy(next, *old)
			copy(next[len(*old):], p)
		} else {
			next = make([]byte, len(p))
			copy(next, p)
		}
		if b.buf.CompareAndSwap(old, &next) {
			break
		}
	}
	return len(p), nil
}

func (b *testLogBuffer) lines() string {
	old := b.buf.Load()
	if old == nil {
		return ""
	}
	return string(*old)
}

// testLogger builds a buffer-backed kernel logger for hermetic assertions.
func testLogger(output *testLogBuffer) (log.Logger, error) {
	if output == nil {
		return nil, errors.New("test logger requires a buffer")
	}

	return logging.New(output, logging.Config{Level: "trace"}), nil
}
