package apikey_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/auth/apikey"
	"github.com/kadekutama/go-template/internal/shared/kernel"
)

// TestModule proves the API key fx module resolves its Params bundle into the
// application API key port. fx.ValidateApp never executes constructors, so no
// database or hasher work happens.
func TestModule(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		options          []fx.Option
		expectedHasError bool
	}

	testCases := []testCase{
		{
			name: "valid graph exposes api key store port",
			options: []fx.Option{
				apikey.Module(),
				fx.Provide(func() apikey.Params {
					return apikey.Params{
						Hasher:         apikey.Hasher{},
						RotationWindow: time.Hour,
						Clock:          kernel.SystemClock{},
					}
				}),
				fx.Invoke(func(appport.APIKeyStore) {}),
			},
			expectedHasError: false,
		},
		{
			name: "missing apikey params fails validation",
			options: []fx.Option{
				apikey.Module(),
				fx.Invoke(func(appport.APIKeyStore) {}),
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
