package oauth2_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/auth/oauth2"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/valkey"
	"github.com/kadekutama/go-template/internal/shared/kernel"
)

// TestModule proves the OAuth2 fx module resolves its Params bundle and
// Valkey-backed KVStore into the application OAuth port. fx.ValidateApp never
// executes constructors, so no Valkey or provider endpoint is dialed.
func TestModule(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		options          []fx.Option
		expectedHasError bool
	}

	testCases := []testCase{
		{
			name: "valid graph exposes oauth provider and kv store",
			options: []fx.Option{
				oauth2.Module(),
				fx.Provide(func() *valkey.Client { return nil }),
				fx.Provide(func() oauth2.OAuthParams {
					return oauth2.OAuthParams{
						Providers: map[string]oauth2.ProviderConfig{
							"test": {},
						},
						StateSecret: make([]byte, 32),
						StateTTL:    time.Minute,
						Clock:       kernel.SystemClock{},
					}
				}),
				fx.Invoke(func(appport.OAuthProvider, oauth2.KVStore) {}),
			},
			expectedHasError: false,
		},
		{
			name: "missing oauth params fails validation",
			options: []fx.Option{
				oauth2.Module(),
				fx.Invoke(func(appport.OAuthProvider, oauth2.KVStore) {}),
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
