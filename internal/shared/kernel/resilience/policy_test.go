package resilience

import (
	"errors"
	"testing"
	"time"
)

func TestDefaultClassifier(t *testing.T) {
	t.Parallel()

	if got := DefaultClassifier(&RetryableError{Err: errors.New("x")}); got != Retryable {
		t.Errorf("want Retryable, got %v", got)
	}
	if got := DefaultClassifier(&PermanentError{Err: errors.New("x")}); got != Permanent {
		t.Errorf("want Permanent, got %v", got)
	}
	if got := DefaultClassifier(errors.New("x")); got != Unknown {
		t.Errorf("want Unknown, got %v", got)
	}
}

func TestShouldRetryMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		class      Class
		idempotent bool
		key        string
		want       bool
	}{
		{"permanent never retries even with key", Permanent, true, "k", false},
		{"retryable with key retries", Retryable, false, "k", true},
		{"retryable idempotent retries", Retryable, true, "", true},
		{"retryable without key or flag never retries", Retryable, false, "", false},
		{"unknown with key retries", Unknown, false, "k", true},
		{"unknown without key never retries", Unknown, false, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ShouldRetry(tc.class, tc.idempotent, tc.key); got != tc.want {
				t.Errorf("ShouldRetry = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBackoffBounds(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{MaxAttempts: 5, InitialBackoff: 100 * time.Millisecond, MaxBackoff: 250 * time.Millisecond, Multiplier: 10}.Normalize()
	if got := policy.BackoffFor(0); got != 100*time.Millisecond {
		t.Errorf("zero retry backoff = %v, want 100ms", got)
	}
	if got := policy.BackoffFor(-1); got != 100*time.Millisecond {
		t.Errorf("negative retry backoff = %v, want 100ms", got)
	}
	if got := policy.BackoffFor(1); got != 100*time.Millisecond {
		t.Errorf("first backoff = %v", got)
	}
	if got := policy.BackoffFor(4); got != 250*time.Millisecond {
		t.Errorf("backoff must cap at max, got %v", got)
	}
	zero := RetryPolicy{}.Normalize()
	if zero.MaxAttempts != DefaultMaxAttempts || zero.Multiplier != DefaultMultiplier {
		t.Errorf("zero policy not defaulted: %+v", zero)
	}
}

func TestErrorWrappers(t *testing.T) {
	t.Parallel()

	inner := errors.New("underlying issue")

	retryable := &RetryableError{Err: inner}
	if retryable.Error() != "underlying issue" {
		t.Errorf("got %q, want %q", retryable.Error(), "underlying issue")
	}
	if !errors.Is(retryable, inner) {
		t.Errorf("errors.Is failed for retryable unwrap")
	}
	if retryable.Unwrap() != inner {
		t.Errorf("Unwrap() got %v, want %v", retryable.Unwrap(), inner)
	}

	permanent := &PermanentError{Err: inner}
	if permanent.Error() != "underlying issue" {
		t.Errorf("got %q, want %q", permanent.Error(), "underlying issue")
	}
	if !errors.Is(permanent, inner) {
		t.Errorf("errors.Is failed for permanent unwrap")
	}
	if permanent.Unwrap() != inner {
		t.Errorf("Unwrap() got %v, want %v", permanent.Unwrap(), inner)
	}
}
