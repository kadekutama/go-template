// Package specification provides stateless, composable executable business
// invariants. Specifications hold no evaluation state and are safe for
// concurrent reuse.
package specification

import (
	"context"
	"maps"
)

const (
	codeNilChild = "SPEC_NIL_CHILD"
	keyAsset     = "asset"
	keyDetail    = "detail"
)

// Violation is one failed business rule with a stable code.
type Violation struct {
	// Code is a stable UPPER_SNAKE token supplied by the spec author.
	Code string
	// Message is a human-readable description of the failure.
	Message string
	// Details carries structured failure context.
	Details map[string]string
}

// SpecResult is the outcome of one specification evaluation.
type SpecResult struct {
	// Violations holds every failed rule in evaluation order. It is nil or
	// empty when the candidate passes.
	Violations []Violation
}

// Passed reports whether the candidate satisfied the specification.
func (r SpecResult) Passed() bool { return len(r.Violations) == 0 }

// Specification is a stateless executable business rule. Implementations must
// not store candidate-specific errors and must be safe for concurrent reuse.
// Evaluate must return a Violation instead of panicking on nil or zero-value
// candidates.
type Specification[T any] interface {
	// Evaluate checks the candidate and returns every applicable violation.
	Evaluate(ctx context.Context, candidate T) SpecResult
}

// funcSpec adapts a predicate function to a Specification.
type funcSpec[T any] struct {
	violation Violation
	fn        func(ctx context.Context, candidate T) bool
}

// NewFuncSpec builds a Specification from a predicate. When fn reports false,
// evaluation fails with a copy of the supplied violation. A nil fn fails every
// evaluation with a SPEC_NIL_FUNC violation instead of panicking.
func NewFuncSpec[T any](code, message string, fn func(ctx context.Context, candidate T) bool) Specification[T] {
	return funcSpec[T]{
		violation: copyViolation(Violation{Code: code, Message: message}),
		fn:        fn,
	}
}

// Evaluate runs the predicate and returns pass or its violation.
func (s funcSpec[T]) Evaluate(ctx context.Context, candidate T) SpecResult {
	if s.fn == nil {
		return SpecResult{Violations: []Violation{{
			Code:    "SPEC_NIL_FUNC",
			Message: "specification: nil predicate function",
		}}}
	}
	if s.fn(ctx, candidate) {
		return SpecResult{}
	}
	return SpecResult{Violations: []Violation{copyViolation(s.violation)}}
}

// allSpec passes only when every child passes.
type allSpec[T any] struct {
	specs []Specification[T]
}

// All returns a Specification that evaluates every child and aggregates every
// child violation in child order. All with zero children passes.
func All[T any](specs ...Specification[T]) Specification[T] {
	return allSpec[T]{specs: specs}
}

// Evaluate runs every child and aggregates all violations.
func (s allSpec[T]) Evaluate(ctx context.Context, candidate T) SpecResult {
	var out []Violation
	for _, child := range s.specs {
		if child == nil {
			out = append(out, Violation{
				Code:    codeNilChild,
				Message: "specification: nil child in All",
			})
			continue
		}
		res := child.Evaluate(ctx, candidate)
		out = append(out, copyViolations(res.Violations)...)
	}
	if len(out) == 0 {
		return SpecResult{}
	}
	return SpecResult{Violations: out}
}

// anySpec passes when at least one child passes.
type anySpec[T any] struct {
	specs []Specification[T]
}

// Any returns a Specification that short-circuits on the first passing child.
// When every child fails, all violations are aggregated in child order. Any
// with zero children fails with an explicit violation.
func Any[T any](specs ...Specification[T]) Specification[T] {
	return anySpec[T]{specs: specs}
}

// Evaluate returns pass on the first passing child, else all violations.
func (s anySpec[T]) Evaluate(ctx context.Context, candidate T) SpecResult {
	if len(s.specs) == 0 {
		return SpecResult{Violations: []Violation{{
			Code:    "SPEC_EMPTY_ANY",
			Message: "specification: Any requires at least one child",
		}}}
	}
	var out []Violation
	for _, child := range s.specs {
		if child == nil {
			out = append(out, Violation{
				Code:    codeNilChild,
				Message: "specification: nil child in Any",
			})
			continue
		}
		res := child.Evaluate(ctx, candidate)
		if res.Passed() {
			return SpecResult{}
		}
		out = append(out, copyViolations(res.Violations)...)
	}
	return SpecResult{Violations: out}
}

// notSpec inverts one child with an explicit violation.
type notSpec[T any] struct {
	spec      Specification[T]
	violation Violation
}

// Not returns a Specification that passes only when its child fails. When the
// child passes, evaluation fails carrying a copy of the explicit violation.
func Not[T any](spec Specification[T], violation Violation) Specification[T] {
	return notSpec[T]{spec: spec, violation: copyViolation(violation)}
}

// Evaluate inverts the child outcome.
func (s notSpec[T]) Evaluate(ctx context.Context, candidate T) SpecResult {
	if s.spec == nil {
		return SpecResult{Violations: []Violation{{
			Code:    codeNilChild,
			Message: "specification: nil child in Not",
		}}}
	}
	if s.spec.Evaluate(ctx, candidate).Passed() {
		return SpecResult{Violations: []Violation{copyViolation(s.violation)}}
	}
	return SpecResult{}
}

// evalFunc adapts a SpecResult-returning function to a Specification for
// rules that need structured details beyond a boolean predicate.
type evalFunc[T any] func(ctx context.Context, candidate T) SpecResult

// Evaluate runs the function.
func (f evalFunc[T]) Evaluate(ctx context.Context, candidate T) SpecResult {
	if f == nil {
		return SpecResult{Violations: []Violation{{
			Code:    "SPEC_NIL_FUNC",
			Message: "specification: nil evaluator function",
		}}}
	}
	return f(ctx, candidate)
}

func copyViolation(in Violation) Violation {
	out := in
	out.Details = maps.Clone(in.Details)
	return out
}

func copyViolations(in []Violation) []Violation {
	if len(in) == 0 {
		return nil
	}
	out := make([]Violation, 0, len(in))
	for _, v := range in {
		out = append(out, copyViolation(v))
	}
	return out
}
