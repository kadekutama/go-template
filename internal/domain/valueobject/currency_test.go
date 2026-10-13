package valueobject_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

const (
	testUSD     = "USD"
	testEUR     = "EUR"
	kindAccount = "account"
	kindPosting = "posting"
	kindEntry   = "entry"
	kindHold    = "hold"
	kindTenant  = "tenant"
	kindLedger  = "ledger"
	kindUser    = "user"
	kindJournal = "journal"
	kindPeriod  = "period"
)

func TestRegistryLookup(t *testing.T) {
	t.Parallel()

	reg, err := valueobject.NewRegistry(
		valueobject.AssetInfo{Code: testUSD, Exponent: 2, Kind: valueobject.AssetKindFiat},
		valueobject.AssetInfo{Code: testEUR, Exponent: 2, Kind: valueobject.AssetKindFiat},
	)
	assert.NoError(t, err)

	type testCase struct {
		name           string
		code           valueobject.AssetCode
		expectedResult valueobject.AssetInfo
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "existing USD lookup",
			code: testUSD,
			expectedResult: valueobject.AssetInfo{
				Code:     testUSD,
				Exponent: 2,
				Kind:     valueobject.AssetKindFiat,
			},
			expectedError: nil,
		},
		{
			name: "existing EUR lookup",
			code: testEUR,
			expectedResult: valueobject.AssetInfo{
				Code:     testEUR,
				Exponent: 2,
				Kind:     valueobject.AssetKindFiat,
			},
			expectedError: nil,
		},
		{
			name:           "unknown currency code",
			code:           "ZZZ",
			expectedResult: valueobject.AssetInfo{},
			expectedError:  valueobject.ErrCurrencyUnknownAssetCodeZZZ,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, lookupErr := reg.Lookup(tc.code)
			assert.Equal(t, tc.expectedResult, got)

			if tc.expectedError != nil {
				assert.Equal(t, tc.expectedError.Error(), lookupErr.Error())
			} else {
				assert.NoError(t, lookupErr)
			}
		})
	}
}

func TestRegistryRegister(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		initialAssets []valueobject.AssetInfo
		info          valueobject.AssetInfo
		expectedError error
	}

	testCases := []testCase{
		{
			name: "valid new asset registration",
			initialAssets: []valueobject.AssetInfo{
				{Code: testUSD, Exponent: 2, Kind: valueobject.AssetKindFiat},
			},
			info: valueobject.AssetInfo{
				Code:     "GBP",
				Exponent: 2,
				Kind:     valueobject.AssetKindFiat,
			},
			expectedError: nil,
		},
		{
			name: "duplicate asset registration",
			initialAssets: []valueobject.AssetInfo{
				{Code: testUSD, Exponent: 2, Kind: valueobject.AssetKindFiat},
			},
			info: valueobject.AssetInfo{
				Code:     testUSD,
				Exponent: 2,
				Kind:     valueobject.AssetKindFiat,
			},
			expectedError: valueobject.ErrCurrencyDuplicateAssetCodeUSD,
		},
		{
			name: "empty code",
			initialAssets: []valueobject.AssetInfo{
				{Code: testUSD, Exponent: 2, Kind: valueobject.AssetKindFiat},
			},
			info: valueobject.AssetInfo{
				Code:     "",
				Exponent: 2,
				Kind:     valueobject.AssetKindFiat,
			},
			expectedError: valueobject.ErrCurrencyAssetCodeIsRequired,
		},
		{
			name: "negative exponent",
			initialAssets: []valueobject.AssetInfo{
				{Code: testUSD, Exponent: 2, Kind: valueobject.AssetKindFiat},
			},
			info: valueobject.AssetInfo{
				Code:     "XXX",
				Exponent: -1,
				Kind:     valueobject.AssetKindFiat,
			},
			expectedError: valueobject.ErrCurrencyNegativeExponentForXXX,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			reg, err := valueobject.NewRegistry(tc.initialAssets...)
			assert.NoError(t, err)

			regErr := reg.Register(tc.info)
			if tc.expectedError != nil {
				assert.Equal(t, tc.expectedError.Error(), regErr.Error())
			} else {
				assert.NoError(t, regErr)
			}
		})
	}
}

func TestNoSettersOnValueObjects(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name string
		typ  reflect.Type
	}

	testCases := []testCase{
		{name: "Money", typ: reflect.TypeFor[valueobject.Money]()},
		{name: "AssetCode", typ: reflect.TypeFor[valueobject.AssetCode]()},
		{name: "Registry", typ: reflect.TypeFor[valueobject.Registry]()},
		{name: "AccountID", typ: reflect.TypeFor[valueobject.AccountID]()},
		{name: "PostingID", typ: reflect.TypeFor[valueobject.PostingID]()},
		{name: "EntryID", typ: reflect.TypeFor[valueobject.EntryID]()},
		{name: "HoldID", typ: reflect.TypeFor[valueobject.HoldID]()},
		{name: "TenantID", typ: reflect.TypeFor[valueobject.TenantID]()},
		{name: "LedgerID", typ: reflect.TypeFor[valueobject.LedgerID]()},
		{name: "UserID", typ: reflect.TypeFor[valueobject.UserID]()},
		{name: "JournalID", typ: reflect.TypeFor[valueobject.JournalID]()},
		{name: "PeriodID", typ: reflect.TypeFor[valueobject.PeriodID]()},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for m := range tc.typ.Methods() {
				assert.False(t, strings.HasPrefix(m.Name, "Set"), "type has setter: "+m.Name)
			}

			ptr := reflect.PointerTo(tc.typ)
			for m := range ptr.Methods() {
				assert.False(t, strings.HasPrefix(m.Name, "Set"), "pointer type has setter: "+m.Name)
			}
		})
	}
}

func TestNoFloatsInValueObjects(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name string
		typ  reflect.Type
	}

	testCases := []testCase{
		{name: "Money", typ: reflect.TypeFor[valueobject.Money]()},
		{name: "AssetInfo", typ: reflect.TypeFor[valueobject.AssetInfo]()},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for f := range tc.typ.Fields() {
				k := f.Type.Kind()
				assert.NotEqual(t, reflect.Float32, k, "field uses float32: "+f.Name)
				assert.NotEqual(t, reflect.Float64, k, "field uses float64: "+f.Name)
			}
		})
	}
}
