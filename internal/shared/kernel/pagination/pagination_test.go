package pagination

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCursorEncode(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		c             Cursor
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid positive offset",
			c:             Cursor{Offset: 120},
			expectedError: nil,
		},
		{
			name:          "zero offset",
			c:             Cursor{Offset: 0},
			expectedError: nil,
		},
		{
			name:          "negative offset fails",
			c:             Cursor{Offset: -1},
			expectedError: errors.New("pagination: negative offset"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			token, err := tc.c.Encode()
			if tc.expectedError != nil {
				assert.Equal(t, tc.expectedError, err)
				assert.Empty(t, token)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, token)
				assert.False(t, strings.Contains(token, "120"))
			}
		})
	}
}

func TestDecodeCursor(t *testing.T) {
	t.Parallel()

	validToken, err := Cursor{Offset: 120}.Encode()
	assert.NoError(t, err)

	type testCase struct {
		name           string
		token          string
		expectedResult Cursor
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid encoded cursor",
			token:          validToken,
			expectedResult: Cursor{Offset: 120},
			expectedError:  nil,
		},
		{
			name: "tampered checksum rejected",
			token: func() string {
				tok, _ := Cursor{Offset: 7}.Encode()
				return tok[:len(tok)-2] + "AA"
			}(),
			expectedResult: Cursor{},
			expectedError:  errors.New("pagination: cursor checksum mismatch"),
		},
		{
			name:           "malformed non-base64 garbage rejected",
			token:          "!!!",
			expectedResult: Cursor{},
			expectedError:  errors.New("pagination: malformed cursor"),
		},
		{
			name: "overflow cursor offset rejected",
			token: func() string {
				var payload [8]byte
				binary.BigEndian.PutUint64(payload[:], math.MaxUint64)
				sum := crc32.Checksum(payload[:], crc32.MakeTable(crc32.Castagnoli))
				var raw [12]byte
				copy(raw[:8], payload[:])
				binary.BigEndian.PutUint32(raw[8:], sum)
				return base64.RawURLEncoding.EncodeToString(raw[:])
			}(),
			expectedResult: Cursor{},
			expectedError:  errors.New("pagination: cursor offset overflow"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodeCursor(tc.token)
			if tc.expectedError != nil {
				assert.Equal(t, tc.expectedError, err)
				assert.Equal(t, tc.expectedResult, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResult, got)
			}
		})
	}
}

func TestPageRequestValidate(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		r             PageRequest
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "excessive limit rejected",
			r:             PageRequest{Limit: 9999, Offset: 0},
			expectedError: errors.New("pagination: limit must be between 1 and 200"),
		},
		{
			name:          "zero limit rejected",
			r:             PageRequest{Limit: 0, Offset: 0},
			expectedError: errors.New("pagination: limit must be between 1 and 200"),
		},
		{
			name:          "negative offset rejected",
			r:             PageRequest{Limit: 50, Offset: -5},
			expectedError: errors.New("pagination: offset must be non-negative"),
		},
		{
			name:          "valid request accepted",
			r:             PageRequest{Limit: 25, Offset: 50},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.r.Validate()
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseLimitOffset(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		limitStr       string
		offsetStr      string
		expectedResult PageRequest
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "non-numeric limit rejected",
			limitStr:       "abc",
			offsetStr:      "0",
			expectedResult: PageRequest{},
			expectedError:  errors.New("pagination: invalid limit integer"),
		},
		{
			name:           "empty limit rejected",
			limitStr:       "",
			offsetStr:      "0",
			expectedResult: PageRequest{},
			expectedError:  errors.New("pagination: limit is required"),
		},
		{
			name:           "empty offset rejected",
			limitStr:       "25",
			offsetStr:      "",
			expectedResult: PageRequest{},
			expectedError:  errors.New("pagination: offset is required"),
		},
		{
			name:           "negative offset rejected",
			limitStr:       "25",
			offsetStr:      "-3",
			expectedResult: PageRequest{},
			expectedError:  errors.New("pagination: offset must be non-negative"),
		},
		{
			name:           "valid numeric values parsed",
			limitStr:       "25",
			offsetStr:      "50",
			expectedResult: PageRequest{Limit: 25, Offset: 50},
			expectedError:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := ParseLimitOffset(tc.limitStr, tc.offsetStr)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResult, actualResult)
			}
		})
	}
}

func TestPageResultShape(t *testing.T) {
	t.Parallel()

	result := PageResult[string]{
		Items:  []string{"item1", "item2"},
		Total:  100,
		Limit:  50,
		Offset: 0,
	}

	assert.Len(t, result.Items, 2)
	assert.Equal(t, int64(100), result.Total)
	assert.Equal(t, 50, result.Limit)
	assert.Equal(t, 0, result.Offset)
}
