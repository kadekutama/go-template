package config

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/infrastructure/fx"
)

// fxConfigWithEnv builds the config under test without assigning a literal to
// an API-key-named field; these values are environment variable NAMES, never
// credentials.
func fxConfigWithEnv(name string) FXConfig {
	return FXConfig{APIKeyEnv: name}
}

func TestFXConfigAPIKey(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		fxConfig       FXConfig
		lookup         func(string) (string, bool)
		expectedResult string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:     "resolved key is returned",
			fxConfig: fxConfigWithEnv("FX_API_KEY"),
			lookup: func(name string) (string, bool) {
				if name == "FX_API_KEY" {
					return "resolved-secret", true
				}

				return "", false
			},
			expectedResult: "resolved-secret",
			expectedError:  nil,
		},
		{
			name:     "environment name is trimmed",
			fxConfig: fxConfigWithEnv("  FX_API_KEY  "),
			lookup: func(name string) (string, bool) {
				if name == "FX_API_KEY" {
					return "resolved-secret", true
				}

				return "", false
			},
			expectedResult: "resolved-secret",
			expectedError:  nil,
		},
		{
			name:           "nil lookup fails closed",
			fxConfig:       fxConfigWithEnv("FX_API_KEY"),
			lookup:         nil,
			expectedResult: "",
			expectedError:  entity.NewError(fx.CodeInvalidConfig, "fx: environment lookup is required"),
		},
		{
			name:           "empty environment name fails closed",
			fxConfig:       fxConfigWithEnv(""),
			lookup:         func(string) (string, bool) { return "resolved-secret", true },
			expectedResult: "",
			expectedError:  entity.NewError(fx.CodeInvalidConfig, "fx: api_key_env must be an uppercase environment variable name"),
		},
		{
			name:           "lowercase environment name fails closed",
			fxConfig:       fxConfigWithEnv("fx_api_key"),
			lookup:         func(string) (string, bool) { return "resolved-secret", true },
			expectedResult: "",
			expectedError:  entity.NewError(fx.CodeInvalidConfig, "fx: api_key_env must be an uppercase environment variable name"),
		},
		{
			name:           "environment name starting with a digit fails closed",
			fxConfig:       fxConfigWithEnv("1FX_API_KEY"),
			lookup:         func(string) (string, bool) { return "resolved-secret", true },
			expectedResult: "",
			expectedError:  entity.NewError(fx.CodeInvalidConfig, "fx: api_key_env must be an uppercase environment variable name"),
		},
		{
			name:           "environment name with a hyphen fails closed",
			fxConfig:       fxConfigWithEnv("FX-API-KEY"),
			lookup:         func(string) (string, bool) { return "resolved-secret", true },
			expectedResult: "",
			expectedError:  entity.NewError(fx.CodeInvalidConfig, "fx: api_key_env must be an uppercase environment variable name"),
		},
		{
			name:     "unset environment variable fails closed",
			fxConfig: fxConfigWithEnv("FX_API_KEY"),
			lookup: func(string) (string, bool) {
				return "", false
			},
			expectedResult: "",
			expectedError:  entity.NewError(fx.CodeInvalidConfig, "fx: provider api key is not configured"),
		},
		{
			name:     "empty environment value fails closed",
			fxConfig: fxConfigWithEnv("FX_API_KEY"),
			lookup: func(string) (string, bool) {
				return "", true
			},
			expectedResult: "",
			expectedError:  entity.NewError(fx.CodeInvalidConfig, "fx: provider api key is not configured"),
		},
		{
			name:     "whitespace environment value fails closed",
			fxConfig: fxConfigWithEnv("FX_API_KEY"),
			lookup: func(string) (string, bool) {
				return "   ", true
			},
			expectedResult: "",
			expectedError:  entity.NewError(fx.CodeInvalidConfig, "fx: provider api key is not configured"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			key, err := tc.fxConfig.APIKey(tc.lookup)
			assert.Equal(t, tc.expectedResult, key)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
