package valueobject_test

import (
	"encoding/json"
	"errors"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func usd(t *testing.T, minor int64) valueobject.Money {
	t.Helper()
	m, err := valueobject.NewMoney(minor, testUSD)
	if err != nil {
		t.Fatalf("NewMoney: %v", err)
	}
	return m
}

func TestMoneyAdd(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		a              valueobject.Money
		b              valueobject.Money
		expectedResult valueobject.Money
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "exact tenth plus fifth equals three tenths",
			a:              usd(t, 10),
			b:              usd(t, 20),
			expectedResult: usd(t, 30),
			expectedError:  nil,
		},
		{
			name:           "negative plus positive",
			a:              usd(t, -50),
			b:              usd(t, 100),
			expectedResult: usd(t, 50),
			expectedError:  nil,
		},
		{
			name: "currency mismatch",
			a:    usd(t, 100),
			b: func() valueobject.Money {
				m, _ := valueobject.NewMoney(50, testEUR)
				return m
			}(),
			expectedResult: valueobject.Money{},
			expectedError:  errors.New("money: CURRENCY_MISMATCH"),
		},
		{
			name:           "overflow on add",
			a:              usd(t, math.MaxInt64),
			b:              usd(t, 1),
			expectedResult: valueobject.Money{},
			expectedError:  errors.New("money: arithmetic overflow"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.a.Add(tc.b)
			if tc.expectedError != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResult, got)
			}
		})
	}
}

func TestCrossCurrencyMismatch(t *testing.T) {
	t.Parallel()

	eurMoney, err := valueobject.NewMoney(50, testEUR)
	assert.NoError(t, err)

	type testCase struct {
		name          string
		op            func(a, b valueobject.Money) error
		expectedError string
	}

	testCases := []testCase{
		{
			name: "Add cross-currency",
			op: func(a, b valueobject.Money) error {
				_, opErr := a.Add(b)
				return opErr
			},
			expectedError: "CURRENCY_MISMATCH",
		},
		{
			name: "Sub cross-currency",
			op: func(a, b valueobject.Money) error {
				_, opErr := a.Sub(b)
				return opErr
			},
			expectedError: "CURRENCY_MISMATCH",
		},
		{
			name: "Compare cross-currency",
			op: func(a, b valueobject.Money) error {
				_, opErr := a.Compare(b)
				return opErr
			},
			expectedError: "CURRENCY_MISMATCH",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			opErr := tc.op(usd(t, 100), eurMoney)
			assert.Error(t, opErr)
			assert.Contains(t, opErr.Error(), tc.expectedError)
		})
	}
}

func TestOverflowRejected(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name string
		op   func() error
	}

	testCases := []testCase{
		{
			name: "MaxInt64+1 overflow",
			op: func() error {
				_, err := usd(t, math.MaxInt64).Add(usd(t, 1))
				return err
			},
		},
		{
			name: "MaxInt64*2 overflow",
			op: func() error {
				_, err := usd(t, math.MaxInt64).MulScalar(2)
				return err
			},
		},
		{
			name: "MinInt64-1 overflow",
			op: func() error {
				_, err := usd(t, math.MinInt64).Sub(usd(t, 1))
				return err
			},
		},
		{
			name: "division by zero",
			op: func() error {
				_, err := usd(t, 1).DivScalar(0)
				return err
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.op()
			assert.Error(t, err)
		})
	}
}

func TestArithmeticProperties(t *testing.T) {
	t.Parallel()

	// #nosec G404 -- deterministic pseudo-random generator for property testing
	rng := rand.New(rand.NewPCG(42, 100))
	vals := make([]int64, 0, 64)
	for i := 0; i < 64; i++ {
		vals = append(vals, rng.Int64N(1_000_000_000)-500_000_000)
	}
	for i := 0; i < 1000; i++ {
		a, b, c := vals[rng.IntN(len(vals))], vals[rng.IntN(len(vals))], vals[rng.IntN(len(vals))]
		ma, mb, mc := usd(t, a), usd(t, b), usd(t, c)
		ab, _ := ma.Add(mb)
		abc1, _ := ab.Add(mc)
		bc, _ := mb.Add(mc)
		abc2, _ := ma.Add(bc)
		assert.Equal(t, abc1.AmountMinor(), abc2.AmountMinor(), "associativity failed")

		ba, _ := mb.Add(ma)
		assert.Equal(t, ab.AmountMinor(), ba.AmountMinor(), "commutativity failed")
	}
}

func TestMoneyJSONShape(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		dto           valueobject.MoneyDTO
		expectedError error
	}

	testCases := []testCase{
		{
			name: "valid DTO round-trip",
			dto: valueobject.MoneyDTO{
				AmountMinor: 1050,
				AssetCode:   testUSD,
			},
			expectedError: nil,
		},
		{
			name: "missing asset code",
			dto: valueobject.MoneyDTO{
				AmountMinor: 10,
				AssetCode:   "",
			},
			expectedError: errors.New("money: asset required"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			money, err := valueobject.MoneyFromDTO(tc.dto)
			if tc.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.dto.AmountMinor, money.AmountMinor())
				assert.Equal(t, tc.dto.AssetCode, money.Asset())
			}
		})
	}

	// JSON encoding check: ensure exact int64 and no exponent notation
	data, err := json.Marshal(usd(t, 1050).DTO())
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"amount_minor":1050`)
	assert.Contains(t, string(data), `"asset_code":"`+testUSD+`"`)

	var bad valueobject.MoneyDTO
	err = json.Unmarshal([]byte(`{"amount_minor":10.5,"asset_code":"`+testUSD+`"}`), &bad)
	assert.Error(t, err, "float quantity must not decode into int64")
}

func TestMoneyFormat(t *testing.T) {
	t.Parallel()

	reg, err := valueobject.NewRegistry(
		valueobject.AssetInfo{Code: testUSD, Exponent: 2, Kind: valueobject.AssetKindFiat},
		valueobject.AssetInfo{Code: "JPY", Exponent: 0, Kind: valueobject.AssetKindFiat},
	)
	assert.NoError(t, err)

	type testCase struct {
		name           string
		money          valueobject.Money
		expectedResult string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "standard USD two decimals",
			money:          usd(t, 1050),
			expectedResult: "10.50",
			expectedError:  nil,
		},
		{
			name:           "zero-exponent JPY no decimals",
			money:          valueobject.MustMoney(100, "JPY"),
			expectedResult: "100",
			expectedError:  nil,
		},
		{
			name:           "unregistered currency format fails",
			money:          valueobject.MustMoney(1, "EUR"),
			expectedResult: "",
			expectedError:  errors.New("format: unregistered currency"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, formatErr := tc.money.Format(reg)
			if tc.expectedError != nil {
				assert.Error(t, formatErr)
			} else {
				assert.NoError(t, formatErr)
				assert.Equal(t, tc.expectedResult, got)
			}
		})
	}
}

func TestAllocateLargestRemainder(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		total          int64
		weights        []int64
		expectedResult []int64
		expectedError  bool
	}

	testCases := []testCase{
		{
			name:           "standard split 50 30 20",
			total:          100,
			weights:        []int64{50, 30, 20},
			expectedResult: []int64{50, 30, 20},
			expectedError:  false,
		},
		{
			name:           "three-way split with remainder 34 33 33",
			total:          100,
			weights:        []int64{1, 1, 1},
			expectedResult: []int64{34, 33, 33},
			expectedError:  false,
		},
		{
			name:           "Hare-Niemeyer ranking unequal weights 2 5 4",
			total:          11,
			weights:        []int64{100, 300, 200},
			expectedResult: []int64{2, 5, 4},
			expectedError:  false,
		},
		{
			name:           "Hare-Niemeyer ranking 2 3 5",
			total:          10,
			weights:        []int64{1, 2, 3},
			expectedResult: []int64{2, 3, 5},
			expectedError:  false,
		},
		{
			name:           "nil weights",
			total:          100,
			weights:        nil,
			expectedResult: nil,
			expectedError:  true,
		},
		{
			name:           "empty weights",
			total:          100,
			weights:        []int64{},
			expectedResult: nil,
			expectedError:  true,
		},
		{
			name:           "negative weight",
			total:          100,
			weights:        []int64{1, -1},
			expectedResult: nil,
			expectedError:  true,
		},
		{
			name:           "min int64 total overflow",
			total:          math.MinInt64,
			weights:        []int64{1, 1},
			expectedResult: nil,
			expectedError:  true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := valueobject.AllocateLargestRemainder(tc.total, tc.weights)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResult, got)
				var sum int64
				for _, s := range got {
					sum += s
				}
				assert.Equal(t, tc.total, sum)
			}
		})
	}
}
