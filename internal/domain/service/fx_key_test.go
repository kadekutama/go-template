package service_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestBuildFXRateKey(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		base          valueobject.AssetCode
		quote         valueobject.AssetCode
		expectedKey   string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid pair",
			base:          "USD",
			quote:         "EUR",
			expectedKey:   "fx:USD:EUR",
			expectedError: nil,
		},
		{
			name:          "outer whitespace is canonicalized",
			base:          " USD ",
			quote:         " EUR ",
			expectedKey:   "fx:USD:EUR",
			expectedError: nil,
		},
		{
			name:          "empty base rejected",
			base:          "",
			quote:         "EUR",
			expectedKey:   "",
			expectedError: entity.NewError("FX_PAIR_INVALID", "fx rate pair requires distinct base and quote codes"),
		},
		{
			name:          "whitespace-only quote rejected",
			base:          "USD",
			quote:         "   ",
			expectedKey:   "",
			expectedError: entity.NewError("FX_PAIR_INVALID", "fx rate pair requires distinct base and quote codes"),
		},
		{
			name:          "equal pair rejected",
			base:          "USD",
			quote:         "USD",
			expectedKey:   "",
			expectedError: entity.NewError("FX_PAIR_INVALID", "fx rate pair requires distinct base and quote codes"),
		},
		{
			name:          "colon in segment rejected",
			base:          "USD",
			quote:         "EU:R",
			expectedKey:   "",
			expectedError: entity.NewError("ISOLATION_KEY_INVALID", "isolation key segment must not contain ':'"),
		},
		{
			name:          "control character rejected",
			base:          "US\nD",
			quote:         "EUR",
			expectedKey:   "",
			expectedError: entity.NewError("ISOLATION_KEY_INVALID", "isolation key segment must not contain control characters"),
		},
		{
			name:          "over-long segment rejected",
			base:          valueobject.AssetCode(strings.Repeat("A", 129)),
			quote:         "EUR",
			expectedKey:   "",
			expectedError: entity.NewError("ISOLATION_KEY_INVALID", "isolation key segment is too long"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actualKey, err := service.BuildFXRateKey(tc.base, tc.quote)
			assert.Equal(t, tc.expectedKey, actualKey)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestParseFXRateKey(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		key           string
		expectedBase  valueobject.AssetCode
		expectedQuote valueobject.AssetCode
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid key",
			key:           "fx:USD:EUR",
			expectedBase:  "USD",
			expectedQuote: "EUR",
			expectedError: nil,
		},
		{
			name:          "surrounding spaces canonicalized",
			key:           "fx: USD : EUR ",
			expectedBase:  "USD",
			expectedQuote: "EUR",
			expectedError: nil,
		},
		{
			name:          "wrong prefix rejected",
			key:           "rate:USD:EUR",
			expectedBase:  "",
			expectedQuote: "",
			expectedError: entity.NewError("ISOLATION_KEY_INVALID", "fx rate key must start with 'fx:'"),
		},
		{
			name:          "missing quote rejected",
			key:           "fx:USD",
			expectedBase:  "",
			expectedQuote: "",
			expectedError: entity.NewError("ISOLATION_KEY_INVALID", "fx rate key must have 2 segments"),
		},
		{
			name:          "extra segment rejected",
			key:           "fx:USD:EUR:GBP",
			expectedBase:  "",
			expectedQuote: "",
			expectedError: entity.NewError("ISOLATION_KEY_INVALID", "fx rate key must have 2 segments"),
		},
		{
			name:          "empty segment rejected",
			key:           "fx::EUR",
			expectedBase:  "",
			expectedQuote: "",
			expectedError: entity.NewError("ISOLATION_KEY_INVALID", "isolation key segment is required"),
		},
		{
			name:          "equal pair rejected",
			key:           "fx:USD:USD",
			expectedBase:  "",
			expectedQuote: "",
			expectedError: entity.NewError("FX_PAIR_INVALID", "fx rate pair requires distinct base and quote codes"),
		},
		{
			name:          "control character rejected",
			key:           "fx:US\x00D:EUR",
			expectedBase:  "",
			expectedQuote: "",
			expectedError: entity.NewError("ISOLATION_KEY_INVALID", "isolation key segment must not contain control characters"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actualBase, actualQuote, err := service.ParseFXRateKey(tc.key)
			assert.Equal(t, tc.expectedBase, actualBase)
			assert.Equal(t, tc.expectedQuote, actualQuote)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
