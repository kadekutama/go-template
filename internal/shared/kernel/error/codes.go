package apperror

import "net/http"

// Kernel error codes. Domain-specific codes live with their owning epics and
// register via Register; the UPPER_SNAKE shape is enforced there.
const (
	CodeValidationFailed Code = "VALIDATION_FAILED"
	CodeNotFound         Code = "NOT_FOUND"
	CodeConflict         Code = "CONFLICT"
	CodeUnauthorized     Code = "UNAUTHORIZED"
	CodeForbidden        Code = "FORBIDDEN"
	CodeInternalError    Code = "INTERNAL_ERROR"
	CodePayoutBlocked    Code = "PAYOUT_BLOCKED"
	CodeOutcomeUnknown   Code = "OUTCOME_UNKNOWN"
)

func defaultCodes() map[Code]int {
	return map[Code]int{
		CodeValidationFailed: http.StatusBadRequest,
		CodeNotFound:         http.StatusNotFound,
		CodeConflict:         http.StatusConflict,
		CodeUnauthorized:     http.StatusUnauthorized,
		CodeForbidden:        http.StatusForbidden,
		CodeInternalError:    http.StatusInternalServerError,
		CodePayoutBlocked:    http.StatusUnprocessableEntity,
		CodeOutcomeUnknown:   http.StatusConflict,
	}
}
