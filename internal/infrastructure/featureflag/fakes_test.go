package featureflag_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/stretchr/testify/mock"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	infraresilience "github.com/kadekutama/go-template/internal/infrastructure/resilience"
	kernelresilience "github.com/kadekutama/go-template/internal/shared/kernel/resilience"
	"github.com/kadekutama/go-template/internal/shared/kernel/validate"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
	mockfeatureflag "github.com/kadekutama/go-template/test/mock/featureflag"
	mockhttpclient "github.com/kadekutama/go-template/test/mock/httpclient"
	mockhttptransport "github.com/kadekutama/go-template/test/mock/httptransport"
	mockresilience "github.com/kadekutama/go-template/test/mock/resilience"
)

// errorCodeOf extracts the comparable failure code from any task error: the
// domain code for coded failures, VALIDATION for constructor tag violations,
// BREAKER_OPEN for fast-fails, CANCELED for cancellation, or the message
// otherwise. Tests assert this single string per error return, so every table
// case executes exactly one assertion per returned value without hardcoding
// validator-library message formats.
func errorCodeOf(err error) string {
	if err == nil {
		return ""
	}

	flagErr, ok := errors.AsType[*entity.Error](err)
	if ok {
		return flagErr.Code
	}

	if errors.Is(err, validate.ErrValidateViolation) {
		return "VALIDATION"
	}

	if errors.Is(err, infraresilience.ErrBreakerOpen) {
		return "BREAKER_OPEN"
	}

	if errors.Is(err, context.Canceled) {
		return "CANCELED"
	}

	return err.Error()
}

// callCount counts recorded invocations of one method on a generated mock.
// One generic helper replaces five per-method counters; method-specific
// key/TTL inspectors below stay separate.
func callCount(m *mock.Mock, method string) int {
	count := 0

	for _, call := range m.Calls {
		if call.Method == method {
			count++
		}
	}

	return count
}

// newMockEgress scripts the generated egress mock to admit or fail every
// attempt with one outcome. Call counts come from recorded Calls.
func newMockEgress(err error) *mockfeatureflag.MockEgressWaiter {
	egress := &mockfeatureflag.MockEgressWaiter{}
	egress.On("Wait", mock.Anything, mock.Anything, mock.Anything).Return(err)

	return egress
}

// waitKeys returns recorded provider/operation keys as "provider/operation".
func waitKeys(egress *mockfeatureflag.MockEgressWaiter) []string {
	keys := []string{}

	for _, call := range egress.Calls {
		if call.Method == "Wait" {
			provider, _ := call.Arguments.Get(1).(string)
			operation, _ := call.Arguments.Get(2).(string)
			keys = append(keys, provider+"/"+operation)
		}
	}

	return keys
}

// newMockBreaker scripts the generated breaker mock: a non-nil openErr
// fast-fails without invoking fn, otherwise fn runs directly (pass-through).
// State reports the scripted mode for the gated-transport fast-fail path.
func newMockBreaker(openErr error) *mockresilience.MockBreaker {
	breaker := &mockresilience.MockBreaker{}

	if openErr != nil {
		breaker.On("Execute", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(openErr)
	} else {
		breaker.On("Execute", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
			func(ctx context.Context, _ string, _ string, fn func(ctx context.Context) error) error {
				return fn(ctx)
			},
		)
	}

	if openErr != nil {
		breaker.On("State").Return(kernelresilience.StateOpen)
	} else {
		breaker.On("State").Return(kernelresilience.StateClosed)
	}

	return breaker
}

// mockEvalStep is one scripted evaluation outcome.
type mockEvalStep struct {
	value bool
	err   error
}

// newMockEvaluator scripts the generated evaluator mock: scripted steps play
// in order, then the sticky value/error repeats. Health and Close report the
// scripted health outcome and nil respectively.
func newMockEvaluator(value bool, err error, script []mockEvalStep, healthErr error) *mockfeatureflag.MockEvaluator {
	evaluator := &mockfeatureflag.MockEvaluator{}

	for _, step := range script {
		evaluator.On("Evaluate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(step.value, step.err).Once()
	}

	evaluator.On("Evaluate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		func(_ context.Context, _ string, def bool, _ appport.FlagContext) (bool, error) {
			if err != nil {
				return def, err
			}

			return value, nil
		},
		nil,
	)
	evaluator.On("Health", mock.Anything).Return(healthErr)
	evaluator.On("Close", mock.Anything).Return(nil)

	return evaluator
}

// newMockRoundTripper scripts the generated transport mock with one outcome
// per call. Call counts come from recorded Calls.
func newMockRoundTripper(status int, roundErr error) *mockhttptransport.MockRoundTripper {
	base := &mockhttptransport.MockRoundTripper{}
	base.On("RoundTrip", mock.Anything).Return(
		func(_ *http.Request) (*http.Response, error) {
			if roundErr != nil {
				return nil, roundErr
			}

			return statusResponse(status), nil
		},
		nil,
	)

	return base
}

// statusResponse builds a minimal response with one status code.
func statusResponse(code int) *http.Response {
	return &http.Response{
		StatusCode: code,
		Body:       io.NopCloser(strings.NewReader("probe")),
	}
}

// newMockDoer scripts the generated HTTP Doer mock to answer every probe
// with a fresh response of one status (or the scripted transport error).
// Call counts come from the mock's recorded Calls; no hand-rolled double.
func newMockDoer(status int, probeErr error) *mockhttpclient.MockDoer {
	doer := &mockhttpclient.MockDoer{}
	doer.On("Do", mock.Anything).Return(
		func(_ *http.Request) (*http.Response, error) {
			if probeErr != nil {
				return nil, probeErr
			}

			return statusResponse(status), nil
		},
		nil,
	)

	return doer
}

// newMockCache scripts the generated Cache mock over one in-memory map:
// Get serves per-key values, Set records with the scripted error. TTLs are
// asserted by inspecting recorded Set calls; no hand-rolled double.
func newMockCache(values map[string][]byte, setErr error) *mockapplication.MockCache {
	cache := &mockapplication.MockCache{}
	cache.On("Get", mock.Anything, mock.Anything).Return(
		func(_ context.Context, key string) ([]byte, error) {
			value, ok := values[key]
			if !ok {
				return nil, errCacheMiss
			}

			return value, nil
		},
		nil,
	)
	cache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(setErr)

	return cache
}

// cacheSetTTL returns the TTL recorded for one Set call.
func cacheSetTTL(cache *mockapplication.MockCache, key string) (time.Duration, bool) {
	for _, call := range cache.Calls {
		if call.Method == "Set" {
			if callKey, _ := call.Arguments.Get(1).(string); callKey == key {
				ttl, _ := call.Arguments.Get(3).(time.Duration)

				return ttl, true
			}
		}
	}

	return 0, false
}

// providerUnavailable builds the typed provider-outage failure for scripts.
func providerUnavailable() error {
	return entity.NewError("FEATUREFLAG_PROVIDER_UNAVAILABLE", "featureflag: unleash provider unavailable")
}

// breakerOpen builds the shared breaker fast-fail for scripts.
func breakerOpen() error {
	return infraresilience.ErrBreakerOpen
}

// testClock returns a mockery clock replaying the given instants in order
// (last instant repeats), so staleness tests advance time deterministically.
func testClock(now ...time.Time) *mockapplication.MockClock {
	clock := &mockapplication.MockClock{}

	for index, instant := range now {
		if index == len(now)-1 {
			clock.On("Now").Return(instant)
		} else {
			clock.On("Now").Return(instant).Once()
		}
	}

	return clock
}
