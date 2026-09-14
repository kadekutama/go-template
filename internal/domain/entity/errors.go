package entity

import (
	"fmt"
)

// Error is a coded domain failure. Codes are stable UPPER_SNAKE tokens that
// application and interface layers map to the AppError envelope; the domain
// never constructs transport errors itself.
type Error struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *Error) Error() string { return e.Code + ": " + e.Message }

// NewError builds a coded domain error.
func NewError(code, message string) *Error { return &Error{Code: code, Message: message} }

// Errorf builds a coded domain error with formatting.
func Errorf(code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}
