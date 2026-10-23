package hybrid_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/hybrid"
)

// TestModuleGraph proves the hybrid cache fx module resolves its Params bundle
// into both the engine contract and the application cache port. fx.ValidateApp
// never executes constructors, so no L1/L2 store is opened.
func TestModuleGraph(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		options          []fx.Option
		expectedHasError bool
	}

	testCases := []testCase{
		{
			name: "valid graph exposes engine and cache ports",
			options: []fx.Option{
				hybrid.Module(),
				fx.Provide(func() hybrid.Params {
					return hybrid.Params{
						L1PopulateTTL: time.Minute,
					}
				}),
				fx.Invoke(func(hybrid.Engine, appport.Cache) {}),
			},
			expectedHasError: false,
		},
		{
			name: "missing hybrid params fails validation",
			options: []fx.Option{
				hybrid.Module(),
				fx.Invoke(func(hybrid.Engine, appport.Cache) {}),
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
