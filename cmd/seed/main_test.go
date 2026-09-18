package main

import (
	"bytes"
	"context"
	"errors"
	"sync"
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
	t.Setenv("APP_FX_START_TIMEOUT", "")
	t.Setenv("APP_FX_STOP_TIMEOUT", "")

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
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *testLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *testLogBuffer) lines() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

// testLogger builds a buffer-backed kernel logger for hermetic assertions.
func testLogger(output *testLogBuffer) (log.Logger, error) {
	if output == nil {
		return nil, errors.New("test logger requires a buffer")
	}

	return logging.New(output, logging.Config{Level: "trace"}), nil
}
