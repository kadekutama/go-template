// Package resilience adapts sony/gobreaker behind the kernel Breaker port
// (E01-T08). Provider code imports the kernel port only; swapping gobreaker
// means replacing this package plus one fx line.
package resilience

import (
	"context"
	"errors"
	"fmt"
	"time"

	kernel "example.com/go-template/internal/shared/kernel/resilience"

	"github.com/sony/gobreaker"
)

// ErrBreakerOpen marks fast-failed calls. Use errors.Is to detect them.
var ErrBreakerOpen = errors.New("resilience: circuit breaker open")

// Settings tunes one breaker instance.
type Settings struct {
	// FailureThreshold trips the breaker after this many consecutive failures.
	FailureThreshold uint32
	// ProbeTimeout is how long an open breaker waits before a half-open probe.
	ProbeTimeout time.Duration
	// OnStateChange observes transitions (metrics in E15); nil disables.
	OnStateChange func(name string, from, to kernel.State)
}

// CircuitBreaker is the gobreaker-backed kernel Breaker.
type CircuitBreaker struct {
	name    string
	inner   *gobreaker.CircuitBreaker
	hooks   func(name string, from, to kernel.State)
	timeout time.Duration
}

// New builds a breaker: MaxRequests=1 admits a single half-open probe.
func New(name string, settings Settings) *CircuitBreaker {
	if settings.FailureThreshold == 0 {
		settings.FailureThreshold = 5
	}
	if settings.ProbeTimeout <= 0 {
		settings.ProbeTimeout = 30 * time.Second
	}
	breaker := &CircuitBreaker{name: name, hooks: settings.OnStateChange, timeout: settings.ProbeTimeout}
	threshold := settings.FailureThreshold
	breaker.inner = gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        name,
		MaxRequests: 1,
		Timeout:     settings.ProbeTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= threshold
		},
		OnStateChange: func(_ string, from gobreaker.State, to gobreaker.State) {
			if breaker.hooks != nil {
				breaker.hooks(name, mapState(from), mapState(to))
			}
		},
	})
	return breaker
}

func mapState(state gobreaker.State) kernel.State {
	switch state {
	case gobreaker.StateOpen:
		return kernel.StateOpen
	case gobreaker.StateHalfOpen:
		return kernel.StateHalfOpen
	default:
		return kernel.StateClosed
	}
}

// Execute runs fn unless the breaker is open. The idempotencyKey exists for
// call-site uniformity with the retry layer (SPEC §7.8); breaking decisions
// use failure counts and timeouts only. Context cancellation is checked before
// calling fn and never retried here (the retry executor owns that rule).
func (b *CircuitBreaker) Execute(ctx context.Context, _ string, _ string, fn func(ctx context.Context) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := b.inner.Execute(func() (any, error) {
		return nil, fn(ctx)
	})
	if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
		return fmt.Errorf("%w (breaker %q)", ErrBreakerOpen, b.name)
	}
	return err
}

// State reports the current breaker position.
func (b *CircuitBreaker) State() kernel.State {
	return mapState(b.inner.State())
}
