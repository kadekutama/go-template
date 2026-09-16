package service

import (
	"context"
	"errors"

	"github.com/kadekutama/go-template/internal/domain/entity"
	apperror "github.com/kadekutama/go-template/internal/shared/kernel/error"
)

// domainCodeMap translates stable domain failure codes to registered kernel
// envelope codes. Domain business rejections are client errors; only unknown
// shapes become INTERNAL_ERROR. Codes not listed here map to
// VALIDATION_FAILED with the domain message preserved.
var domainCodeMap = map[string]apperror.Code{
	"PLATFORM_ACCOUNT_NOT_FOUND":  apperror.CodeNotFound,
	"CONNECTED_ACCOUNT_NOT_FOUND": apperror.CodeNotFound,
	"PROCESSOR_ACCOUNT_NOT_FOUND": apperror.CodeNotFound,
	"POSTING_NOT_FOUND":           apperror.CodeNotFound,
	"HOLD_NOT_FOUND":              apperror.CodeNotFound,
	"IDEMPOTENCY_CONFLICT":        apperror.CodeConflict,
	"FORBIDDEN":                   apperror.CodeForbidden,
	"PAYOUT_BLOCKED":              apperror.CodePayoutBlocked,
}

// ToAppError converts err to the transport envelope. *apperror.AppError
// passes through untouched and nil stays nil; domain *entity.Error maps
// through domainCodeMap with its message and cause preserved; anything else
// becomes INTERNAL_ERROR wrapping the original.
func ToAppError(err error) *apperror.AppError {
	if err == nil {
		return nil
	}
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	var domainErr *entity.Error
	if errors.As(err, &domainErr) {
		code, ok := domainCodeMap[domainErr.Code]
		if !ok {
			code = apperror.CodeValidationFailed
		}
		if converted, buildErr := apperror.New(code, domainErr.Message, apperror.WithCause(domainErr)); buildErr == nil {
			return converted
		}
	}
	converted, _ := apperror.New(apperror.CodeInternalError, err.Error(), apperror.WithCause(err))
	return converted
}

// Localize renders err for the context locale through translator. It never
// blanks messages: unknown codes or locales fall back to the source message,
// and nil renders empty.
func Localize(ctx context.Context, translator *apperror.Translator, err error) string {
	if err == nil {
		return ""
	}
	return translator.Translate(ctx, ToAppError(err))
}
