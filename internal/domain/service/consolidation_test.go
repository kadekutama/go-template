package service_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func testEURUSDRate(at time.Time) valueobject.FxRate {
	pair, err := valueobject.NewFxPair("EUR", "USD")
	if err != nil {
		panic(err)
	}
	return valueobject.FxRate{
		ID:          "EURUSD-1",
		Pair:        pair,
		Numerator:   10850,
		Denominator: 10000,
		Source:      "ecb",
		QuotedAt:    at,
		TTL:         time.Hour,
	}
}

func TestConsolidateBalances(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)

	type testCase struct {
		name           string
		req            service.ConsolidationRequest
		expectedResult int64
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "single same-asset child",
			req: service.ConsolidationRequest{
				BaseAsset: "USD",
				Children: []service.ChildBalance{
					{TenantID: "t-a", AssetCode: "USD", AmountMinor: 10000},
				},
				At: at,
			},
			expectedResult: int64(10000),
			expectedError:  nil,
		},
		{
			// EUR 10000 @ 1.0850 half-up: (10000*10850 + 5000) / 10000 = 10850.
			name: "mixed assets convert",
			req: service.ConsolidationRequest{
				BaseAsset: "USD",
				Children: []service.ChildBalance{
					{TenantID: "t-a", AssetCode: "USD", AmountMinor: 10000},
					{TenantID: "t-b", AssetCode: "EUR", AmountMinor: 10000},
				},
				Rates: map[valueobject.AssetCode]valueobject.FxRate{
					"EUR": testEURUSDRate(at),
				},
				At: at,
			},
			expectedResult: int64(20850),
			expectedError:  nil,
		},
		{
			name: "mismatched rate pair rejected",
			req: func() service.ConsolidationRequest {
				pair, _ := valueobject.NewFxPair("USD", "EUR")
				rate := testEURUSDRate(at)
				rate.Pair = pair
				return service.ConsolidationRequest{
					BaseAsset: "USD",
					Children: []service.ChildBalance{
						{TenantID: "t-b", AssetCode: "EUR", AmountMinor: 10000},
					},
					Rates: map[valueobject.AssetCode]valueobject.FxRate{
						"EUR": rate,
					},
					At: at,
				}
			}(),
			expectedResult: int64(0),
			expectedError:  entity.NewError("FX_RATE_MISMATCH", "fx rate pair must convert child asset to base asset"),
		},
		{
			name: "stale rate rejected",
			req: func() service.ConsolidationRequest {
				rate := testEURUSDRate(at.Add(-3 * time.Hour))
				return service.ConsolidationRequest{
					BaseAsset: "USD",
					Children: []service.ChildBalance{
						{TenantID: "t-b", AssetCode: "EUR", AmountMinor: 10000},
					},
					Rates: map[valueobject.AssetCode]valueobject.FxRate{
						"EUR": rate,
					},
					At: at,
				}
			}(),
			expectedResult: int64(0),
			expectedError:  entity.NewError("FX_RATE_STALE", "fx rate is stale"),
		},
		{
			name: "fx conversion overflow rejected",
			req: service.ConsolidationRequest{
				BaseAsset: "USD",
				Children: []service.ChildBalance{
					{TenantID: "t-b", AssetCode: "EUR", AmountMinor: math.MaxInt64},
				},
				Rates: map[valueobject.AssetCode]valueobject.FxRate{
					"EUR": testEURUSDRate(at),
				},
				At: at,
			},
			expectedResult: int64(0),
			expectedError:  entity.NewError("FX_OVERFLOW", "fx conversion overflowed"),
		},
		{
			name: "empty children rejected",
			req: service.ConsolidationRequest{
				BaseAsset: "USD",
				At:        at,
			},
			expectedResult: int64(0),
			expectedError:  entity.NewError("CONSOLIDATION_EMPTY", "consolidation requires at least one child balance"),
		},
		{
			name: "missing rate rejected",
			req: service.ConsolidationRequest{
				BaseAsset: "USD",
				Children: []service.ChildBalance{
					{TenantID: "t-b", AssetCode: "EUR", AmountMinor: 10000},
				},
				At: at,
			},
			expectedResult: int64(0),
			expectedError:  entity.NewError("FX_RATE_MISSING", "consolidation requires an FX rate for child asset"),
		},
		{
			name: "negative amount rejected",
			req: service.ConsolidationRequest{
				BaseAsset: "USD",
				Children: []service.ChildBalance{
					{TenantID: "t-a", AssetCode: "USD", AmountMinor: -1},
				},
				At: at,
			},
			expectedResult: int64(0),
			expectedError:  entity.NewError("CONSOLIDATION_AMOUNT_INVALID", "consolidation amount must be non-negative"),
		},
		{
			name: "whitespace base asset rejected",
			req: service.ConsolidationRequest{
				BaseAsset: "   ",
				Children: []service.ChildBalance{
					{TenantID: "t-a", AssetCode: "USD", AmountMinor: 100},
				},
				At: at,
			},
			expectedResult: int64(0),
			expectedError:  entity.NewError("CONSOLIDATION_ASSET_REQUIRED", "consolidation requires a base asset"),
		},
		{
			name: "zero consolidation time rejected",
			req: service.ConsolidationRequest{
				BaseAsset: "USD",
				Children: []service.ChildBalance{
					{TenantID: "t-a", AssetCode: "USD", AmountMinor: 100},
				},
				At: time.Time{},
			},
			expectedResult: int64(0),
			expectedError:  entity.NewError("CONSOLIDATION_TIME_REQUIRED", "consolidation time is required"),
		},
		{
			name: "whitespace child tenant rejected",
			req: service.ConsolidationRequest{
				BaseAsset: "USD",
				Children: []service.ChildBalance{
					{TenantID: "   ", AssetCode: "USD", AmountMinor: 100},
				},
				At: at,
			},
			expectedResult: int64(0),
			expectedError:  entity.NewError("CONSOLIDATION_TENANT_REQUIRED", "consolidation child requires a tenant id"),
		},
		{
			name: "whitespace child asset rejected",
			req: service.ConsolidationRequest{
				BaseAsset: "USD",
				Children: []service.ChildBalance{
					{TenantID: "t-a", AssetCode: "   ", AmountMinor: 100},
				},
				At: at,
			},
			expectedResult: int64(0),
			expectedError:  entity.NewError("CONSOLIDATION_ASSET_REQUIRED", "consolidation child requires an asset code"),
		},
		{
			name: "overflow rejected",
			req: service.ConsolidationRequest{
				BaseAsset: "USD",
				Children: []service.ChildBalance{
					{TenantID: "t-a", AssetCode: "USD", AmountMinor: math.MaxInt64},
					{TenantID: "t-b", AssetCode: "USD", AmountMinor: 1},
				},
				At: at,
			},
			expectedResult: int64(0),
			expectedError:  entity.NewError("CONSOLIDATION_OVERFLOW", "consolidation total overflowed"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.ConsolidateBalances(tc.req)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestValidateTransferGrant(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		grants        map[string]bool
		childTenantID string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "explicit grant passes",
			grants:        map[string]bool{"t-child": true},
			childTenantID: "t-child",
			expectedError: nil,
		},
		{
			name:          "padded child tenant id passes",
			grants:        map[string]bool{"t-child": true},
			childTenantID: "  t-child  ",
			expectedError: nil,
		},
		{
			name:          "nil grants rejected",
			grants:        nil,
			childTenantID: "t-child",
			expectedError: entity.NewError("HIERARCHY_GRANT_REQUIRED", "cross-child transfer requires an explicit hierarchy grant"),
		},
		{
			name:          "missing grant rejected",
			grants:        map[string]bool{},
			childTenantID: "t-child",
			expectedError: entity.NewError("HIERARCHY_GRANT_REQUIRED", "cross-child transfer requires an explicit hierarchy grant"),
		},
		{
			name:          "false grant rejected",
			grants:        map[string]bool{"t-child": false},
			childTenantID: "t-child",
			expectedError: entity.NewError("HIERARCHY_GRANT_REQUIRED", "cross-child transfer requires an explicit hierarchy grant"),
		},
		{
			name:          "empty child id rejected",
			grants:        map[string]bool{"t-child": true},
			childTenantID: "  ",
			expectedError: entity.NewError("HIERARCHY_ID_REQUIRED", "hierarchy grant requires a child tenant id"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.ValidateTransferGrant(tc.grants, tc.childTenantID)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestHasTransferGrant(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		grants         map[string]bool
		childTenantID  string
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "grant present",
			grants:         map[string]bool{"t-a": true},
			childTenantID:  "t-a",
			expectedResult: true,
		},
		{
			name:           "padded child id matches grant",
			grants:         map[string]bool{"t-a": true},
			childTenantID:  "  t-a  ",
			expectedResult: true,
		},
		{
			name:           "nil grants returns false",
			grants:         nil,
			childTenantID:  "t-a",
			expectedResult: false,
		},
		{
			name:           "grant absent",
			grants:         map[string]bool{},
			childTenantID:  "t-a",
			expectedResult: false,
		},
		{
			name:           "grant false",
			grants:         map[string]bool{"t-a": false},
			childTenantID:  "t-a",
			expectedResult: false,
		},
		{
			name:           "empty child never granted",
			grants:         map[string]bool{"": true},
			childTenantID:  "",
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := service.HasTransferGrant(tc.grants, tc.childTenantID)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestValidateReadGrant(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		grants        map[string]bool
		childTenantID string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "explicit grant passes",
			grants:        map[string]bool{"t-child": true},
			childTenantID: "t-child",
			expectedError: nil,
		},
		{
			name:          "padded child tenant id passes",
			grants:        map[string]bool{"t-child": true},
			childTenantID: "  t-child  ",
			expectedError: nil,
		},
		{
			name:          "nil grants rejected",
			grants:        nil,
			childTenantID: "t-child",
			expectedError: entity.NewError("HIERARCHY_GRANT_REQUIRED", "parent read access requires an explicit hierarchy grant"),
		},
		{
			name:          "missing grant rejected",
			grants:        map[string]bool{},
			childTenantID: "t-child",
			expectedError: entity.NewError("HIERARCHY_GRANT_REQUIRED", "parent read access requires an explicit hierarchy grant"),
		},
		{
			name:          "false grant rejected",
			grants:        map[string]bool{"t-child": false},
			childTenantID: "t-child",
			expectedError: entity.NewError("HIERARCHY_GRANT_REQUIRED", "parent read access requires an explicit hierarchy grant"),
		},
		{
			name:          "empty child id rejected",
			grants:        map[string]bool{"t-child": true},
			childTenantID: "  ",
			expectedError: entity.NewError("HIERARCHY_ID_REQUIRED", "hierarchy grant requires a child tenant id"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.ValidateReadGrant(tc.grants, tc.childTenantID)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestHasReadGrant(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		grants         map[string]bool
		childTenantID  string
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "grant present",
			grants:         map[string]bool{"t-a": true},
			childTenantID:  "t-a",
			expectedResult: true,
		},
		{
			name:           "padded child id matches grant",
			grants:         map[string]bool{"t-a": true},
			childTenantID:  "  t-a  ",
			expectedResult: true,
		},
		{
			name:           "nil grants returns false",
			grants:         nil,
			childTenantID:  "t-a",
			expectedResult: false,
		},
		{
			name:           "grant absent",
			grants:         map[string]bool{},
			childTenantID:  "t-a",
			expectedResult: false,
		},
		{
			name:           "grant false",
			grants:         map[string]bool{"t-a": false},
			childTenantID:  "t-a",
			expectedResult: false,
		},
		{
			name:           "empty child never granted",
			grants:         map[string]bool{"": true},
			childTenantID:  "",
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := service.HasReadGrant(tc.grants, tc.childTenantID)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}
