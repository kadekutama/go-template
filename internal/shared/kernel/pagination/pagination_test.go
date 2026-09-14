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

func TestPageRequestNormalize(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		r              PageRequest
		expectedResult PageRequest
	}

	testCases := []testCase{
		{
			name: "excessive limit and negative offset clamped",
			r:    PageRequest{Limit: 9999, Offset: -5},
			expectedResult: PageRequest{
				Limit:  MaxLimit,
				Offset: 0,
			},
		},
		{
			name: "empty request defaulted",
			r:    PageRequest{},
			expectedResult: PageRequest{
				Limit:  DefaultLimit,
				Offset: 0,
			},
		},
		{
			name: "valid request preserved",
			r:    PageRequest{Limit: 25, Offset: 50},
			expectedResult: PageRequest{
				Limit:  25,
				Offset: 50,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := tc.r.Normalize()
			assert.Equal(t, tc.expectedResult, actualResult)
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
	}

	testCases := []testCase{
		{
			name:      "non-numeric strings fall back to defaults",
			limitStr:  "abc",
			offsetStr: "-3",
			expectedResult: PageRequest{
				Limit:  DefaultLimit,
				Offset: 0,
			},
		},
		{
			name:      "valid numeric values parsed",
			limitStr:  "25",
			offsetStr: "50",
			expectedResult: PageRequest{
				Limit:  25,
				Offset: 50,
			},
		},
		{
			name:      "empty strings fall back to defaults",
			limitStr:  "",
			offsetStr: "",
			expectedResult: PageRequest{
				Limit:  DefaultLimit,
				Offset: 0,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := ParseLimitOffset(tc.limitStr, tc.offsetStr)
			assert.Equal(t, tc.expectedResult, actualResult)
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
