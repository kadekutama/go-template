package log_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

func TestFieldConstructors(t *testing.T) {
	t.Parallel()

	type dummyMeta struct {
		ID int
	}
	sampleErr := errors.New("sample error")

	type testCase struct {
		name          string
		field         log.Field
		expectedKey   string
		expectedValue any
	}

	testCases := []testCase{
		{
			name:          "error field with err",
			field:         log.Err(sampleErr),
			expectedKey:   log.FieldError,
			expectedValue: sampleErr,
		},
		{
			name:          "error field with nil",
			field:         log.Err(nil),
			expectedKey:   log.FieldError,
			expectedValue: nil,
		},
		{
			name:          "metadata field",
			field:         log.Metadata(dummyMeta{ID: 42}),
			expectedKey:   log.FieldMetadata,
			expectedValue: dummyMeta{ID: 42},
		},
		{
			name:          "request field",
			field:         log.Request("req-data"),
			expectedKey:   log.FieldRequest,
			expectedValue: "req-data",
		},
		{
			name:          "response field",
			field:         log.Response("resp-data"),
			expectedKey:   log.FieldResponse,
			expectedValue: "resp-data",
		},
		{
			name:          "any field",
			field:         log.Any("custom_key", 123),
			expectedKey:   "custom_key",
			expectedValue: 123,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expectedKey, tc.field.Key)
			assert.Equal(t, tc.expectedValue, tc.field.Value)
		})
	}
}
