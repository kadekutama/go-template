package valueobject_test

import (
	"errors"
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
			expectedError:  errors.New("currency: unknown asset code \"ZZZ\""),
		},
	}

	for _, tc := range testCases {
		tc := tc
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
			expectedError: errors.New("currency: duplicate asset code \"USD\""),
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
			expectedError: errors.New("currency: asset code is required"),
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
			expectedError: errors.New("currency: negative exponent for \"XXX\""),
		},
	}

	for _, tc := range testCases {
		tc := tc
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
		{name: "Money", typ: reflect.TypeOf(valueobject.Money{})},
		{name: "AssetCode", typ: reflect.TypeOf(valueobject.AssetCode(""))},
		{name: "Registry", typ: reflect.TypeOf(valueobject.Registry{})},
		{name: "AccountID", typ: reflect.TypeOf(valueobject.AccountID(""))},
		{name: "PostingID", typ: reflect.TypeOf(valueobject.PostingID(""))},
		{name: "EntryID", typ: reflect.TypeOf(valueobject.EntryID(""))},
		{name: "HoldID", typ: reflect.TypeOf(valueobject.HoldID(""))},
		{name: "TenantID", typ: reflect.TypeOf(valueobject.TenantID(""))},
		{name: "LedgerID", typ: reflect.TypeOf(valueobject.LedgerID(""))},
		{name: "UserID", typ: reflect.TypeOf(valueobject.UserID(""))},
		{name: "JournalID", typ: reflect.TypeOf(valueobject.JournalID(""))},
		{name: "PeriodID", typ: reflect.TypeOf(valueobject.PeriodID(""))},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for i := 0; i < tc.typ.NumMethod(); i++ {
				assert.False(t, strings.HasPrefix(tc.typ.Method(i).Name, "Set"), "type has setter: "+tc.typ.Method(i).Name)
			}
			ptr := reflect.PointerTo(tc.typ)
			for i := 0; i < ptr.NumMethod(); i++ {
				assert.False(t, strings.HasPrefix(ptr.Method(i).Name, "Set"), "pointer type has setter: "+ptr.Method(i).Name)
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
		{name: "Money", typ: reflect.TypeOf(valueobject.Money{})},
		{name: "AssetInfo", typ: reflect.TypeOf(valueobject.AssetInfo{})},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for i := 0; i < tc.typ.NumField(); i++ {
				k := tc.typ.Field(i).Type.Kind()
				assert.NotEqual(t, reflect.Float32, k, "field uses float32: "+tc.typ.Field(i).Name)
				assert.NotEqual(t, reflect.Float64, k, "field uses float64: "+tc.typ.Field(i).Name)
			}
		})
	}
}
