package apperror

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

var testCodeCounter atomic.Int64

func TestNew(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		code           Code
		message        string
		opts           []Option
		expectedStatus int
		expectedError  bool
	}

	testCases := []testCase{
		{
			name:           "valid registered envelope",
			code:           CodeValidationFailed,
			message:        "bad input",
			opts:           []Option{WithDetails(map[string]any{"field": "email"})},
			expectedStatus: http.StatusBadRequest,
			expectedError:  false,
		},
		{
			name:           "custom http status override",
			code:           CodeInternalError,
			message:        "timed out",
			opts:           []Option{WithHTTPStatus(http.StatusGatewayTimeout)},
			expectedStatus: http.StatusGatewayTimeout,
			expectedError:  false,
		},
		{
			name:           "unregistered code rejected",
			code:           "NOPE_X",
			message:        "nope",
			opts:           nil,
			expectedStatus: 0,
			expectedError:  true,
		},
		{
			name:           "empty code rejected",
			code:           "",
			message:        "empty",
			opts:           nil,
			expectedStatus: 0,
			expectedError:  true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err, newErr := New(tc.code, tc.message, tc.opts...)
			if tc.expectedError {
				assert.Error(t, newErr)
				assert.Nil(t, err)
			} else {
				assert.NoError(t, newErr)
				assert.NotNil(t, err)
				assert.Equal(t, tc.code, err.Code)
				assert.Equal(t, tc.message, err.Message)
				assert.Equal(t, tc.expectedStatus, err.HTTPStatus)
			}
		})
	}
}

func TestRegisterValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		codes         map[Code]int
		expectedPanic bool
	}

	testCases := []testCase{
		{
			name:          "empty code",
			codes:         map[Code]int{"": http.StatusBadRequest},
			expectedPanic: true,
		},
		{
			name:          "starts with number",
			codes:         map[Code]int{"123_STARTS_WITH_NUMBER": http.StatusBadRequest},
			expectedPanic: true,
		},
		{
			name:          "lowercase code",
			codes:         map[Code]int{"lowercase_code": http.StatusBadRequest},
			expectedPanic: true,
		},
		{
			name:          "trailing underscore",
			codes:         map[Code]int{"TRAILING_UNDERSCORE_": http.StatusBadRequest},
			expectedPanic: true,
		},
		{
			name:          "dash not allowed",
			codes:         map[Code]int{"DASH-NOT-ALLOWED": http.StatusBadRequest},
			expectedPanic: true,
		},
		{
			name:          "leading underscore",
			codes:         map[Code]int{"_LEADING_UNDERSCORE": http.StatusBadRequest},
			expectedPanic: true,
		},
		{
			name:          "duplicate code",
			codes:         map[Code]int{CodeNotFound: http.StatusNotFound},
			expectedPanic: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.expectedPanic {
				assert.Panics(t, func() {
					Register(tc.codes)
				})
			} else {
				assert.NotPanics(t, func() {
					Register(tc.codes)
				})
			}
		})
	}
}

func TestHTTPStatusFor(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		code           Code
		expectedResult int
	}

	testCases := []testCase{
		{
			name:           "not found code",
			code:           CodeNotFound,
			expectedResult: http.StatusNotFound,
		},
		{
			name:           "validation failed code",
			code:           CodeValidationFailed,
			expectedResult: http.StatusBadRequest,
		},
		{
			name:           "conflict code",
			code:           CodeConflict,
			expectedResult: http.StatusConflict,
		},
		{
			name:           "unauthorized code",
			code:           CodeUnauthorized,
			expectedResult: http.StatusUnauthorized,
		},
		{
			name:           "unknown code falls back to internal server error",
			code:           "UNKNOWN_NONEXISTENT_CODE",
			expectedResult: http.StatusInternalServerError,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult := HTTPStatusFor(tc.code)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestTranslatorTranslate(t *testing.T) {
	t.Parallel()

	translator, err := NewTranslator()
	assert.NoError(t, err)

	type testCase struct {
		name           string
		ctx            context.Context
		err            *AppError
		expectedResult string
	}

	testCases := []testCase{
		{
			name:           "english locale translation",
			ctx:            WithLocale(context.Background(), "en"),
			err:            MustNew(CodeValidationFailed, "fallback"),
			expectedResult: "request failed validation",
		},
		{
			name:           "indonesian locale translation",
			ctx:            WithLocale(context.Background(), "id"),
			err:            MustNew(CodeValidationFailed, "fallback"),
			expectedResult: "permintaan gagal validasi",
		},
		{
			name:           "default context falls back to english catalog",
			ctx:            context.Background(),
			err:            MustNew(CodeNotFound, "gone"),
			expectedResult: "resource not found",
		},
		{
			name:           "nil error returns empty string",
			ctx:            context.Background(),
			err:            nil,
			expectedResult: "",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult := translator.Translate(tc.ctx, tc.err)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestAppErrorFormatting(t *testing.T) {
	t.Parallel()

	sentinel := http.ErrHandlerTimeout

	type testCase struct {
		name           string
		err            *AppError
		expectedError  string
		expectedUnwrap error
	}

	testCases := []testCase{
		{
			name:           "simple error without cause",
			err:            MustNew(CodeConflict, "conflict occurred"),
			expectedError:  "CONFLICT: conflict occurred",
			expectedUnwrap: nil,
		},
		{
			name:           "wrapped error with cause",
			err:            MustNew(CodeInternalError, "timed out", WithCause(sentinel), WithHTTPStatus(http.StatusGatewayTimeout)),
			expectedError:  "INTERNAL_ERROR: timed out: http: Handler timeout",
			expectedUnwrap: sentinel,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expectedError, tc.err.Error())
			assert.Equal(t, tc.expectedUnwrap, tc.err.Unwrap())
		})
	}
}

func TestMustNewPanicsOnUnregistered(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		code          Code
		message       string
		expectedPanic bool
	}

	testCases := []testCase{
		{
			name:          "unregistered code panics",
			code:          "UNKNOWN_CODE",
			message:       "message",
			expectedPanic: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Panics(t, func() {
				_ = MustNew(tc.code, tc.message)
			})
		})
	}
}

func TestRegisterValidCustomCode(t *testing.T) {
	id := testCodeCounter.Add(1)
	customCode := Code(fmt.Sprintf("CUSTOM_CODE_%d", id))
	Register(map[Code]int{customCode: http.StatusGatewayTimeout})

	err, newErr := New(customCode, "timed out at provider")
	assert.NoError(t, newErr)
	assert.Equal(t, customCode, err.Code)
	assert.Equal(t, http.StatusGatewayTimeout, err.HTTPStatus)
	assert.Equal(t, http.StatusGatewayTimeout, HTTPStatusFor(customCode))
}

func TestNilTranslatorSafety(t *testing.T) {
	t.Parallel()

	appErr := MustNew(CodeValidationFailed, "fallback message")
	var nilTranslator *Translator
	assert.Equal(t, "fallback message", nilTranslator.Translate(context.Background(), appErr))
}
