package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/service"
	"github.com/kadekutama/go-template/internal/domain/entity"
	apperror "github.com/kadekutama/go-template/internal/shared/kernel/error"
)

func TestToAppError(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name         string
		err          error
		expectedCode apperror.Code
		expectedNil  bool
	}

	testCases := []testCase{
		{
			name:        "nil error returns nil",
			err:         nil,
			expectedNil: true,
		},
		{
			name: "app error passes through untouched",
			err: func() error {
				appErr, _ := apperror.New(apperror.CodeNotFound, "not found")
				return appErr
			}(),
			expectedCode: apperror.CodeNotFound,
			expectedNil:  false,
		},
		{
			name:         "domain error with mapped code",
			err:          entity.NewError("PLATFORM_ACCOUNT_NOT_FOUND", "platform account missing"),
			expectedCode: apperror.CodeNotFound,
			expectedNil:  false,
		},
		{
			name:         "domain error with unmapped code defaults to validation failed",
			err:          entity.NewError("SOME_OTHER_BUSINESS_ERROR", "invalid business rule"),
			expectedCode: apperror.CodeValidationFailed,
			expectedNil:  false,
		},
		{
			name:         "unknown generic error becomes internal error",
			err:          errors.New("db connection failure"),
			expectedCode: apperror.CodeInternalError,
			expectedNil:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := service.ToAppError(tc.err)
			if tc.expectedNil {
				assert.Nil(t, actual)
				return
			}
			require.NotNil(t, actual)
			assert.Equal(t, tc.expectedCode, actual.Code)
		})
	}
}

func TestLocalize(t *testing.T) {
	t.Parallel()

	translator, err := apperror.NewTranslator()
	require.NoError(t, err)

	type testCase struct {
		name            string
		ctx             context.Context
		translator      *apperror.Translator
		err             error
		expectedMessage string
	}

	testCases := []testCase{
		{
			name:            "nil error returns empty string",
			ctx:             context.Background(),
			translator:      translator,
			err:             nil,
			expectedMessage: "",
		},
		{
			name:            "english localization for not found",
			ctx:             apperror.WithLocale(context.Background(), "en"),
			translator:      translator,
			err:             entity.NewError("PLATFORM_ACCOUNT_NOT_FOUND", "platform account missing"),
			expectedMessage: "resource not found",
		},
		{
			name:            "indonesian localization for not found",
			ctx:             apperror.WithLocale(context.Background(), "id"),
			translator:      translator,
			err:             entity.NewError("PLATFORM_ACCOUNT_NOT_FOUND", "platform account missing"),
			expectedMessage: "sumber daya tidak ditemukan",
		},
		{
			name:            "generic error translates to localized internal server error",
			ctx:             apperror.WithLocale(context.Background(), "en"),
			translator:      translator,
			err:             errors.New("custom error message"),
			expectedMessage: "internal server error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := service.Localize(tc.ctx, tc.translator, tc.err)
			assert.Equal(t, tc.expectedMessage, actual)
		})
	}
}
