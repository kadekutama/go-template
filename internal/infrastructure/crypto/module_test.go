package crypto_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/crypto"
)

// TestModule proves the crypto fx module resolves its Params bundle into the
// application envelope port. fx.ValidateApp never executes constructors, so
// no key material or transit backend is required.
func TestModule(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		options          []fx.Option
		expectedHasError bool
	}

	testCases := []testCase{
		{
			name: "valid graph exposes envelope crypto port",
			options: []fx.Option{
				crypto.Module(),
				fx.Provide(func() crypto.Params {
					return crypto.Params{
						KEK: nil,
					}
				}),
				fx.Invoke(func(appport.EnvelopeCrypto) {}),
			},
			expectedHasError: false,
		},
		{
			name: "missing crypto params fails validation",
			options: []fx.Option{
				crypto.Module(),
				fx.Invoke(func(appport.EnvelopeCrypto) {}),
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
