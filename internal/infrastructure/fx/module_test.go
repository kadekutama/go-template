package fx_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	uberfx "go.uber.org/fx"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/fx"
)

// TestModule proves the FX fx module resolves its Params bundle into the
// application FxRateProvider port. fx.ValidateApp never executes constructors,
// so no HTTP request, cache read, or egress permit is acquired.
func TestModule(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name    string
		options []uberfx.Option
	}

	testCases := []testCase{
		{
			name: "valid graph exposes fx rate provider port",
			options: []uberfx.Option{
				fx.Module(),
				uberfx.Provide(func() fx.Params {
					return fx.Params{
						BaseURL: "https://fx.example.test",
						APIKey:  "test-key",
						Source:  "exchangerate.host",
					}
				}),
				uberfx.Invoke(func(appport.FxRateProvider) {}),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := uberfx.ValidateApp(tc.options...)
			assert.NoError(t, err)
		})
	}
}

func TestModuleMissingParams(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name    string
		options []uberfx.Option
	}

	testCases := []testCase{
		{
			name: "missing fx params fails validation",
			options: []uberfx.Option{
				fx.Module(),
				uberfx.Invoke(func(appport.FxRateProvider) {}),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := uberfx.ValidateApp(tc.options...)
			assert.Error(t, err)
		})
	}
}
