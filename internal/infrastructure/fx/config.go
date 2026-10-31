// Package fx adapts an exchangerate.host-compatible FX rate provider behind
// the application FxRateProvider port (E10-T02). It owns fixed-point parsing,
// cache-aside freshness, egress admission, circuit breaking, and visible
// stale fallback; no provider SDK type crosses the port.
package fx

import (
	"context"
	"net/url"
	"strings"
	"time"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/coalesce"
	"github.com/kadekutama/go-template/internal/infrastructure/egress"
	"github.com/kadekutama/go-template/internal/infrastructure/httpclient"
	kernelres "github.com/kadekutama/go-template/internal/shared/kernel/resilience"
	"github.com/kadekutama/go-template/internal/shared/kernel/validate"
)

// Adapter-local typed error codes. They are stable UPPER_SNAKE tokens mapped
// by the application/interface layers onto the AppError envelope.
const (
	// CodeInvalidConfig reports missing or contradictory constructor params.
	CodeInvalidConfig = "FX_INVALID_CONFIG"
	// CodePairInvalid reports an empty, equal, or unsafe currency pair.
	CodePairInvalid = "FX_PAIR_INVALID"
	// CodeInvalidResponse reports an unusable provider payload.
	CodeInvalidResponse = "FX_INVALID_RESPONSE"
	// CodeRateInvalid reports a parsed rate that fails FxRate.Validate.
	CodeRateInvalid = "FX_RATE_INVALID"
	// CodeProviderUnavailable reports a transport/5xx/429/breaker failure.
	CodeProviderUnavailable = "FX_PROVIDER_UNAVAILABLE"
	// CodeProviderRejected reports a permanent provider rejection.
	CodeProviderRejected = "FX_PROVIDER_REJECTED"
	// CodeEgressUnavailable reports an egress denial/store failure.
	CodeEgressUnavailable = "FX_EGRESS_UNAVAILABLE"
	// CodeRateUnavailable reports that no rate exists for the requested pair.
	CodeRateUnavailable = "FX_RATE_UNAVAILABLE"
)

// Protocol and domain invariants. Runtime settings arrive through Params from
// the config flow (fx.live_path, fx.fetch_timeout_sec, fx.max_response_bytes,
// fx.max_future_skew_sec, fx.max_decimal_len); only policy identifiers and
// money-semantic constants stay hard-coded here.
const (
	egressProvider  = "fx"
	egressOperation = "rates"
	coalesceScope   = "global"
	roundingPolicy  = "half-up"
)

// RateCache is the typed hybrid-cache view the adapter needs. It is
// consumer-owned so the adapter never constructs cache layers or imports the
// hybrid implementation.
type RateCache interface {
	Get(ctx context.Context, key string) (valueobject.FxRate, error)
	Set(ctx context.Context, key string, value valueobject.FxRate, ttl time.Duration) error
}

// Coalescer collapses concurrent identical cache-miss refreshes. It shares
// bytes because the concrete E09.1-T04 coalescer is codec-agnostic.
type Coalescer interface {
	Do(ctx context.Context, key coalesce.Key, fn func(context.Context) ([]byte, error)) ([]byte, error)
}

// Params wires a Provider (Parameter Object pattern). Data fields carry
// validate tags; dependency seams are checked explicitly so callers can rely
// on typed FX_INVALID_CONFIG errors instead of (nil, nil).
type Params struct {
	// BaseURL is the exchangerate.host-compatible API origin.
	BaseURL string `validate:"required,url"`
	// APIKey is the resolved provider credential. It is never logged or
	// stored in configuration; the config resolver reads it from the
	// environment variable named by fx.api_key_env.
	APIKey string `validate:"required"`
	// Source labels the accepted rate (for example exchangerate.host).
	Source string `validate:"required,min=1,max=64"`
	// LivePath is the configured provider request path (fx.live_path).
	LivePath string `validate:"required,startswith=/"`
	// HTTPClient performs the outbound provider GET.
	HTTPClient httpclient.Doer `validate:"-"`
	// Cache stores the complete FxRate for logical freshness and stale
	// fallback.
	Cache RateCache `validate:"-"`
	// Egress admits every provider attempt.
	Egress egress.Waiter `validate:"-"`
	// Breaker guards the provider HTTP dependency.
	Breaker kernelres.Breaker `validate:"-"`
	// Coalescer collapses concurrent identical cache-miss refreshes.
	Coalescer Coalescer `validate:"-"`
	// Clock supplies the observation instant; no local clock is created.
	Clock appport.Clock `validate:"-"`
	// RetryPolicy bounds provider attempts. It is explicit so tests can use
	// tiny backoffs and production can raise the attempt budget.
	RetryPolicy kernelres.RetryPolicy `validate:"-"`
	// FetchTimeout bounds one refresh loop (fx.fetch_timeout_sec).
	FetchTimeout time.Duration `validate:"-"`
	// MaxResponseBytes caps the provider response body
	// (fx.max_response_bytes).
	MaxResponseBytes int64 `validate:"-"`
	// MaxFutureSkew tolerates vendor clock skew
	// (fx.max_future_skew_sec).
	MaxFutureSkew time.Duration `validate:"-"`
	// MaxDecimalLen caps the accepted quote decimal string
	// (fx.max_decimal_len).
	MaxDecimalLen int `validate:"-"`
	// TTL is the logical freshness window (cache.ttls.fx_sec).
	TTL time.Duration `validate:"-"`
	// StaleRetention is the physical cache retention; it must exceed TTL so a
	// last-known rate survives logical expiry.
	StaleRetention time.Duration `validate:"-"`
	// MaxQuoteAge bounds how old a vendor timestamp may be.
	MaxQuoteAge time.Duration `validate:"-"`
}

// Validate checks construction params deterministically. Data-tag violations
// and every seam/numeric invariant become typed FX_INVALID_CONFIG errors.
func (p Params) Validate() error {
	if p.HTTPClient == nil {
		return configError("fx: http client is required")
	}

	if p.Cache == nil {
		return configError("fx: rate cache is required")
	}

	if p.Egress == nil {
		return configError("fx: egress limiter is required")
	}

	if p.Breaker == nil {
		return configError("fx: circuit breaker is required")
	}

	if p.Coalescer == nil {
		return configError("fx: coalescer is required")
	}

	if p.Clock == nil {
		return configError("fx: clock is required")
	}

	if p.FetchTimeout <= 0 {
		return configError("fx: fetch timeout must be positive")
	}

	if p.MaxResponseBytes <= 0 {
		return configError("fx: max response bytes must be positive")
	}

	if p.MaxFutureSkew < 0 {
		return configError("fx: max future skew must be non-negative")
	}

	if p.MaxDecimalLen <= 0 {
		return configError("fx: max decimal length must be positive")
	}

	if p.TTL <= 0 {
		return configError("fx: logical ttl must be positive")
	}

	if p.StaleRetention <= 0 {
		return configError("fx: stale retention must be positive")
	}

	if p.StaleRetention <= p.TTL {
		return configError("fx: stale retention must exceed the logical ttl")
	}

	if p.MaxQuoteAge <= 0 {
		return configError("fx: max quote age must be positive")
	}

	if err := p.RetryPolicy.Validate(); err != nil {
		return configError("fx: retry policy is invalid: " + err.Error())
	}

	if err := validate.Struct("fx", "params", p); err != nil {
		return configError("fx: params are invalid: " + err.Error())
	}

	return nil
}

// Provider implements appport.FxRateProvider.
type Provider struct {
	endpoint         *url.URL
	apiKey           string
	source           string
	httpClient       httpclient.Doer
	cache            RateCache
	egress           egress.Waiter
	breaker          kernelres.Breaker
	coalescer        Coalescer
	clock            appport.Clock
	retryPolicy      kernelres.RetryPolicy
	fetchTimeout     time.Duration
	maxResponseBytes int64
	maxFutureSkew    time.Duration
	maxDecimalLen    int
	ttl              time.Duration
	staleRetention   time.Duration
	maxQuoteAge      time.Duration
}

// NewProvider validates params and builds an immutable adapter. It never
// returns (nil, nil).
func NewProvider(params Params) (*Provider, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	endpoint, err := url.Parse(strings.TrimRight(params.BaseURL, "/") + params.LivePath)
	if err != nil {
		return nil, configError("fx: base url is invalid")
	}

	return &Provider{
		endpoint:         endpoint,
		apiKey:           params.APIKey,
		source:           params.Source,
		httpClient:       params.HTTPClient,
		cache:            params.Cache,
		egress:           params.Egress,
		breaker:          params.Breaker,
		coalescer:        params.Coalescer,
		clock:            params.Clock,
		retryPolicy:      params.RetryPolicy,
		fetchTimeout:     params.FetchTimeout,
		maxResponseBytes: params.MaxResponseBytes,
		maxFutureSkew:    params.MaxFutureSkew,
		maxDecimalLen:    params.MaxDecimalLen,
		ttl:              params.TTL,
		staleRetention:   params.StaleRetention,
		maxQuoteAge:      params.MaxQuoteAge,
	}, nil
}

// configError builds one typed constructor failure without leaking a secret.
func configError(message string) error {
	return entity.NewError(CodeInvalidConfig, message)
}
