// Package featureflag_test proves the OpenFeature + Unleash flag boundary
// end to end with zero vendor credentials (E10-T01 G4 slice). A real
// Unleash server (plus Postgres) runs in local Testcontainers; the production
// UnleashEvaluator polls it through the egress-gated transport, so flag flips
// land without a restart and unknown flags resolve to fallback.
//
// One evaluator per test binary (see TestMain): OpenFeature binds providers
// process-wide, so sharing one evaluator across -count runs keeps the suite
// race-clean. Integration parents stay sequential per the repository lint
// exclusion; unit tests carry parallel coverage.
package featureflag_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/infrastructure/egress"
	"github.com/kadekutama/go-template/internal/infrastructure/featureflag"
	infraresilience "github.com/kadekutama/go-template/internal/infrastructure/resilience"
	"github.com/kadekutama/go-template/internal/shared/kernel"
	kernelresilience "github.com/kadekutama/go-template/internal/shared/kernel/resilience"
	"github.com/kadekutama/go-template/internal/shared/kernel/sleep"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
)

// flagName is the single integration flag, created on the real server.
const flagName = "payMethod.card.enabled"

// sharedHarness is the real Unleash + Postgres pair for the whole binary.
var sharedHarness *unleashHarness

// sharedEvaluator is the single production evaluator for the whole binary.
var sharedEvaluator featureflag.Evaluator

// setupFatal reports a setup failure and exits: TestMain has no *testing.T.
func setupFatal(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "featureflag integration setup: %v\n", err)

	os.Exit(1)
}

// TestMain boots one real Unleash server and one evaluator shared across
// -count runs, so no second SetProvider races the first shutdown in the
// background.
func TestMain(m *testing.M) {
	if !dockerReachable() {
		_, _ = fmt.Fprintln(os.Stderr, "featureflag integration setup: skipping without Docker")

		os.Exit(0)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

	harness, err := startUnleashHarness(ctx)
	if err != nil {
		setupFatal(err)
	}

	sharedHarness = harness

	limiterMock := &mockapplication.MockRateLimiter{}
	limiterMock.On("Allow", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		appport.RateLimitDecision{Allowed: true, Remaining: 999},
		nil,
	)

	limiter, err := egress.NewEgressLimiter(egress.Params{
		Limiter: limiterMock,
		Policies: []egress.Policy{
			{Provider: featureflag.EgressProvider, Operation: featureflag.EgressOperation, Budget: 1000, Window: time.Minute, MaxWait: 5 * time.Second},
		},
		Clock:   kernel.SystemClock{},
		Jitter:  egress.NewCappedJitter(),
		Sleeper: sleep.TimerSleeper{},
	})
	if err != nil {
		setupFatal(err)
	}

	gated, err := featureflag.NewGatedTransport(limiter, testBreaker(), http.DefaultTransport, nil)
	if err != nil {
		setupFatal(err)
	}

	evaluator, err := featureflag.NewUnleashEvaluator(featureflag.UnleashEvaluatorParams{
		UnleashURL:      harness.baseURL + "/api",
		AppName:         "ledger-integration",
		RefreshInterval: 200 * time.Millisecond,
		ClientToken:     harness.clientToken,
		InitTimeout:     30 * time.Second,
		HTTPClient:      &http.Client{Transport: gated},
	})
	if err != nil {
		setupFatal(err)
	}

	sharedEvaluator = evaluator

	if err := harness.createFlag(ctx, flagName); err != nil {
		setupFatal(err)
	}

	code := m.Run()

	cancel()

	if err := evaluator.Close(context.Background()); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "featureflag integration close: %v\n", err)
	}

	harness.terminate(context.Background())

	os.Exit(code)
}

// testBreaker builds the production breaker for integration composition.
func testBreaker() *infraresilience.CircuitBreaker {
	return infraresilience.New("featureflag-integration", infraresilience.Settings{
		FailureThreshold: 5,
		ProbeTimeout:     5 * time.Second,
	})
}

// testEgress builds the production egress limiter over the generated
// rate-limiter mock. It proves permit-per-attempt composition without a real
// Valkey; the Valkey-backed burst proof lives in E09.1-T03/T05.
func testEgress(t *testing.T) *egress.Limiter {
	t.Helper()

	limiter := &mockapplication.MockRateLimiter{}
	limiter.On("Allow", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		appport.RateLimitDecision{Allowed: true, Remaining: 999},
		nil,
	)

	waiter, err := egress.NewEgressLimiter(egress.Params{
		Limiter: limiter,
		Policies: []egress.Policy{
			{Provider: featureflag.EgressProvider, Operation: featureflag.EgressOperation, Budget: 1000, Window: time.Minute, MaxWait: 5 * time.Second},
		},
		Clock:   kernel.SystemClock{},
		Jitter:  egress.NewCappedJitter(),
		Sleeper: sleep.TimerSleeper{},
	})
	require.NoError(t, err)

	return waiter
}

// TestUnleashEval proves flag evaluation through the real OpenFeature +
// Unleash stack: flip without restart and health. It reuses the shared
// evaluator and resets the flag each run so -count repeats stay green.
func TestUnleashEval(t *testing.T) {
	t.Run("flag flip lands without restart", func(t *testing.T) {
		require.NotNil(t, sharedEvaluator)
		require.NotNil(t, sharedHarness)

		require.NoError(t, sharedEvaluator.Health(context.Background()))

		evalCtx := appport.FlagContext{TenantID: "tenant-1"}

		// Reset to disabled so -count repeats start from the same state.
		require.NoError(t, sharedHarness.setFlagEnabled(context.Background(), flagName, false))

		// Initially disabled: the evaluator serves the server value (false),
		// not an error, on the next poll cycle.
		require.Eventually(t, func() bool {
			value, err := sharedEvaluator.Evaluate(context.Background(), flagName, false, evalCtx)

			return err == nil && value == false
		}, 10*time.Second, 200*time.Millisecond)

		// Flip without restarting the evaluator.
		require.NoError(t, sharedHarness.setFlagEnabled(context.Background(), flagName, true))

		require.Eventually(t, func() bool {
			value, err := sharedEvaluator.Evaluate(context.Background(), flagName, false, evalCtx)

			return err == nil && value == true
		}, 10*time.Second, 200*time.Millisecond)
	})
}

// TestUnleashUnknownFlag proves an unregistered flag resolves to fallback
// with a typed error through the full Client (CR-003): the default is served
// with Fallback true, never as a fresh provider value. It wraps the shared
// evaluator in a Client, so no second provider registers and the process-wide
// OpenFeature singleton stays untouched across -count repeats.
func TestUnleashUnknownFlag(t *testing.T) {
	require.NotNil(t, sharedEvaluator)
	require.NotNil(t, sharedHarness)

	client, err := featureflag.NewClient(featureflag.Params{
		UnleashURL:        sharedHarness.baseURL + "/api",
		AppName:           "ledger-integration-unknown",
		RefreshInterval:   10 * time.Second,
		CacheTTL:          time.Hour,
		InitTimeout:       5 * time.Second,
		ProbeMaxBodyBytes: 1024,
		Transport:         http.DefaultTransport,
		RetryPolicy:       testRetryPolicy(),
		Egress:            testEgress(t),
		Breaker:           testBreaker(),
		HTTP:              &http.Client{},
		Clock:             kernel.SystemClock{},
		Evaluator:         sharedEvaluator,
	})
	require.NoError(t, err)

	actualResult, err := client.BooleanWithDetails(
		context.Background(),
		"flag.does.not.exist",
		true,
		appport.FlagContext{TenantID: "tenant-1"},
	)
	assert.Equal(t, appport.FlagDetails{Value: true, Fallback: true}, actualResult)
	require.Error(t, err)

	var flagErr *entity.Error
	require.ErrorAs(t, err, &flagErr)
	assert.Equal(t, "FEATUREFLAG_PROVIDER_UNAVAILABLE", flagErr.Code)
}

// testRetryPolicy bounds integration retries without wall-clock waits.
func testRetryPolicy() kernelresilience.RetryPolicy {
	return kernelresilience.RetryPolicy{
		MaxAttempts:    3,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     5 * time.Millisecond,
		Multiplier:     2.0,
	}
}
