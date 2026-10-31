package fx

import (
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// decimalPattern is the only accepted quote grammar: a non-negative decimal
// with no sign, exponent, fraction, or leading zero. It is a package-level
// compiled expression because it is immutable and used on every parse.
var decimalPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]+)?$`)

// parsePolicy bundles the configured response-parsing bounds so parseLiveRate
// stays pure and no bound is hard-coded in the adapter.
type parsePolicy struct {
	TTL           time.Duration
	MaxQuoteAge   time.Duration
	MaxFutureSkew time.Duration
	MaxDecimalLen int
}

// liveResponse is the exchangerate.host-compatible /live payload. Quote values
// decode as jsonparser.Number so a bare number and a numeric string both
// preserve their literal spelling (no float round-trip); parseDecimal owns the
// strict decimal grammar.
type liveResponse struct {
	Success   bool                         `json:"success"`
	Source    string                       `json:"source"`
	Quotes    map[string]jsonparser.Number `json:"quotes"`
	Timestamp int64                        `json:"timestamp"`
}

// parseLiveRate converts one provider payload into an exact fixed-point rate.
// It is pure: no clock, cache, or network access. The vendor timestamp is a
// sanity bound; QuotedAt is the observation instant supplied by the caller.
func parseLiveRate(
	body []byte,
	base, quote valueobject.AssetCode,
	source string,
	now time.Time,
	policy parsePolicy,
) (valueobject.FxRate, error) {
	var payload liveResponse
	if err := jsonparser.Unmarshal(body, &payload); err != nil {
		return valueobject.FxRate{}, entity.NewError(CodeInvalidResponse, "fx: provider response does not parse")
	}

	if !payload.Success {
		return valueobject.FxRate{}, entity.NewError(CodeProviderRejected, msgProviderRejected)
	}

	if !strings.EqualFold(payload.Source, string(base)) {
		return valueobject.FxRate{}, entity.NewError(CodeInvalidResponse, "fx: provider response source does not match the request")
	}

	if err := validateQuoteTimestamp(payload.Timestamp, now, policy.MaxFutureSkew, policy.MaxQuoteAge); err != nil {
		return valueobject.FxRate{}, err
	}

	quoteKey := strings.ToUpper(string(base) + string(quote))

	raw, ok := payload.Quotes[quoteKey]
	if !ok {
		for key, value := range payload.Quotes {
			if strings.EqualFold(key, quoteKey) {
				raw, ok = value, true

				break
			}
		}
	}

	if !ok {
		return valueobject.FxRate{}, entity.NewError(CodeInvalidResponse, "fx: provider response is missing the requested quote")
	}

	numerator, denominator, err := parseDecimal(string(raw), policy.MaxDecimalLen)
	if err != nil {
		return valueobject.FxRate{}, err
	}

	pair, err := valueobject.NewFxPair(base, quote)
	if err != nil {
		return valueobject.FxRate{}, entity.NewError(CodePairInvalid, "fx rate pair requires distinct base and quote codes")
	}

	rate := valueobject.FxRate{
		ID:             "fx:" + string(base) + ":" + string(quote) + ":" + strconv.FormatInt(now.UnixNano(), 10),
		Pair:           pair,
		Numerator:      numerator,
		Denominator:    denominator,
		Source:         source,
		QuotedAt:       now,
		TTL:            policy.TTL,
		RoundingPolicy: roundingPolicy,
	}
	if err := rate.Validate(); err != nil {
		return valueobject.FxRate{}, entity.NewError(CodeRateInvalid, "fx: parsed rate is invalid: "+err.Error())
	}

	return rate, nil
}

// parseDecimal converts a strict positive decimal string into a reduced
// int64 fraction with big.Rat: exact reduction, no float, and no manual gcd
// arithmetic. Signs, exponents, fractions, leading zeros, over-long input,
// zero, and int64 overflow are rejected.
func parseDecimal(raw string, maxDecimalLen int) (int64, int64, error) {
	if raw == "" || len(raw) > maxDecimalLen || !decimalPattern.MatchString(raw) {
		return 0, 0, entity.NewError(CodeInvalidResponse, "fx: provider quote is not a positive decimal")
	}

	value, ok := new(big.Rat).SetString(raw)
	if !ok {
		return 0, 0, entity.NewError(CodeInvalidResponse, "fx: provider quote is not a positive decimal")
	}

	if value.Sign() == 0 {
		return 0, 0, entity.NewError(CodeInvalidResponse, "fx: provider quote must be positive")
	}

	numerator := value.Num()
	denominator := value.Denom()

	if !numerator.IsInt64() || !denominator.IsInt64() {
		return 0, 0, entity.NewError(CodeInvalidResponse, "fx: provider quote exceeds fixed-point range")
	}

	return numerator.Int64(), denominator.Int64(), nil
}

// validateQuoteTimestamp bounds the vendor timestamp around the observation
// instant so a stale or implausible feed fails closed instead of being
// re-stamped as fresh.
func validateQuoteTimestamp(timestamp int64, now time.Time, maxFutureSkew, maxQuoteAge time.Duration) error {
	if timestamp <= 0 {
		return entity.NewError(CodeInvalidResponse, "fx: provider quote timestamp is required")
	}

	vendorTime := time.Unix(timestamp, 0).UTC()
	if vendorTime.After(now.Add(maxFutureSkew)) {
		return entity.NewError(CodeInvalidResponse, "fx: provider quote timestamp is in the future")
	}

	if vendorTime.Before(now.Add(-maxQuoteAge)) {
		return entity.NewError(CodeInvalidResponse, "fx: provider quote timestamp is too old")
	}

	return nil
}
