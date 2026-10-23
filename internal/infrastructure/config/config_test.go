package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/infrastructure/profiling"
	kernelvalidate "github.com/kadekutama/go-template/internal/shared/kernel/validate"
)

func TestBaseConfigProfilingDefaults(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		base              string
		overlays          []string
		expectedProfiling ProfilingConfig
		expectedError     error
	}

	testCases := []testCase{
		{
			name: "base config carries disabled profiling defaults",
			base: repoConfig("config.yaml"),
			expectedProfiling: ProfilingConfig{
				Enabled:              false,
				ServerAddress:        "http://localhost:4040",
				ApplicationName:      "fintech-ledger",
				Environment:          "local",
				UploadRate:           15 * time.Second,
				MutexProfileFraction: 5,
				BlockProfileRate:     5,
				DisableGCRuns:        false,
				Tags: map[string]string{
					"service": "fintech-ledger",
					"env":     "local",
					"region":  "local",
					"version": "dev",
				},
			},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actualResult, err := Load(tc.base, tc.overlays...)
			assert.Equal(t, tc.expectedError, err)

			if err != nil {
				return
			}

			assert.Equal(t, tc.expectedProfiling, actualResult.Observability.Profiling)
		})
	}
}

func TestProfilingConfigMapping(t *testing.T) {
	t.Parallel()

	base := ProfilingConfig{
		Enabled:              true,
		ServerAddress:        "http://localhost:4040",
		ApplicationName:      "fintech-ledger",
		Environment:          "local",
		UploadRate:           15 * time.Second,
		MutexProfileFraction: 5,
		BlockProfileRate:     5,
		DisableGCRuns:        false,
		Tags: map[string]string{
			"service": "fintech-ledger",
			"env":     "local",
		},
	}

	type testCase struct {
		name           string
		config         ProfilingConfig
		expectedResult profiling.Config
		expectedError  error
	}

	testCases := []testCase{
		{
			name:   "disabled empty config maps and validates",
			config: ProfilingConfig{},
			expectedResult: profiling.Config{
				Tags: nil,
			},
			expectedError: nil,
		},
		{
			name:   "enabled valid config maps every field",
			config: base,
			expectedResult: profiling.Config{
				Enabled:              true,
				ServerAddress:        "http://localhost:4040",
				ApplicationName:      "fintech-ledger",
				Environment:          "local",
				UploadRate:           15 * time.Second,
				MutexProfileFraction: 5,
				BlockProfileRate:     5,
				DisableGCRuns:        false,
				Tags: map[string]string{
					"service": "fintech-ledger",
					"env":     "local",
				},
			},
			expectedError: nil,
		},
		{
			name: "invalid server address rejected when enabled",
			config: func() ProfilingConfig {
				c := base
				c.ServerAddress = "://bad"

				return c
			}(),
			expectedResult: profiling.Config{},
			expectedError:  kernelvalidate.ErrValidateViolation,
		},
		{
			name: "missing application name rejected when enabled",
			config: func() ProfilingConfig {
				c := base
				c.ApplicationName = ""

				return c
			}(),
			expectedResult: profiling.Config{},
			expectedError:  kernelvalidate.ErrValidateViolation,
		},
		{
			name: "negative mutex fraction rejected",
			config: func() ProfilingConfig {
				c := base
				c.MutexProfileFraction = -1

				return c
			}(),
			expectedResult: profiling.Config{},
			expectedError:  kernelvalidate.ErrValidateViolation,
		},
		{
			name: "negative block rate rejected",
			config: func() ProfilingConfig {
				c := base
				c.BlockProfileRate = -1

				return c
			}(),
			expectedResult: profiling.Config{},
			expectedError:  kernelvalidate.ErrValidateViolation,
		},
		{
			name: "forbidden tenant tag rejected",
			config: func() ProfilingConfig {
				c := base
				c.Tags = map[string]string{"tenant_id": "t-1"}

				return c
			}(),
			expectedResult: profiling.Config{},
			expectedError:  profiling.ErrForbiddenTagKey,
		},
		{
			name: "blank tag key rejected",
			config: func() ProfilingConfig {
				c := base
				c.Tags = map[string]string{"   ": "value"}

				return c
			}(),
			expectedResult: profiling.Config{},
			expectedError:  profiling.ErrEmptyTagKey,
		},
		{
			name: "unknown high-cardinality tag rejected",
			config: func() ProfilingConfig {
				c := base
				c.Tags = map[string]string{"request_id": "req-1"}

				return c
			}(),
			expectedResult: profiling.Config{},
			expectedError:  profiling.ErrUnsupportedTagKey,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actualResult, err := tc.config.Profiling()
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.ErrorIs(t, err, tc.expectedError)
		})
	}
}

func TestTracingMappingIncludesProfilingEnabled(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		profilingConfig ProfilingConfig
		expectedResult  bool
	}

	testCases := []testCase{
		{
			name:            "profiling disabled leaves tracing unwrapped",
			profilingConfig: ProfilingConfig{Enabled: false},
			expectedResult:  false,
		},
		{
			name:            "profiling enabled wraps tracing provider",
			profilingConfig: ProfilingConfig{Enabled: true},
			expectedResult:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := Config{
				App: AppConfig{Name: "svc", Env: "local"},
				Observability: ObservabilityConfig{
					SampleRatio:    0.1,
					ExcludedRoutes: []string{"/healthz"},
					Profiling:      tc.profilingConfig,
				},
			}

			tracingConfig, err := cfg.Tracing("v1")
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, tracingConfig.ProfilingEnabled)
		})
	}
}
