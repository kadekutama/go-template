package valueobject_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestMoneyScalarEdges(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		money          valueobject.Money
		op             func(m valueobject.Money) (valueobject.Money, error)
		expectedResult valueobject.Money
		expectedError  bool
	}

	testCases := []testCase{
		{
			name:  "multiply by zero returns zero",
			money: valueobject.MustMoney(7, testUSD),
			op: func(m valueobject.Money) (valueobject.Money, error) {
				return m.MulScalar(0)
			},
			expectedResult: valueobject.MustMoney(0, testUSD),
			expectedError:  false,
		},
		{
			name:  "multiply by negative three",
			money: valueobject.MustMoney(7, testUSD),
			op: func(m valueobject.Money) (valueobject.Money, error) {
				return m.MulScalar(-3)
			},
			expectedResult: valueobject.MustMoney(-21, testUSD),
			expectedError:  false,
		},
		{
			name:  "divide by two truncates",
			money: valueobject.MustMoney(7, testUSD),
			op: func(m valueobject.Money) (valueobject.Money, error) {
				return m.DivScalar(2)
			},
			expectedResult: valueobject.MustMoney(3, testUSD),
			expectedError:  false,
		},
		{
			name:  "MinInt64 divided by negative one overflows",
			money: valueobject.MustMoney(math.MinInt64, testUSD),
			op: func(m valueobject.Money) (valueobject.Money, error) {
				return m.DivScalar(-1)
			},
			expectedResult: valueobject.Money{},
			expectedError:  true,
		},
		{
			name:  "MinInt64 multiplied by negative one overflows",
			money: valueobject.MustMoney(math.MinInt64, testUSD),
			op: func(m valueobject.Money) (valueobject.Money, error) {
				return m.MulScalar(-1)
			},
			expectedResult: valueobject.Money{},
			expectedError:  true,
		},
		{
			name:  "negative one multiplied by MinInt64 overflows",
			money: valueobject.MustMoney(-1, testUSD),
			op: func(m valueobject.Money) (valueobject.Money, error) {
				return m.MulScalar(math.MinInt64)
			},
			expectedResult: valueobject.Money{},
			expectedError:  true,
		},
		{
			name:  "MinInt64 minus one overflows",
			money: valueobject.MustMoney(math.MinInt64, testUSD),
			op: func(m valueobject.Money) (valueobject.Money, error) {
				return m.Sub(valueobject.MustMoney(1, testUSD))
			},
			expectedResult: valueobject.Money{},
			expectedError:  true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.op(tc.money)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResult, got)
			}
		})
	}

	_, emptyErr := valueobject.NewMoney(1, "")
	assert.Error(t, emptyErr, "empty asset must error")

	assert.Panics(t, func() {
		valueobject.MustMoney(1, "")
	}, "MustMoney with empty asset must panic")
}

func TestMoneyCompareAndPredicates(t *testing.T) {
	t.Parallel()

	a := valueobject.MustMoney(5, testUSD)
	b := valueobject.MustMoney(9, testUSD)
	aCopy := valueobject.MustMoney(5, testUSD)

	type testCase struct {
		name           string
		m1             valueobject.Money
		m2             valueobject.Money
		expectedResult int
	}

	testCases := []testCase{
		{
			name:           "equal values compare to zero",
			m1:             a,
			m2:             aCopy,
			expectedResult: 0,
		},
		{
			name:           "lesser value compares to negative one",
			m1:             a,
			m2:             b,
			expectedResult: -1,
		},
		{
			name:           "greater value compares to positive one",
			m1:             b,
			m2:             a,
			expectedResult: 1,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, err := tc.m1.Compare(tc.m2)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedResult, c)
		})
	}

	assert.True(t, valueobject.MustMoney(0, testUSD).IsZero())
	assert.False(t, a.IsZero())
	assert.True(t, a.IsPositive())
	assert.False(t, valueobject.MustMoney(-1, testUSD).IsPositive())
	assert.Equal(t, valueobject.AssetCode(testUSD), a.Asset())
	assert.Equal(t, int64(5), a.AmountMinor())
}

func TestMoneyFormatNegative(t *testing.T) {
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
	}

	testCases := []testCase{
		{
			name:           "negative USD formatting",
			money:          valueobject.MustMoney(-1050, testUSD),
			expectedResult: "-10.50",
		},
		{
			name:           "negative JPY formatting",
			money:          valueobject.MustMoney(-5, "JPY"),
			expectedResult: "-5",
		},
		{
			name:           "min int64 USD formatting",
			money:          valueobject.MustMoney(math.MinInt64, testUSD),
			expectedResult: "-92233720368547758.08",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s, formatErr := tc.money.Format(reg)
			assert.NoError(t, formatErr)
			assert.Equal(t, tc.expectedResult, s)
		})
	}
}

func TestAllocateEdges(t *testing.T) {
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
			name:           "negative total mirrors sign onto exact-sum shares",
			total:          -100,
			weights:        []int64{1, 1, 1},
			expectedResult: []int64{-34, -33, -33},
			expectedError:  false,
		},
		{
			name:           "all-zero weights split evenly",
			total:          10,
			weights:        []int64{0, 0},
			expectedResult: []int64{5, 5},
			expectedError:  false,
		},
		{
			name:           "overflow in weight*total product",
			total:          math.MaxInt64,
			weights:        []int64{math.MaxInt64},
			expectedResult: nil,
			expectedError:  true,
		},
		{
			name:           "weight-sum overflow",
			total:          1,
			weights:        []int64{math.MaxInt64, math.MaxInt64},
			expectedResult: nil,
			expectedError:  true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			shares, err := valueobject.AllocateLargestRemainder(tc.total, tc.weights)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResult, shares)
				var sum int64
				for _, s := range shares {
					sum += s
				}
				assert.Equal(t, tc.total, sum)
			}
		})
	}
}

func TestRegistryZeroValue(t *testing.T) {
	t.Parallel()

	var reg valueobject.Registry
	err := reg.Register(valueobject.AssetInfo{Code: "USD", Exponent: 2, Kind: valueobject.AssetKindFiat})
	assert.NoError(t, err)

	info, err := reg.Lookup("USD")
	assert.NoError(t, err)
	assert.Equal(t, valueobject.AssetCode("USD"), info.Code)

	_, err = valueobject.NewRegistry(valueobject.AssetInfo{Code: "", Exponent: 0})
	assert.Error(t, err, "NewRegistry with empty code must error")
}

func TestAllIDTypesTable(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name  string
		parse func(string) (string, error)
	}

	testCases := []testCase{
		{
			name: kindAccount,
			parse: func(s string) (string, error) {
				id, err := valueobject.ParseAccountID(s)
				return id.String(), err
			},
		},
		{
			name: kindPosting,
			parse: func(s string) (string, error) {
				id, err := valueobject.ParsePostingID(s)
				return id.String(), err
			},
		},
		{
			name: kindEntry,
			parse: func(s string) (string, error) {
				id, err := valueobject.ParseEntryID(s)
				return id.String(), err
			},
		},
		{
			name: kindHold,
			parse: func(s string) (string, error) {
				id, err := valueobject.ParseHoldID(s)
				return id.String(), err
			},
		},
		{
			name: kindTenant,
			parse: func(s string) (string, error) {
				id, err := valueobject.ParseTenantID(s)
				return id.String(), err
			},
		},
		{
			name: kindLedger,
			parse: func(s string) (string, error) {
				id, err := valueobject.ParseLedgerID(s)
				return id.String(), err
			},
		},
		{
			name: kindUser,
			parse: func(s string) (string, error) {
				id, err := valueobject.ParseUserID(s)
				return id.String(), err
			},
		},
		{
			name: kindJournal,
			parse: func(s string) (string, error) {
				id, err := valueobject.ParseJournalID(s)
				return id.String(), err
			},
		},
		{
			name: kindPeriod,
			parse: func(s string) (string, error) {
				id, err := valueobject.ParsePeriodID(s)
				return id.String(), err
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gen := &seqIDs{}
			a, b := gen.NewID(), gen.NewID()
			assert.NotEqual(t, a, b, "generated ids must be unique")

			back, err := tc.parse(a)
			assert.NoError(t, err)
			assert.Equal(t, a, back)

			_, err = tc.parse("0193e5f0-1c2c-7a4e-b3f2")
			assert.Error(t, err, "truncated uuid must error")

			_, err = tc.parse("0193e5f0-1c2c-7a4e-b3f2-9c1e5a7b9c1!")
			assert.Error(t, err, "non-hex uuid must error")

			_, err = tc.parse("0193e5f01c2c-7a4e-b3f2-9c1e5a7b9c1e")
			assert.Error(t, err, "misplaced hyphen must error")

			upperBack, err := tc.parse("0193E5F0-1C2C-7A4E-B3F2-9C1E5A7B9C1E")
			assert.NoError(t, err, "uppercase hex must parse")
			assert.NotEmpty(t, upperBack)

			_, err = tc.parse("a-b-c-d-e-f")
			assert.Error(t, err, "six groups must error")
		})
	}
}

func checkIDEquivalence(t *testing.T, s1, s2 string) {
	t.Helper()
	a1, _ := valueobject.ParseAccountID(s1)
	a1Copy, _ := valueobject.ParseAccountID(s1)
	a2, _ := valueobject.ParseAccountID(s2)
	assert.True(t, a1.Equals(a1Copy))
	assert.False(t, a1.Equals(a2))

	p1, _ := valueobject.ParsePostingID(s1)
	p1Copy, _ := valueobject.ParsePostingID(s1)
	p2, _ := valueobject.ParsePostingID(s2)
	assert.True(t, p1.Equals(p1Copy))
	assert.False(t, p1.Equals(p2))

	e1, _ := valueobject.ParseEntryID(s1)
	e1Copy, _ := valueobject.ParseEntryID(s1)
	e2, _ := valueobject.ParseEntryID(s2)
	assert.True(t, e1.Equals(e1Copy))
	assert.False(t, e1.Equals(e2))

	h1, _ := valueobject.ParseHoldID(s1)
	h1Copy, _ := valueobject.ParseHoldID(s1)
	h2, _ := valueobject.ParseHoldID(s2)
	assert.True(t, h1.Equals(h1Copy))
	assert.False(t, h1.Equals(h2))
}

func checkScopeIDEquivalence(t *testing.T, s1, s2 string) {
	t.Helper()
	t1, _ := valueobject.ParseTenantID(s1)
	t1Copy, _ := valueobject.ParseTenantID(s1)
	t2, _ := valueobject.ParseTenantID(s2)
	assert.True(t, t1.Equals(t1Copy))
	assert.False(t, t1.Equals(t2))

	l1, _ := valueobject.ParseLedgerID(s1)
	l1Copy, _ := valueobject.ParseLedgerID(s1)
	l2, _ := valueobject.ParseLedgerID(s2)
	assert.True(t, l1.Equals(l1Copy))
	assert.False(t, l1.Equals(l2))

	u1, _ := valueobject.ParseUserID(s1)
	u1Copy, _ := valueobject.ParseUserID(s1)
	u2, _ := valueobject.ParseUserID(s2)
	assert.True(t, u1.Equals(u1Copy))
	assert.False(t, u1.Equals(u2))

	j1, _ := valueobject.ParseJournalID(s1)
	j1Copy, _ := valueobject.ParseJournalID(s1)
	j2, _ := valueobject.ParseJournalID(s2)
	assert.True(t, j1.Equals(j1Copy))
	assert.False(t, j1.Equals(j2))

	d1, _ := valueobject.ParsePeriodID(s1)
	d1Copy, _ := valueobject.ParsePeriodID(s1)
	d2, _ := valueobject.ParsePeriodID(s2)
	assert.True(t, d1.Equals(d1Copy))
	assert.False(t, d1.Equals(d2))
}

func TestIDEqualsAllTypes(t *testing.T) {
	t.Parallel()

	gen := &seqIDs{}
	s1, s2 := gen.NewID(), gen.NewID()
	checkIDEquivalence(t, s1, s2)
	checkScopeIDEquivalence(t, s1, s2)

	_, err := valueobject.ParseHoldID("bad")
	assert.Error(t, err, "bad hold id must error")
}

func TestParseRejectsAlternateUUIDForms(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name string
		raw  string
	}

	testCases := []testCase{
		{name: "urn prefix form", raw: "urn:uuid:0193e5f0-1c2c-7a4e-b3f2-9c1e5a7b9c1e"},
		{name: "braced form", raw: "{0193e5f0-1c2c-7a4e-b3f2-9c1e5a7b9c1e}"},
		{name: "hex without hyphens", raw: "0193e5f01c2c7a4eb3f29c1e5a7b9c1e"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := valueobject.ParseAccountID(tc.raw)
			assert.Error(t, err, "ParseAccountID must reject non-canonical form")
		})
	}
}
