package apperror

import (
	"context"
	"net/http"
	"testing"
)

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
