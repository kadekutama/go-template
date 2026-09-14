// Package resilience owns the inner-layer resilience contracts (E01-T08,
// SPEC §7.8): circuit breaking, classified retry, and deadline propagation.
//
// Adapters (gobreaker, backoff executors) live in
// internal/infrastructure/resilience and are the ONLY packages importing
// provider libraries. A retry policy MUST classify errors and MUST refuse
// blind retries of non-idempotent money operations without a
// durable/provider idempotency key (docs/ledger-core.md §8).
package resilience

import (
	"context"
	"errors"
)

// Class is the retryability of a failed call.
type Class int

const (
	// Unknown means unclassified: retryable ONLY with an idempotency key.
	Unknown Class = iota
	// Retryable means safe to retry (with a key; bounded).
	Retryable
	// Permanent means retrying cannot help (validation, auth, conflict).
	Permanent
)

// Classifier maps a call error to its Class. Nil errors are never classified
// (success short-circuits before classification).
type Classifier func(err error) Class

// RetryableError marks err retryable.
type RetryableError struct{ Err error }

func (e *RetryableError) Error() string { return e.Err.Error() }
func (e *RetryableError) Unwrap() error { return e.Err }

// PermanentError marks err non-retryable.
type PermanentError struct{ Err error }

func (e *PermanentError) Error() string { return e.Err.Error() }
func (e *PermanentError) Unwrap() error { return e.Err }

// DefaultClassifier maps the wrapper types; anything else is Unknown.
func DefaultClassifier(err error) Class {
	var retryable *RetryableError
	if errors.As(err, &retryable) {
		return Retryable
	}
	var permanent *PermanentError
	if errors.As(err, &permanent) {
		return Permanent
	}
	return Unknown
}

// Breaker guards one dependency (payment processor, FX, SMTP, OAuth, flags…).
type Breaker interface {
	// Execute runs fn unless the breaker is open. operation names the
	// dependency call; idempotencyKey carries the durable/provider key (empty
	// when the operation has none — see the no-blind-retry rule).
	Execute(ctx context.Context, operation string, idempotencyKey string, fn func(ctx context.Context) error) error
	// State reports closed/open/half-open for health and tests.
	State() State
}

// State is the breaker position.
type State int

const (
	// StateClosed passes calls and counts failures.
	StateClosed State = iota
	// StateOpen fast-fails calls until the probe timeout elapses.
	StateOpen
	// StateHalfOpen admits one probe call; success closes, failure re-opens.
	StateHalfOpen
)
