package featureflag_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/featureflag"
	"github.com/kadekutama/go-template/internal/shared/kernel"
)

// TestModule proves the feature flag fx module resolves its Params bundle into
// the application flag port. fx.ValidateApp never executes constructors, so
// no Unleash request or egress permit is acquired.
func TestModule(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		options          []fx.Option
		expectedHasError bool
	}

	testCases := []testCase{
		{
			name: "valid graph exposes flag client port",
			options: []fx.Option{
				featureflag.Module(),
				fx.Provide(func() featureflag.Params {
					return featureflag.Params{
						UnleashURL:        "http://127.0.0.1:4242/api",
						AppName:           "test-app",
						RefreshInterval:   time.Minute,
						CacheTTL:          time.Minute,
						InitTimeout:       time.Second,
						ProbeMaxBodyBytes: 1024,
						Clock:             kernel.SystemClock{},
					}
				}),
				fx.Invoke(func(appport.FlagClient) {}),
			},
			expectedHasError: false,
		},
		{
			name: "missing flag params fails validation",
			options: []fx.Option{
				featureflag.Module(),
				fx.Invoke(func(appport.FlagClient) {}),
			},
			expectedHasError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := fx.ValidateApp(tc.options...)
			assert.Equal(t, tc.expectedHasError, err != nil)
		})
	}
}
