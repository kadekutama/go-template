package jsonparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// moneyShape mirrors ledger integer-minor amounts: exact int64 round-trip is
// money-safety relevant (no float encoding may sneak in).
type moneyShape struct {
	AccountID   string `json:"account_id"`
	AmountMinor int64  `json:"amount_minor"`
	AssetCode   string `json:"asset_code"`
}

func TestRoundTripMinorMoney(t *testing.T) {
	t.Parallel()

	in := moneyShape{AccountID: "acc_01", AmountMinor: 9_007_199_254_740_993, AssetCode: "USD"}
	data, err := Marshal(in)
	assert.NoError(t, err)
	assert.NotContains(t, string(data), "e+")
	assert.NotContains(t, string(data), "E+")

	var out moneyShape
	err = Unmarshal(data, &out)
	assert.NoError(t, err)
	assert.Equal(t, in, out)
}

func TestRoundTripLargePayload(t *testing.T) {
	t.Parallel()

	in := make([]moneyShape, 0, 20000)
	for i := 0; i < 20000; i++ {
		in = append(in, moneyShape{AccountID: "acc", AmountMinor: int64(i), AssetCode: "IDR"})
	}
	data, err := Marshal(in)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(data), 1<<20)

	var out []moneyShape
	err = Unmarshal(data, &out)
	assert.NoError(t, err)
	assert.Equal(t, len(in), len(out))
	assert.Equal(t, int64(19999), out[19999].AmountMinor)
}

func TestGet(t *testing.T) {
	t.Parallel()

	doc := []byte(`{"entries":[{"amount_minor":1500,"asset":"USD"}],"meta":{"tenant":"t1"}}`)

	type testCase struct {
		name           string
		data           []byte
		path           []string
		expectedResult string
		expectedError  bool
	}

	testCases := []testCase{
		{
			name:           "valid array element field traversal",
			data:           doc,
			path:           []string{"entries", "0", "amount_minor"},
			expectedResult: "1500",
			expectedError:  false,
		},
		{
			name:           "out-of-range array index rejected",
			data:           doc,
			path:           []string{"entries", "3", "amount_minor"},
			expectedResult: "",
			expectedError:  true,
		},
		{
			name:           "missing map key rejected",
			data:           doc,
			path:           []string{"meta", "missing"},
			expectedResult: "",
			expectedError:  true,
		},
		{
			name:           "descending into scalar rejected",
			data:           doc,
			path:           []string{"meta", "tenant", "deep"},
			expectedResult: "",
			expectedError:  true,
		},
		{
			name:           "malformed json document rejected",
			data:           []byte(`{oops`),
			path:           []string{"a"},
			expectedResult: "",
			expectedError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Get(tc.data, tc.path...)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResult, string(got))
			}
		})
	}
}

func TestMarshal(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		v             any
		expectedError bool
	}

	testCases := []testCase{
		{
			name: "valid struct round-trip",
			v: moneyShape{
				AccountID:   "acc_01",
				AmountMinor: 9_007_199_254_740_993,
				AssetCode:   "USD",
			},
			expectedError: false,
		},
		{
			name:          "unsupported channel type fails",
			v:             make(chan int),
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := Marshal(tc.v)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotContains(t, string(data), "e+")
				assert.NotContains(t, string(data), "E+")
			}
		})
	}
}

func TestUnmarshal(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		data          []byte
		target        any
		expectedError bool
	}

	testCases := []testCase{
		{
			name:          "valid json unmarshals",
			data:          []byte(`{"account_id":"acc_01","amount_minor":100,"asset_code":"USD"}`),
			target:        &moneyShape{},
			expectedError: false,
		},
		{
			name:          "malformed json fails",
			data:          []byte("{invalid-json"),
			target:        &map[string]any{},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := Unmarshal(tc.data, tc.target)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func BenchmarkMarshal(b *testing.B) {
	in := moneyShape{AccountID: "acc_bench", AmountMinor: 123456, AssetCode: "USD"}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Marshal(in); err != nil {
			b.Fatalf("Marshal: %v", err)
		}
	}
}
