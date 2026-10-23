package jwt_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/auth/jwt"
	"github.com/kadekutama/go-template/internal/shared/kernel"
)

// TestModule proves the JWT fx module resolves its Params bundle into the
// application token port. fx.ValidateApp never executes constructors, so no
// signing key is loaded and no token is minted.
func TestModule(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		options          []fx.Option
		expectedHasError bool
	}

	testCases := []testCase{
		{
			name: "valid graph exposes token issuer port",
			options: []fx.Option{
				jwt.Module(),
				fx.Provide(func() jwt.Params {
					return jwt.Params{
						Issuer:        "test-issuer",
						Audience:      "test-audience",
						AccessTTL:     time.Minute,
						RefreshTTL:    time.Hour,
						RotationGrace: time.Minute,
						Clock:         kernel.SystemClock{},
					}
				}),
				fx.Invoke(func(appport.TokenIssuer) {}),
			},
			expectedHasError: false,
		},
		{
			name: "missing jwt params fails validation",
			options: []fx.Option{
				jwt.Module(),
				fx.Invoke(func(appport.TokenIssuer) {}),
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
