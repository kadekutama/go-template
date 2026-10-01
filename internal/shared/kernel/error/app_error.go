// Package apperror is the shared error envelope (E01-T06).
//
// Every error carries a stable UPPER_SNAKE code from the registry below.
// Construct errors with New (which rejects unregistered codes) and render
// them with Translator for locale-aware messages.
package apperror

import (
	"fmt"
	"maps"
	"net/http"
	"regexp"
	"sync/atomic"
)

// Code is a stable UPPER_SNAKE error identifier, e.g. VALIDATION_FAILED.
type Code string

// AppError is the standard envelope carried from domain to transport.
type AppError struct {
	Code       Code
	Message    string
	Details    map[string]any
	Cause      error
	HTTPStatus int
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Cause }

var codePattern = regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)*$`)

var registry atomic.Pointer[map[Code]int]

func init() {
	initial := defaultCodes()
	registry.Store(&initial)
}

// Register adds codes with their default HTTP status. It panics on blank,
// malformed, or duplicate codes: registration happens at package load of the
// owning layer, where a panic fails fast before serving traffic.
func Register(codes map[Code]int) {
	for code := range codes {
		if code == "" || !codePattern.MatchString(string(code)) {
			panic(fmt.Sprintf("apperror: malformed code %q (want UPPER_SNAKE)", code))
		}
	}

	for {
		oldPtr := registry.Load()
		var oldMap map[Code]int
		if oldPtr != nil {
			oldMap = *oldPtr
		}
		for code := range codes {
			if _, dup := oldMap[code]; dup {
				panic(fmt.Sprintf("apperror: duplicate code %q", code))
			}
		}

		newMap := make(map[Code]int, len(oldMap)+len(codes))
		maps.Copy(newMap, oldMap)
		maps.Copy(newMap, codes)

		if registry.CompareAndSwap(oldPtr, &newMap) {
			break
		}
	}
}

// Option tunes New.
type Option func(*AppError)

// WithDetails attaches structured context.
func WithDetails(details map[string]any) Option {
	return func(e *AppError) { e.Details = details }
}

// WithCause wraps a lower-level error.
func WithCause(cause error) Option {
	return func(e *AppError) { e.Cause = cause }
}

// WithHTTPStatus overrides the registry default for this instance.
func WithHTTPStatus(status int) Option {
	return func(e *AppError) { e.HTTPStatus = status }
}

// New builds an *AppError. Unknown codes fail: every code must be registered
// by its owning layer (see codes.go and later epics).
func New(code Code, message string, opts ...Option) (*AppError, error) {
	ptr := registry.Load()
	if ptr == nil {
		return nil, fmt.Errorf("apperror: unregistered code %q", code)
	}

	status, ok := (*ptr)[code]
	if !ok {
		return nil, fmt.Errorf("apperror: unregistered code %q", code)
	}

	err := &AppError{Code: code, Message: message, HTTPStatus: status}
	for _, opt := range opts {
		opt(err)
	}

	return err, nil
}

// MustNew is New for call sites that hold a code constant proven registered by
// tests (e.g. package-level error helpers). It panics on unknown codes.
func MustNew(code Code, message string, opts ...Option) *AppError {
	err, newErr := New(code, message, opts...)
	if newErr != nil {
		panic(newErr)
	}

	return err
}

// HTTPStatusFor reports the registry default for code, or 500 when unknown.
func HTTPStatusFor(code Code) int {
	ptr := registry.Load()
	if ptr != nil {
		if status, ok := (*ptr)[code]; ok {
			return status
		}
	}

	return http.StatusInternalServerError
}
