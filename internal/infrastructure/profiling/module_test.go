package profiling_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/kadekutama/go-template/internal/infrastructure/logging"
	"github.com/kadekutama/go-template/internal/infrastructure/profiling"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

func TestModuleValid(t *testing.T) {
	t.Parallel()

	testLogger := logging.New(os.Stderr, logging.Config{Level: "info"})

	type testCase struct {
		name    string
		options []fx.Option
	}

	testCases := []testCase{
		{
			name: "module resolves profiler from config and logger",
			options: []fx.Option{
				profiling.Module(),
				fx.Supply(profiling.Config{Enabled: false}),
				fx.Provide(func() log.Logger {
					return testLogger
				}),
				fx.Invoke(func(*profiling.Profiler) {}),
				fx.NopLogger,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, fx.ValidateApp(tc.options...))
		})
	}
}

func TestModuleInvalid(t *testing.T) {
	t.Parallel()

	testLogger := logging.New(os.Stderr, logging.Config{Level: "info"})

	type testCase struct {
		name    string
		options []fx.Option
	}

	testCases := []testCase{
		{
			name: "module requires a profiling config",
			options: []fx.Option{
				profiling.Module(),
				fx.Provide(func() log.Logger {
					return testLogger
				}),
				fx.Invoke(func(*profiling.Profiler) {}),
				fx.NopLogger,
			},
		},
		{
			name: "module requires a logger",
			options: []fx.Option{
				profiling.Module(),
				fx.Supply(profiling.Config{Enabled: false}),
				fx.Invoke(func(*profiling.Profiler) {}),
				fx.NopLogger,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Error(t, fx.ValidateApp(tc.options...))
		})
	}
}

func TestModuleLifecycle(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name   string
		config profiling.Config
	}

	testCases := []testCase{
		{
			name: "disabled module starts and stops cleanly",
			config: profiling.Config{
				Enabled: false,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := fxtest.New(t,
				profiling.Module(),
				fx.Supply(tc.config),
				fx.Provide(func() log.Logger {
					return logging.New(os.Stderr, logging.Config{Level: "info"})
				}),
				fx.NopLogger,
			)

			app.RequireStart()
			app.RequireStop()
		})
	}
}
