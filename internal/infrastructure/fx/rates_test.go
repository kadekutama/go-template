package fx

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestParseDecimal(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                string
		raw                 string
		maxDecimalLen       int
		expectedNumerator   int64
		expectedDenominator int64
		expectedError       error
	}

	testCases := []testCase{
		{
			name:                "two decimal places reduced",
			raw:                 "0.92",
			maxDecimalLen:       40,
			expectedNumerator:   23,
			expectedDenominator: 25,
			expectedError:       nil,
		},
		{
			name:                "integer accepted",
			raw:                 "1",
			maxDecimalLen:       40,
			expectedNumerator:   1,
			expectedDenominator: 1,
			expectedError:       nil,
		},
		{
			name:                "half accepted",
			raw:                 "0.5",
			maxDecimalLen:       40,
			expectedNumerator:   1,
			expectedDenominator: 2,
			expectedError:       nil,
		},
		{
			name:                "eighteen decimal places accepted",
			raw:                 "0.000000000000000001",
			maxDecimalLen:       40,
			expectedNumerator:   1,
			expectedDenominator: 1_000_000_000_000_000_000,
			expectedError:       nil,
		},
		{
			name:                "zero rejected",
			raw:                 "0",
			maxDecimalLen:       40,
			expectedNumerator:   0,
			expectedDenominator: 0,
			expectedError:       entity.NewError(CodeInvalidResponse, "fx: provider quote must be positive"),
		},
		{
			name:                "negative rejected",
			raw:                 "-1",
			maxDecimalLen:       40,
			expectedNumerator:   0,
			expectedDenominator: 0,
			expectedError:       entity.NewError(CodeInvalidResponse, "fx: provider quote is not a positive decimal"),
		},
		{
			name:                "exponent rejected",
			raw:                 "1e-3",
			maxDecimalLen:       40,
			expectedNumerator:   0,
			expectedDenominator: 0,
			expectedError:       entity.NewError(CodeInvalidResponse, "fx: provider quote is not a positive decimal"),
		},
		{
			name:                "fraction rejected",
			raw:                 "1/2",
			maxDecimalLen:       40,
			expectedNumerator:   0,
			expectedDenominator: 0,
			expectedError:       entity.NewError(CodeInvalidResponse, "fx: provider quote is not a positive decimal"),
		},
		{
			name:                "leading dot rejected",
			raw:                 ".92",
			maxDecimalLen:       40,
			expectedNumerator:   0,
			expectedDenominator: 0,
			expectedError:       entity.NewError(CodeInvalidResponse, "fx: provider quote is not a positive decimal"),
		},
		{
			name:                "trailing dot rejected",
			raw:                 "1.",
			maxDecimalLen:       40,
			expectedNumerator:   0,
			expectedDenominator: 0,
			expectedError:       entity.NewError(CodeInvalidResponse, "fx: provider quote is not a positive decimal"),
		},
		{
			name:                "leading zero integer rejected",
			raw:                 "01",
			maxDecimalLen:       40,
			expectedNumerator:   0,
			expectedDenominator: 0,
			expectedError:       entity.NewError(CodeInvalidResponse, "fx: provider quote is not a positive decimal"),
		},
		{
			name:                "leading zero fraction rejected",
			raw:                 "00.92",
			maxDecimalLen:       40,
			expectedNumerator:   0,
			expectedDenominator: 0,
			expectedError:       entity.NewError(CodeInvalidResponse, "fx: provider quote is not a positive decimal"),
		},
		{
			name:                "empty rejected",
			raw:                 "",
			maxDecimalLen:       40,
			expectedNumerator:   0,
			expectedDenominator: 0,
			expectedError:       entity.NewError(CodeInvalidResponse, "fx: provider quote is not a positive decimal"),
		},
		{
			name:                "over-long input rejected",
			raw:                 strings.Repeat("9", 41),
			maxDecimalLen:       40,
			expectedNumerator:   0,
			expectedDenominator: 0,
			expectedError:       entity.NewError(CodeInvalidResponse, "fx: provider quote is not a positive decimal"),
		},
		{
			name:                "zero decimal length cap rejects",
			raw:                 "1",
			maxDecimalLen:       0,
			expectedNumerator:   0,
			expectedDenominator: 0,
			expectedError:       entity.NewError(CodeInvalidResponse, "fx: provider quote is not a positive decimal"),
		},
		{
			name:                "negative decimal length cap rejects",
			raw:                 "1",
			maxDecimalLen:       -1,
			expectedNumerator:   0,
			expectedDenominator: 0,
			expectedError:       entity.NewError(CodeInvalidResponse, "fx: provider quote is not a positive decimal"),
		},
		{
			name:                "int64 numerator overflow rejected",
			raw:                 "99999999999999999999",
			maxDecimalLen:       40,
			expectedNumerator:   0,
			expectedDenominator: 0,
			expectedError:       entity.NewError(CodeInvalidResponse, "fx: provider quote exceeds fixed-point range"),
		},
		{
			name:                "int64 denominator overflow rejected",
			raw:                 "0.0000000000000000001",
			maxDecimalLen:       40,
			expectedNumerator:   0,
			expectedDenominator: 0,
			expectedError:       entity.NewError(CodeInvalidResponse, "fx: provider quote exceeds fixed-point range"),
		},
		{
			name:                "leading whitespace rejected",
			raw:                 " 1",
			maxDecimalLen:       40,
			expectedNumerator:   0,
			expectedDenominator: 0,
			expectedError:       entity.NewError(CodeInvalidResponse, "fx: provider quote is not a positive decimal"),
		},
		{
			name:                "plus sign rejected",
			raw:                 "+1",
			maxDecimalLen:       40,
			expectedNumerator:   0,
			expectedDenominator: 0,
			expectedError:       entity.NewError(CodeInvalidResponse, "fx: provider quote is not a positive decimal"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			numerator, denominator, err := parseDecimal(tc.raw, tc.maxDecimalLen)
			assert.Equal(t, tc.expectedNumerator, numerator)
			assert.Equal(t, tc.expectedDenominator, denominator)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestParseLiveRateAccepts(t *testing.T) {
	t.Parallel()

	baseNow := testNow()
	basePolicy := parsePolicy{
		TTL:           time.Hour,
		MaxQuoteAge:   48 * time.Hour,
		MaxFutureSkew: 5 * time.Minute,
		MaxDecimalLen: 40,
	}

	type testCase struct {
		name          string
		body          []byte
		base          valueobject.AssetCode
		quote         valueobject.AssetCode
		source        string
		now           time.Time
		policy        parsePolicy
		expectedRate  valueobject.FxRate
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "numeric quote accepted",
			body:          liveBody(true, "USD", "USDEUR", "0.92", baseNow.Unix()),
			base:          "USD",
			quote:         "EUR",
			source:        testSource,
			now:           baseNow,
			policy:        basePolicy,
			expectedRate:  expectedLiveRate(baseNow, "EUR", 23, 25),
			expectedError: nil,
		},
		{
			name:          "string quote accepted",
			body:          liveBody(true, "USD", "USDEUR", `"0.92"`, baseNow.Unix()),
			base:          "USD",
			quote:         "EUR",
			source:        testSource,
			now:           baseNow,
			policy:        basePolicy,
			expectedRate:  expectedLiveRate(baseNow, "EUR", 23, 25),
			expectedError: nil,
		},
		{
			name:          "integer quote accepted",
			body:          liveBody(true, "USD", "USDEUR", "1", baseNow.Unix()),
			base:          "USD",
			quote:         "EUR",
			source:        testSource,
			now:           baseNow,
			policy:        basePolicy,
			expectedRate:  expectedLiveRate(baseNow, "EUR", 1, 1),
			expectedError: nil,
		},
		{
			name:   "configured ttl is stamped",
			body:   liveBody(true, "USD", "USDEUR", "0.92", baseNow.Unix()),
			base:   "USD",
			quote:  "EUR",
			source: testSource,
			now:    baseNow,
			policy: parsePolicy{TTL: 2 * time.Hour, MaxQuoteAge: 48 * time.Hour, MaxFutureSkew: 5 * time.Minute, MaxDecimalLen: 40},
			expectedRate: func() valueobject.FxRate {
				rate := expectedLiveRate(baseNow, "EUR", 23, 25)
				rate.TTL = 2 * time.Hour

				return rate
			}(),
			expectedError: nil,
		},
		{
			name:          "lowercase source and quote key accepted",
			body:          liveBody(true, "usd", "usdeur", "0.92", baseNow.Unix()),
			base:          "USD",
			quote:         "EUR",
			source:        testSource,
			now:           baseNow,
			policy:        basePolicy,
			expectedRate:  expectedLiveRate(baseNow, "EUR", 23, 25),
			expectedError: nil,
		},
		{
			name:          "exact max age boundary accepted",
			body:          liveBody(true, "USD", "USDEUR", "0.92", baseNow.Add(-48*time.Hour).Unix()),
			base:          "USD",
			quote:         "EUR",
			source:        testSource,
			now:           baseNow,
			policy:        basePolicy,
			expectedRate:  expectedLiveRate(baseNow, "EUR", 23, 25),
			expectedError: nil,
		},
		{
			name:          "exact future skew boundary accepted",
			body:          liveBody(true, "USD", "USDEUR", "0.92", baseNow.Add(5*time.Minute).Unix()),
			base:          "USD",
			quote:         "EUR",
			source:        testSource,
			now:           baseNow,
			policy:        basePolicy,
			expectedRate:  expectedLiveRate(baseNow, "EUR", 23, 25),
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rate, err := parseLiveRate(tc.body, tc.base, tc.quote, tc.source, tc.now, tc.policy)
			assert.Equal(t, tc.expectedRate, rate)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestParseLiveRateRejects(t *testing.T) {
	t.Parallel()

	baseNow := testNow()
	basePolicy := parsePolicy{
		TTL:           time.Hour,
		MaxQuoteAge:   48 * time.Hour,
		MaxFutureSkew: 5 * time.Minute,
		MaxDecimalLen: 40,
	}

	type testCase struct {
		name          string
		body          []byte
		base          valueobject.AssetCode
		quote         valueobject.AssetCode
		source        string
		now           time.Time
		policy        parsePolicy
		expectedRate  valueobject.FxRate
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "configured future skew is honored",
			body:          liveBody(true, "USD", "USDEUR", "0.92", baseNow.Add(2*time.Minute).Unix()),
			base:          "USD",
			quote:         "EUR",
			source:        testSource,
			now:           baseNow,
			policy:        parsePolicy{TTL: time.Hour, MaxQuoteAge: 48 * time.Hour, MaxFutureSkew: time.Minute, MaxDecimalLen: 40},
			expectedRate:  valueobject.FxRate{},
			expectedError: entity.NewError(CodeInvalidResponse, "fx: provider quote timestamp is in the future"),
		},
		{
			name:          "configured max quote age is honored",
			body:          liveBody(true, "USD", "USDEUR", "0.92", baseNow.Add(-10*time.Minute).Unix()),
			base:          "USD",
			quote:         "EUR",
			source:        testSource,
			now:           baseNow,
			policy:        parsePolicy{TTL: time.Hour, MaxQuoteAge: 5 * time.Minute, MaxFutureSkew: 5 * time.Minute, MaxDecimalLen: 40},
			expectedRate:  valueobject.FxRate{},
			expectedError: entity.NewError(CodeInvalidResponse, "fx: provider quote timestamp is too old"),
		},
		{
			name:          "configured decimal length cap is honored",
			body:          liveBody(true, "USD", "USDEUR", "0.92", baseNow.Unix()),
			base:          "USD",
			quote:         "EUR",
			source:        testSource,
			now:           baseNow,
			policy:        parsePolicy{TTL: time.Hour, MaxQuoteAge: 48 * time.Hour, MaxFutureSkew: 5 * time.Minute, MaxDecimalLen: 3},
			expectedRate:  valueobject.FxRate{},
			expectedError: entity.NewError(CodeInvalidResponse, "fx: provider quote is not a positive decimal"),
		},
		{
			name:          "provider rejection reported",
			body:          liveBody(false, "USD", "USDEUR", "0.92", baseNow.Unix()),
			base:          "USD",
			quote:         "EUR",
			source:        testSource,
			now:           baseNow,
			policy:        basePolicy,
			expectedRate:  valueobject.FxRate{},
			expectedError: entity.NewError(CodeProviderRejected, msgProviderRejected),
		},
		{
			name:          "malformed body rejected",
			body:          []byte(`{"success":true,`),
			base:          "USD",
			quote:         "EUR",
			source:        testSource,
			now:           baseNow,
			policy:        basePolicy,
			expectedRate:  valueobject.FxRate{},
			expectedError: entity.NewError(CodeInvalidResponse, "fx: provider response does not parse"),
		},
		{
			name:          "source mismatch rejected",
			body:          liveBody(true, "GBP", "USDEUR", "0.92", baseNow.Unix()),
			base:          "USD",
			quote:         "EUR",
			source:        testSource,
			now:           baseNow,
			policy:        basePolicy,
			expectedRate:  valueobject.FxRate{},
			expectedError: entity.NewError(CodeInvalidResponse, "fx: provider response source does not match the request"),
		},
		{
			name:          "missing quote rejected",
			body:          liveBody(true, "USD", "USDGBP", "0.92", baseNow.Unix()),
			base:          "USD",
			quote:         "EUR",
			source:        testSource,
			now:           baseNow,
			policy:        basePolicy,
			expectedRate:  valueobject.FxRate{},
			expectedError: entity.NewError(CodeInvalidResponse, "fx: provider response is missing the requested quote"),
		},
		{
			name:          "zero quote rejected",
			body:          liveBody(true, "USD", "USDEUR", "0", baseNow.Unix()),
			base:          "USD",
			quote:         "EUR",
			source:        testSource,
			now:           baseNow,
			policy:        basePolicy,
			expectedRate:  valueobject.FxRate{},
			expectedError: entity.NewError(CodeInvalidResponse, "fx: provider quote must be positive"),
		},
		{
			name:          "negative quote rejected",
			body:          liveBody(true, "USD", "USDEUR", "-0.92", baseNow.Unix()),
			base:          "USD",
			quote:         "EUR",
			source:        testSource,
			now:           baseNow,
			policy:        basePolicy,
			expectedRate:  valueobject.FxRate{},
			expectedError: entity.NewError(CodeInvalidResponse, "fx: provider quote is not a positive decimal"),
		},
		{
			name:          "overflowing quote rejected",
			body:          liveBody(true, "USD", "USDEUR", "99999999999999999999", baseNow.Unix()),
			base:          "USD",
			quote:         "EUR",
			source:        testSource,
			now:           baseNow,
			policy:        basePolicy,
			expectedRate:  valueobject.FxRate{},
			expectedError: entity.NewError(CodeInvalidResponse, "fx: provider quote exceeds fixed-point range"),
		},
		{
			name:          "missing timestamp rejected",
			body:          liveBody(true, "USD", "USDEUR", "0.92", 0),
			base:          "USD",
			quote:         "EUR",
			source:        testSource,
			now:           baseNow,
			policy:        basePolicy,
			expectedRate:  valueobject.FxRate{},
			expectedError: entity.NewError(CodeInvalidResponse, "fx: provider quote timestamp is required"),
		},
		{
			name:          "stale timestamp rejected",
			body:          liveBody(true, "USD", "USDEUR", "0.92", baseNow.Add(-49*time.Hour).Unix()),
			base:          "USD",
			quote:         "EUR",
			source:        testSource,
			now:           baseNow,
			policy:        basePolicy,
			expectedRate:  valueobject.FxRate{},
			expectedError: entity.NewError(CodeInvalidResponse, "fx: provider quote timestamp is too old"),
		},
		{
			name:          "future timestamp rejected",
			body:          liveBody(true, "USD", "USDEUR", "0.92", baseNow.Add(6*time.Minute).Unix()),
			base:          "USD",
			quote:         "EUR",
			source:        testSource,
			now:           baseNow,
			policy:        basePolicy,
			expectedRate:  valueobject.FxRate{},
			expectedError: entity.NewError(CodeInvalidResponse, "fx: provider quote timestamp is in the future"),
		},
		{
			name:          "equal pair rejected",
			body:          liveBody(true, "USD", "USDUSD", "0.92", baseNow.Unix()),
			base:          "USD",
			quote:         "USD",
			source:        testSource,
			now:           baseNow,
			policy:        basePolicy,
			expectedRate:  valueobject.FxRate{},
			expectedError: entity.NewError(CodePairInvalid, "fx rate pair requires distinct base and quote codes"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rate, err := parseLiveRate(tc.body, tc.base, tc.quote, tc.source, tc.now, tc.policy)
			assert.Equal(t, tc.expectedRate, rate)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
