package openbao_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/secrets/openbao"
	"github.com/kadekutama/go-template/internal/shared/kernel"
)

// TestModule proves the OpenBao fx module resolves its Params bundle into the
// application secret port. fx.ValidateApp never executes constructors, so no
// OpenBao endpoint is dialed.
func TestModule(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		options          []fx.Option
		expectedHasError bool
	}

	testCases := []testCase{
		{
			name: "valid graph exposes secret store port",
			options: []fx.Option{
				openbao.Module(),
				fx.Provide(func() openbao.Params {
					return openbao.Params{
						Address:    "http://127.0.0.1:8200",
						Token:      "test-token",
						KVMount:    "secret",
						DBMount:    "database",
						DefaultTTL: time.Minute,
						Clock:      kernel.SystemClock{},
					}
				}),
				fx.Invoke(func(appport.SecretStore) {}),
			},
			expectedHasError: false,
		},
		{
			name: "missing openbao params fails validation",
			options: []fx.Option{
				openbao.Module(),
				fx.Invoke(func(appport.SecretStore) {}),
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
