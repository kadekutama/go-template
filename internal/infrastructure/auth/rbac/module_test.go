package rbac_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/auth/rbac"
)

// TestModule proves the RBAC fx module resolves its Params bundle into the
// application authorizer port. fx.ValidateApp never executes constructors, so
// no policy file or database is read.
func TestModule(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		options          []fx.Option
		expectedHasError bool
	}

	testCases := []testCase{
		{
			name: "valid graph exposes authorizer port",
			options: []fx.Option{
				rbac.Module(),
				fx.Provide(func() rbac.Params {
					return rbac.Params{
						ModelText:            "",
						ThresholdAmountMinor: 1000,
					}
				}),
				fx.Invoke(func(appport.Authorizer) {}),
			},
			expectedHasError: false,
		},
		{
			name: "missing rbac params fails validation",
			options: []fx.Option{
				rbac.Module(),
				fx.Invoke(func(appport.Authorizer) {}),
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
