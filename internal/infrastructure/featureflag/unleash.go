package featureflag

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/Unleash/unleash-go-sdk/v6"
	"github.com/Unleash/unleash-go-sdk/v6/api"
	unleashContext "github.com/Unleash/unleash-go-sdk/v6/context"
	"github.com/open-feature/go-sdk/openfeature"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	infraresilience "github.com/kadekutama/go-template/internal/infrastructure/resilience"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
	kernelresilience "github.com/kadekutama/go-template/internal/shared/kernel/resilience"
)

// gatedTransport is the single choke point for every outbound Unleash HTTP
// request. Composition per attempt is normative (E09.1-T03): egress Wait
// first, then one breaker-wrapped base round trip. Limiter errors are terminal
// and never count as breaker failures; breaker-open fast-fails without
// touching the network.
type gatedTransport struct {
	egress  EgressWaiter
	breaker kernelresilience.Breaker
	base    http.RoundTripper
	logger  log.Logger
}

// NewGatedTransport builds the egress-gated transport. Egress, Breaker, and
// base are required: nil is rejected, never silently defaulted (production
// passes http.DefaultTransport explicitly). Logger is nil-tolerant: a nil
// logger disables bookkeeping-failure logging.
func NewGatedTransport(egressWaiter EgressWaiter, breaker kernelresilience.Breaker, base http.RoundTripper, logger log.Logger) (http.RoundTripper, error) {
	if egressWaiter == nil {
		return nil, newConfigError("featureflag: gated transport egress is required")
	}

	if breaker == nil {
		return nil, newConfigError("featureflag: gated transport breaker is required")
	}

	if base == nil {
		return nil, newConfigError("featureflag: gated transport base is required")
	}

	return &gatedTransport{egress: egressWaiter, breaker: breaker, base: base, logger: logger}, nil
}

// RoundTrip admits one Unleash HTTP request through the egress quota and the
// breaker. It never makes an unmetered request: a limiter denial, timeout, or
// store failure returns before the base transport runs.
//
// Breaker bookkeeping stays separate from the network call so the response
// body flows directly back to the Unleash client (which closes it): an open
// breaker fast-fails before dialing, while closed/half-open admits one base
// round trip and records the outcome without a second request.
func (t *gatedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t == nil || t.egress == nil || t.breaker == nil || t.base == nil {
		return nil, newConfigError("featureflag: gated transport is not initialized")
	}

	if err := req.Context().Err(); err != nil {
		return nil, err
	}

	if err := t.egress.Wait(req.Context(), EgressProvider, EgressOperation); err != nil {
		return nil, err
	}

	if t.breaker.State() == kernelresilience.StateOpen {
		return nil, fmt.Errorf("%w (breaker %q)", infraresilience.ErrBreakerOpen, "unleash-flags")
	}

	resp, err := t.base.RoundTrip(req.Clone(req.Context()))
	if err != nil {
		t.record(req.Context(), err)

		return nil, err
	}

	t.record(req.Context(), nil)

	return resp, nil
}

// record books one breaker outcome. Bookkeeping failures never mask the
// transport result; they are logged (without request details) so a degraded
// breaker stays visible.
func (t *gatedTransport) record(ctx context.Context, err error) {
	if recordErr := t.breaker.Execute(ctx, "unleash-flags", "", func(context.Context) error {
		return err
	}); recordErr != nil && !errors.Is(recordErr, err) && t.logger != nil {
		t.logger.Warn(ctx, "featureflag: breaker bookkeeping failed", "cause", recordErr)
	}
}

// UnleashEvaluatorParams wires the production OpenFeature + Unleash evaluator
// (Parameter Object pattern).
type UnleashEvaluatorParams struct {
	// UnleashURL is the Unleash server base URL.
	UnleashURL string
	// AppName is the Unleash client application name.
	AppName string
	// InstanceID identifies this client instance. Empty uses hostname.
	InstanceID string
	// Environment scopes evaluation. Empty is global.
	Environment string
	// RefreshInterval is the 10s toggle refresh bound. Must be positive.
	RefreshInterval time.Duration
	// ClientToken authenticates SDK polling via the Authorization header.
	// Required against real servers (API tokens enforced); empty only for
	// auth-disabled instances. Never logged or embedded in errors.
	ClientToken string
	// InitTimeout bounds the initial toggle load wait. Must be positive and
	// operator-tunable: a healthy server answers in milliseconds, while an
	// unreachable server must not stall construction past this bound.
	InitTimeout time.Duration
	// HTTPClient is the egress-gated client for Unleash polling. Required.
	HTTPClient *http.Client
}

// UnleashEvaluator evaluates boolean flags through OpenFeature with the
// task-owned Unleash provider over unleash-go-sdk/v6. Evaluations read the
// locally cached toggles refreshed on RefreshInterval; every background
// refresh request flows through the gated transport, so no poll runs
// unmetered. The provider holds a dedicated per-instance client (never the
// SDK global), so shutdown stays scoped and sequential evaluators in one
// process share no mutable state.
type UnleashEvaluator struct {
	provider *unleashProvider
	client   *openfeature.Client
}

// Compile-time evaluator conformance.
var _ Evaluator = (*UnleashEvaluator)(nil)

// NewUnleashEvaluator builds the production evaluator. It initializes the
// Unleash v6 client with the gated HTTP transport and registers the
// task-owned OpenFeature provider. Metrics are disabled so background traffic
// is limited to toggle refreshes admitted by the egress quota.
func NewUnleashEvaluator(params UnleashEvaluatorParams) (*UnleashEvaluator, error) {
	if params.UnleashURL == "" || params.AppName == "" {
		return nil, newConfigError("featureflag: unleash url and app name are required")
	}

	if params.RefreshInterval <= 0 {
		return nil, newConfigError("featureflag: refresh interval must be positive")
	}

	if params.InitTimeout <= 0 {
		return nil, newConfigError("featureflag: init timeout must be positive")
	}

	if params.HTTPClient == nil {
		return nil, newConfigError("featureflag: gated http client is required")
	}

	options := []unleash.ConfigOption{
		unleash.WithUrl(params.UnleashURL),
		unleash.WithAppName(params.AppName),
		unleash.WithRefreshInterval(params.RefreshInterval),
		unleash.WithHttpClient(params.HTTPClient),
		unleash.WithDisableMetrics(true),
	}

	if params.ClientToken != "" {
		options = append(options, unleash.WithCustomHeaders(http.Header{"Authorization": {params.ClientToken}}))
	}

	if params.InstanceID != "" {
		options = append(options, unleash.WithInstanceId(params.InstanceID))
	}

	if params.Environment != "" {
		options = append(options, unleash.WithEnvironment(params.Environment))
	}

	provider, err := newUnleashProvider(options, params.InitTimeout)
	if err != nil {
		return nil, err
	}

	// Register the provider with OpenFeature synchronously so Health is Ready
	// before the first evaluation. SetProviderAndWait invokes Init (which
	// waits for the first poll through the gated transport) and waits for it.
	// Each evaluator owns a dedicated client instance, so sequential
	// evaluators in one process share no mutable state.
	if err := openfeature.SetProviderAndWait(provider); err != nil {
		return nil, fmt.Errorf("%w: %s", newProviderUnavailableError(), err.Error())
	}

	client := openfeature.NewClient("featureflag")

	return &UnleashEvaluator{provider: provider, client: client}, nil
}

// Evaluate returns the provider value for key. A nil error means fresh from
// Unleash; a non-nil error means the caller must serve fallback. Evaluation
// context carries only non-sensitive routing attributes.
func (e *UnleashEvaluator) Evaluate(ctx context.Context, key string, def bool, evalCtx appport.FlagContext) (bool, error) {
	if e == nil || e.client == nil || e.provider == nil {
		return def, newProviderUnavailableError()
	}

	if key == "" {
		return def, entity.NewError(CodeInvalidFlagKey, "featureflag: flag key is required")
	}

	if err := ctx.Err(); err != nil {
		return def, err
	}

	ofCtx := toOpenFeatureContext(evalCtx)

	value, err := e.client.BooleanValue(ctx, key, def, ofCtx)
	if err != nil {
		return def, fmt.Errorf("%w: %s", newProviderUnavailableError(), err.Error())
	}

	return value, nil
}

// Health reports provider readiness. It performs no network call and emits
// no credentials.
func (e *UnleashEvaluator) Health(_ context.Context) error {
	if e == nil || e.provider == nil {
		return newProviderUnavailableError()
	}

	if e.provider.Status() != openfeature.ReadyState {
		return newProviderUnavailableError()
	}

	return nil
}

// Close releases provider resources. It is safe to call more than once:
// provider Shutdown is compare-and-swap-guarded over the dedicated client
// instance. The evaluator closes its provider explicitly; the SDK invokes no
// shutdown callback for non-StateHandler providers. Shutdown failures
// propagate instead of being ignored.
func (e *UnleashEvaluator) Close(_ context.Context) error {
	if e == nil || e.provider == nil {
		return nil
	}

	return e.provider.Shutdown()
}

// toOpenFeatureContext converts the port evaluation context to an OpenFeature
// context. Targeting uses the user ID when present, otherwise the tenant ID.
// Tenant ID is also mirrored as an attribute so strategies can target it.
// Attributes are copied; credentials must never be placed here by callers.
func toOpenFeatureContext(evalCtx appport.FlagContext) openfeature.EvaluationContext {
	targeting := evalCtx.UserID
	if targeting == "" {
		targeting = evalCtx.TenantID
	}

	attrs := make(map[string]any, len(evalCtx.Attributes)+1)
	for key, value := range evalCtx.Attributes {
		attrs[key] = value
	}

	if evalCtx.TenantID != "" {
		attrs["tenant_id"] = evalCtx.TenantID
	}

	return openfeature.NewEvaluationContext(targeting, attrs)
}

// newProviderUnavailableError builds the typed provider-outage failure.
func newProviderUnavailableError() error {
	return entity.NewError(CodeProviderUnavailable, "featureflag: unleash provider unavailable")
}

// isBreakerOpen reports whether err is a breaker fast-fail.
func isBreakerOpen(err error) bool {
	return errors.Is(err, infraresilience.ErrBreakerOpen)
}

const (
	providerNotReady = "Provider not ready"
)

// unleashProvider is the task-owned OpenFeature provider over
// unleash-go-sdk/v6. The go-sdk-contrib unleash provider only wraps the old
// unleash-client-go/v4, so this type ports its evaluation semantics (status
// lifecycle, fallback, type-mismatch) to the v6 client. Only boolean
// evaluation carries a provider path; other kinds report type-mismatch
// exactly like the wrapped provider did.
type unleashProvider struct {
	client *unleash.Client
	// ready is the single lifecycle bit: set once the client loads toggles,
	// cleared on shutdown. The client itself is immutable after construction,
	// so the whole provider is lock-free (no mutex, no sync.Once).
	ready  atomic.Bool
	closed atomic.Bool
}

// Compile-time OpenFeature provider conformance.
var _ openfeature.FeatureProvider = (*unleashProvider)(nil)

// newUnleashProvider builds the provider with a dedicated polling client,
// starts polling immediately, and waits for the first toggle load up to
// initTimeout. Construction owns the whole startup sequence so no evaluation
// path ever observes a half-built provider: the client field is immutable and
// readiness is resolved before return. initTimeout must be positive; the
// client fetches immediately on start, so a healthy server answers in
// milliseconds, while an unreachable server only leaves the provider NotReady
// and evaluations fail closed to fallback until the client recovers.
func newUnleashProvider(options []unleash.ConfigOption, initTimeout time.Duration) (*unleashProvider, error) {
	if initTimeout <= 0 {
		return nil, newConfigError("featureflag: init timeout must be positive")
	}

	client, err := unleash.NewClient(options...)
	if err != nil {
		return nil, err
	}

	provider := &unleashProvider{
		client: client,
	}

	select {
	case <-client.Ready():
		provider.ready.Store(true)
	case <-time.After(initTimeout):
	}

	return provider, nil
}

// NOTE: the provider intentionally does not implement
// openfeature.StateHandler. Construction already resolved initial readiness
// (the SDK assumes non-StateHandler providers ready), and Shutdown below is
// a plain method our evaluator calls directly — so there is no Init function
// and no half-built state. The SDK skips shutdown callbacks for replaced
// non-StateHandler providers; every evaluator in this repository closes its
// provider explicitly, so no client leaks.

// Status reports the provider lifecycle state, derived from the readiness bit.
func (p *unleashProvider) Status() openfeature.State {
	if p.ready.Load() {
		return openfeature.ReadyState
	}

	return openfeature.NotReadyState
}

// Shutdown closes the dedicated client exactly once via compare-and-swap
// (no mutex, no sync.Once, no panic recovery). It clears readiness first so
// concurrent evaluations fail closed to fallback. The client Close error
// (bounded metrics flush) propagates to the caller instead of being ignored.
func (p *unleashProvider) Shutdown() error {
	if !p.closed.CompareAndSwap(false, true) {
		return nil
	}

	p.ready.Store(false)

	return p.client.Close()
}

// Hooks returns no hooks: the provider has none.
func (p *unleashProvider) Hooks() []openfeature.Hook {
	return []openfeature.Hook{}
}

// Metadata identifies the provider to the OpenFeature SDK.
func (p *unleashProvider) Metadata() openfeature.Metadata {
	return openfeature.Metadata{
		Name: "Unleash",
	}
}

// BooleanEvaluation resolves one boolean flag from the locally cached
// toggles, falling back to the default when the provider is not ready.
func (p *unleashProvider) BooleanEvaluation(
	_ context.Context,
	flag string,
	defaultValue bool,
	evalCtx openfeature.FlattenedContext,
) openfeature.BoolResolutionDetail {
	p.ensureReady()

	if !p.isReady() {
		return errorDetail(defaultValue, openfeature.NewProviderNotReadyResolutionError(providerNotReady), openfeature.ErrorReason)
	}

	unleashCtx, err := toUnleashEvalContext(evalCtx)
	if err != nil {
		return errorDetail(defaultValue, openfeature.NewInvalidContextResolutionError(err.Error()), openfeature.ErrorReason)
	}

	if !p.flagExists(flag) {
		return errorDetail(defaultValue, openfeature.NewFlagNotFoundResolutionError(flag), openfeature.ErrorReason)
	}

	res := p.client.IsEnabled(flag, unleash.FeatureOptions{Ctx: unleashCtx, Fallback: &defaultValue})

	return valueDetail(res, "", map[string]any{"enabled": res})
}

// FloatEvaluation resolves through the object path with float coercion.
func (p *unleashProvider) FloatEvaluation(
	ctx context.Context,
	flag string,
	defaultValue float64,
	evalCtx openfeature.FlattenedContext,
) openfeature.FloatResolutionDetail {
	res := p.ObjectEvaluation(ctx, flag, defaultValue, evalCtx)

	if value, ok := res.Value.(float64); ok {
		return openfeature.FloatResolutionDetail{
			Value:                    value,
			ProviderResolutionDetail: res.ProviderResolutionDetail,
		}
	}

	if strValue, ok := res.Value.(string); ok {
		value, err := strconv.ParseFloat(strValue, 64)
		if err == nil {
			return openfeature.FloatResolutionDetail{
				Value:                    value,
				ProviderResolutionDetail: res.ProviderResolutionDetail,
			}
		}
	}

	return errorDetail(defaultValue, openfeature.NewTypeMismatchResolutionError("FloatEvaluation type error for "+flag), openfeature.ErrorReason)
}

// IntEvaluation resolves through the object path with integer coercion.
func (p *unleashProvider) IntEvaluation(
	ctx context.Context,
	flag string,
	defaultValue int64,
	evalCtx openfeature.FlattenedContext,
) openfeature.IntResolutionDetail {
	res := p.ObjectEvaluation(ctx, flag, defaultValue, evalCtx)

	if value, ok := res.Value.(int64); ok {
		return openfeature.IntResolutionDetail{
			Value:                    value,
			ProviderResolutionDetail: res.ProviderResolutionDetail,
		}
	}

	if strValue, ok := res.Value.(string); ok {
		value, err := strconv.ParseInt(strValue, 10, 64)
		if err == nil {
			return openfeature.IntResolutionDetail{
				Value:                    value,
				ProviderResolutionDetail: res.ProviderResolutionDetail,
			}
		}
	}

	return errorDetail(defaultValue, openfeature.NewTypeMismatchResolutionError("IntEvaluation type error for "+flag), openfeature.ErrorReason)
}

// StringEvaluation resolves through the object path with string coercion.
func (p *unleashProvider) StringEvaluation(
	ctx context.Context,
	flag string,
	defaultValue string,
	evalCtx openfeature.FlattenedContext,
) openfeature.StringResolutionDetail {
	res := p.ObjectEvaluation(ctx, flag, defaultValue, evalCtx)
	inner := openfeature.ProviderResolutionDetail{
		ResolutionError: res.ResolutionError,
		Reason:          res.Reason,
		Variant:         res.Variant,
		FlagMetadata:    res.FlagMetadata,
	}

	return openfeature.StringResolutionDetail{
		Value:                    fmt.Sprint(res.Value),
		ProviderResolutionDetail: inner,
	}
}

// ObjectEvaluation resolves one variant payload, falling back to the default
// when the provider is not ready or the variant is disabled.
func (p *unleashProvider) ObjectEvaluation(
	_ context.Context,
	flag string,
	defaultValue any,
	evalCtx openfeature.FlattenedContext,
) openfeature.InterfaceResolutionDetail {
	p.ensureReady()

	if !p.isReady() {
		return errorDetail(defaultValue, openfeature.NewProviderNotReadyResolutionError(providerNotReady), openfeature.ErrorReason)
	}

	unleashCtx, err := toUnleashEvalContext(evalCtx)
	if err != nil {
		return errorDetail(defaultValue, openfeature.NewGeneralResolutionError(err.Error()), openfeature.ErrorReason)
	}

	if !p.flagExists(flag) {
		return errorDetail(defaultValue, openfeature.NewFlagNotFoundResolutionError(flag), openfeature.ErrorReason)
	}

	variant := p.client.GetVariant(flag, unleash.VariantOptions{Ctx: unleashCtx})
	flagMetadata := map[string]any{
		"enabled": variant.Enabled,
	}

	if variant.Name == api.DISABLED_VARIANT.Name {
		return valueDetail(defaultValue, "", flagMetadata)
	}

	return valueDetail(any(variant.Payload.Value), variant.Name, flagMetadata)
}

// errorDetail builds a resolution detail carrying a resolution error. The
// embedded detail travels through a named intermediate: inline literals force
// a choice between two analyzer suggestions whose combination the gc rejects
// ("missing type in composite literal" for the elided cross-package embedded
// form), while a variable needs no elision.
func errorDetail[T any](
	def T,
	resolutionErr openfeature.ResolutionError,
	reason openfeature.Reason,
) openfeature.GenericResolutionDetail[T] {
	inner := openfeature.ProviderResolutionDetail{
		ResolutionError: resolutionErr,
		Reason:          reason,
	}

	return openfeature.GenericResolutionDetail[T]{
		Value:                    def,
		ProviderResolutionDetail: inner,
	}
}

// valueDetail builds a successful resolution detail with provenance metadata.
func valueDetail[T any](val T, variant string, metadata openfeature.FlagMetadata) openfeature.GenericResolutionDetail[T] {
	inner := openfeature.ProviderResolutionDetail{
		Variant:      variant,
		FlagMetadata: metadata,
	}

	return openfeature.GenericResolutionDetail[T]{
		Value:                    val,
		ProviderResolutionDetail: inner,
	}
}

// ensureReady promotes a NotReady provider whose client has since loaded
// toggles, so recovery after a startup outage needs no restart. Promotion is
// a single atomic store, so concurrent evaluations cannot race: at worst two
// callers drain the same already-signaled channel. It performs no network
// call.
func (p *unleashProvider) ensureReady() {
	if p.ready.Load() {
		return
	}

	select {
	case <-p.client.Ready():
		p.ready.Store(true)
	default:
	}
}

// isReady reports lifecycle readiness without taking the mutex.
func (p *unleashProvider) isReady() bool {
	return p.ready.Load()
}

// flagExists reports whether the flag exists in the latest polled snapshot.
// Unknown flags resolve to a FlagNotFound error (fallback, no last-known
// overwrite) instead of serving the default as a fresh provider value.
func (p *unleashProvider) flagExists(flag string) bool {
	for _, feature := range p.client.ListFeatures() {
		if feature.Name == flag {
			return true
		}
	}

	return false
}

// toUnleashEvalContext maps flattened OpenFeature attributes onto the v6
// Unleash evaluation context. Known keys fill typed fields; everything else
// becomes a custom string property. Non-string scalars convert; other values
// are an invalid-context error, never silently dropped.
func toUnleashEvalContext(evalCtx openfeature.FlattenedContext) (unleashContext.Context, error) {
	unleashCtx := unleashContext.Context{}

	if len(evalCtx) == 0 {
		return unleashCtx, nil
	}

	custom := make(map[string]string)

	for key, origVal := range evalCtx {
		val, ok := evalAttrToString(origVal)
		if !ok {
			return unleashContext.Context{}, fmt.Errorf("%w: %q", ErrInvalidEvalContext, key)
		}

		switch key {
		case openfeature.TargetingKey:
			unleashCtx.UserId = val
		case "AppName":
			unleashCtx.AppName = val
		case "CurrentTime":
			unleashCtx.CurrentTime = val
		case "Environment":
			unleashCtx.Environment = val
		case "RemoteAddress":
			unleashCtx.RemoteAddress = val
		case "SessionId":
			unleashCtx.SessionId = val
		case "UserId":
			unleashCtx.UserId = val
		default:
			custom[key] = val
		}
	}

	unleashCtx.Properties = custom

	return unleashCtx, nil
}

// evalAttrToString converts scalar evaluation attributes to strings.
func evalAttrToString(val any) (string, bool) {
	switch v := val.(type) {
	case string:
		return v, true
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v), true
	case float32, float64:
		return fmt.Sprintf("%.6f", v), true
	case bool:
		return strconv.FormatBool(v), true
	default:
		return "", false
	}
}
