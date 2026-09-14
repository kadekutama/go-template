package apperror

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
)

var testCodeCounter atomic.Int64

func TestNewRejectsUnregisteredCode(t *testing.T) {
	t.Parallel()

	if _, err := New("NOPE_X", "nope"); err == nil {
		t.Fatal("expected registry error, got nil")
	}
}

func TestNewBuildsRegisteredEnvelope(t *testing.T) {
	t.Parallel()

	err, newErr := New(CodeValidationFailed, "bad input", WithDetails(map[string]any{"field": "email"}))
	if newErr != nil {
		t.Fatalf("New: %v", newErr)
	}
	if err.Code != CodeValidationFailed || err.HTTPStatus != http.StatusBadRequest {
		t.Errorf("unexpected envelope: %+v", err)
	}
	if err.Details["field"] != "email" {
		t.Errorf("details lost: %+v", err.Details)
	}
}

func TestRegisterRejectsMalformedAndDuplicate(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Error("expected panic for malformed code")
		}
	}()
	Register(map[Code]int{"lower_snake": http.StatusBadRequest})
}

func TestRegisterRejectsDuplicate(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Error("expected panic for duplicate code")
		}
	}()
	Register(map[Code]int{CodeNotFound: http.StatusNotFound})
}

func TestTranslateEnAndId(t *testing.T) {
	t.Parallel()

	translator, err := NewTranslator()
	if err != nil {
		t.Fatalf("NewTranslator: %v", err)
	}
	appErr := MustNew(CodeValidationFailed, "fallback")

	en := translator.Translate(WithLocale(context.Background(), "en"), appErr)
	id := translator.Translate(WithLocale(context.Background(), "id"), appErr)

	if en != "request failed validation" {
		t.Errorf("unexpected EN message: %q", en)
	}
	if id != "permintaan gagal validasi" {
		t.Errorf("unexpected ID message: %q", id)
	}
}

func TestTranslateFallsBackToMessage(t *testing.T) {
	t.Parallel()

	translator, err := NewTranslator()
	if err != nil {
		t.Fatalf("NewTranslator: %v", err)
	}
	// Unknown locale tag falls back to English; unregistered codes cannot be
	// constructed, so fallback covers missing-catalog cases only.
	appErr := MustNew(CodeNotFound, "gone")
	if got := translator.Translate(context.Background(), appErr); got != "resource not found" {
		t.Errorf("default locale should be EN, got %q", got)
	}
}

func TestAppErrorFormattingAndUnwrap(t *testing.T) {
	t.Parallel()

	simple := MustNew(CodeConflict, "conflict occurred")
	if simple.Error() != "CONFLICT: conflict occurred" {
		t.Errorf("Error() = %q, want CONFLICT: conflict occurred", simple.Error())
	}
	if simple.Unwrap() != nil {
		t.Errorf("simple error unwrap want nil, got %v", simple.Unwrap())
	}

	cause := http.ErrHandlerTimeout
	wrapped := MustNew(CodeInternalError, "timed out", WithCause(cause), WithHTTPStatus(http.StatusGatewayTimeout))
	if wrapped.Error() != "INTERNAL_ERROR: timed out: http: Handler timeout" {
		t.Errorf("wrapped Error() = %q", wrapped.Error())
	}
	if wrapped.Unwrap() != cause {
		t.Errorf("Unwrap() = %v, want %v", wrapped.Unwrap(), cause)
	}
	if wrapped.HTTPStatus != http.StatusGatewayTimeout {
		t.Errorf("HTTPStatus = %d, want %d", wrapped.HTTPStatus, http.StatusGatewayTimeout)
	}
}

func TestHTTPStatusFor(t *testing.T) {
	t.Parallel()

	if got := HTTPStatusFor(CodeNotFound); got != http.StatusNotFound {
		t.Errorf("HTTPStatusFor(NOT_FOUND) = %d, want %d", got, http.StatusNotFound)
	}
	if got := HTTPStatusFor("UNKNOWN_NONEXISTENT_CODE"); got != http.StatusInternalServerError {
		t.Errorf("HTTPStatusFor(unknown) = %d, want %d", got, http.StatusInternalServerError)
	}
}

func TestMustNewPanicsOnUnregistered(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Error("MustNew with unregistered code must panic")
		}
	}()
	_ = MustNew("UNKNOWN_CODE", "message")
}

func TestRegisterValidCustomCode(t *testing.T) {
	id := testCodeCounter.Add(1)
	customCode := Code(fmt.Sprintf("CUSTOM_CODE_%d", id))
	Register(map[Code]int{customCode: http.StatusGatewayTimeout})

	err, newErr := New(customCode, "timed out at provider")
	if newErr != nil {
		t.Fatalf("expected registered code to succeed, got: %v", newErr)
	}
	if err.Code != customCode || err.HTTPStatus != http.StatusGatewayTimeout {
		t.Errorf("unexpected custom envelope: %+v", err)
	}
	if got := HTTPStatusFor(customCode); got != http.StatusGatewayTimeout {
		t.Errorf("HTTPStatusFor(%s) = %d, want %d", customCode, got, http.StatusGatewayTimeout)
	}
}

func TestRegisterRejectsMalformedVariations(t *testing.T) {
	t.Parallel()

	malformed := []Code{
		"",
		"123_STARTS_WITH_NUMBER",
		"lowercase_code",
		"TRAILING_UNDERSCORE_",
		"DASH-NOT-ALLOWED",
		"_LEADING_UNDERSCORE",
	}

	for _, bad := range malformed {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("expected panic for malformed code %q", bad)
				}
			}()
			Register(map[Code]int{bad: http.StatusBadRequest})
		}()
	}
}

func TestTranslateNilSafety(t *testing.T) {
	t.Parallel()

	translator, err := NewTranslator()
	if err != nil {
		t.Fatalf("NewTranslator: %v", err)
	}

	// Nil error must return empty string without panicking
	if got := translator.Translate(context.Background(), nil); got != "" {
		t.Errorf("Translate(ctx, nil) = %q, want empty string", got)
	}

	// Nil translator must fall back to err.Message without panicking
	appErr := MustNew(CodeValidationFailed, "fallback message")
	var nilTranslator *Translator
	if got := nilTranslator.Translate(context.Background(), appErr); got != "fallback message" {
		t.Errorf("nilTranslator.Translate = %q, want fallback message", got)
	}
}
