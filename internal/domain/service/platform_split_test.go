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

func splitTestRate(base, quote valueobject.AssetCode, num, den int64, quotedAt time.Time) valueobject.FxRate {
	pair, err := valueobject.NewFxPair(base, quote)
	if err != nil {
		panic(err)
	}
	return valueobject.FxRate{
		ID:          string(base) + string(quote) + "-1",
		Pair:        pair,
		Numerator:   num,
		Denominator: den,
		Source:      "ecb",
		QuotedAt:    quotedAt,
		TTL:         time.Hour,
	}
}

func TestValidateSplit(t *testing.T) {
	t.Parallel()

	const (
		acctPlatform  = valueobject.AccountID("a-platform")
		acctConnected = valueobject.AccountID("a-connected")
		acctProcessor = valueobject.AccountID("a-processor")
		acctConnEUR   = valueobject.AccountID("a-connected-eur")
		acctFrozen    = valueobject.AccountID("a-frozen")
		acctClosed    = valueobject.AccountID("a-closed")
		currUSD       = valueobject.AssetCode("USD")
		currEUR       = valueobject.AssetCode("EUR")
	)

	at := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)

	accounts := map[valueobject.AccountID]entity.AccountData{
		acctPlatform:  {ID: acctPlatform, TenantID: "t-platform", LedgerID: "l-1", Number: "3000", Name: "platform", Class: valueobject.ClassRevenue, AssetCode: currUSD, Status: valueobject.StatusActive, Version: 1},
		acctConnected: {ID: acctConnected, TenantID: "t-conn", LedgerID: "l-1", Number: "3001", Name: "connected", Class: valueobject.ClassLiability, AssetCode: currUSD, Status: valueobject.StatusActive, Version: 1},
		acctProcessor: {ID: acctProcessor, TenantID: "t-platform", LedgerID: "l-1", Number: "3002", Name: "processor", Class: valueobject.ClassAsset, AssetCode: currUSD, Status: valueobject.StatusActive, Version: 1},
		acctConnEUR:   {ID: acctConnEUR, TenantID: "t-conn", LedgerID: "l-1", Number: "3003", Name: "connected-eur", Class: valueobject.ClassLiability, AssetCode: currEUR, Status: valueobject.StatusActive, Version: 1},
		acctFrozen:    {ID: acctFrozen, TenantID: "t-conn", LedgerID: "l-1", Number: "3004", Name: "frozen", Class: valueobject.ClassLiability, AssetCode: currUSD, Status: valueobject.StatusFrozen, Version: 1},
		acctClosed:    {ID: acctClosed, TenantID: "t-platform", LedgerID: "l-1", Number: "3005", Name: "closed", Class: valueobject.ClassRevenue, AssetCode: currUSD, Status: valueobject.StatusClosed, Version: 1},
	}

	baseReq := service.SplitRequest{
		PlatformAccount:  acctPlatform,
		ConnectedAccount: acctConnected,
		ProcessorAccount: acctProcessor,
		GrossMinor:       10000,
		FeeMinor:         290,
		AssetCode:        currUSD,
		Grants:           map[string]bool{"t-conn": true},
	}

	type testCase struct {
		name           string
		req            service.SplitRequest
		accounts       map[valueobject.AccountID]entity.AccountData
		expectedResult service.SplitLines
		expectedError  error
	}

	testCases := []testCase{
		{
			name:     "same-currency capture carves fee at charge time",
			req:      baseReq,
			accounts: accounts,
			expectedResult: service.SplitLines{
				DebitAccount:        acctProcessor,
				DebitAmountMinor:    10000,
				ConnectedAccount:    acctConnected,
				NetMinor:            9710,
				PlatformAccount:     acctPlatform,
				FeeMinor:            290,
				AssetCode:           currUSD,
				SettlementAsset:     currUSD,
				ConvertedGrossMinor: 10000,
				ConvertedNetMinor:   9710,
				ConvertedFeeMinor:   290,
				GainLossMinor:       0,
				FXRateID:            "",
			},
			expectedError: nil,
		},
		{
			name: "zero fee passes net equal to gross",
			req: func() service.SplitRequest {
				r := baseReq
				r.FeeMinor = 0
				return r
			}(),
			accounts: accounts,
			expectedResult: service.SplitLines{
				DebitAccount:        acctProcessor,
				DebitAmountMinor:    10000,
				ConnectedAccount:    acctConnected,
				NetMinor:            10000,
				PlatformAccount:     acctPlatform,
				FeeMinor:            0,
				AssetCode:           currUSD,
				SettlementAsset:     currUSD,
				ConvertedGrossMinor: 10000,
				ConvertedNetMinor:   10000,
				ConvertedFeeMinor:   0,
				GainLossMinor:       0,
				FXRateID:            "",
			},
			expectedError: nil,
		},
		{
			name: "fee equal to gross passes zero net",
			req: func() service.SplitRequest {
				r := baseReq
				r.FeeMinor = 10000
				return r
			}(),
			accounts: accounts,
			expectedResult: service.SplitLines{
				DebitAccount:        acctProcessor,
				DebitAmountMinor:    10000,
				ConnectedAccount:    acctConnected,
				NetMinor:            0,
				PlatformAccount:     acctPlatform,
				FeeMinor:            10000,
				AssetCode:           currUSD,
				SettlementAsset:     currUSD,
				ConvertedGrossMinor: 10000,
				ConvertedNetMinor:   0,
				ConvertedFeeMinor:   10000,
				GainLossMinor:       0,
				FXRateID:            "",
			},
			expectedError: nil,
		},
		{
			name: "zero gross rejected",
			req: func() service.SplitRequest {
				r := baseReq
				r.GrossMinor = 0
				return r
			}(),
			accounts:       accounts,
			expectedResult: service.SplitLines{},
			expectedError:  entity.NewError("INVALID_SPLIT_AMOUNT", "split gross amount must be positive"),
		},
		{
			name: "negative gross rejected",
			req: func() service.SplitRequest {
				r := baseReq
				r.GrossMinor = -100
				return r
			}(),
			accounts:       accounts,
			expectedResult: service.SplitLines{},
			expectedError:  entity.NewError("INVALID_SPLIT_AMOUNT", "split gross amount must be positive"),
		},
		{
			name: "fee exceeding gross rejected",
			req: func() service.SplitRequest {
				r := baseReq
				r.FeeMinor = 10001
				return r
			}(),
			accounts:       accounts,
			expectedResult: service.SplitLines{},
			expectedError:  entity.NewError("SPLIT_MISMATCH", "split fee must satisfy 0 <= fee <= gross"),
		},
		{
			name: "negative fee rejected",
			req: func() service.SplitRequest {
				r := baseReq
				r.FeeMinor = -1
				return r
			}(),
			accounts:       accounts,
			expectedResult: service.SplitLines{},
			expectedError:  entity.NewError("SPLIT_MISMATCH", "split fee must satisfy 0 <= fee <= gross"),
		},
		{
			name: "empty asset rejected",
			req: func() service.SplitRequest {
				r := baseReq
				r.AssetCode = ""
				return r
			}(),
			accounts:       accounts,
			expectedResult: service.SplitLines{},
			expectedError:  entity.NewError("SPLIT_ASSET_REQUIRED", "split requires an asset code"),
		},
		{
			name: "whitespace asset rejected",
			req: func() service.SplitRequest {
				r := baseReq
				r.AssetCode = "   "
				return r
			}(),
			accounts:       accounts,
			expectedResult: service.SplitLines{},
			expectedError:  entity.NewError("SPLIT_ASSET_REQUIRED", "split requires an asset code"),
		},
		{
			name: "missing platform account rejected",
			req: func() service.SplitRequest {
				r := baseReq
				r.PlatformAccount = ""
				return r
			}(),
			accounts:       accounts,
			expectedResult: service.SplitLines{},
			expectedError:  entity.NewError("SPLIT_ACCOUNT_REQUIRED", "split requires platform, connected, and processor accounts"),
		},
		{
			name: "duplicate posting accounts rejected",
			req: func() service.SplitRequest {
				r := baseReq
				r.PlatformAccount = acctConnected
				return r
			}(),
			accounts:       accounts,
			expectedResult: service.SplitLines{},
			expectedError:  entity.NewError("SPLIT_ACCOUNT_INVALID", "split posting accounts must be distinct"),
		},
		{
			name:           "unknown platform account rejected",
			req:            baseReq,
			accounts:       map[valueobject.AccountID]entity.AccountData{},
			expectedResult: service.SplitLines{},
			expectedError:  entity.NewError("PLATFORM_ACCOUNT_NOT_FOUND", "platform account is unknown"),
		},
		{
			name: "unknown connected account rejected",
			req: func() service.SplitRequest {
				r := baseReq
				r.ConnectedAccount = valueobject.AccountID("a-ghost")
				return r
			}(),
			accounts:       accounts,
			expectedResult: service.SplitLines{},
			expectedError:  entity.NewError("CONNECTED_ACCOUNT_NOT_FOUND", "connected account is unknown"),
		},
		{
			name: "unknown processor account rejected",
			req: func() service.SplitRequest {
				r := baseReq
				r.ProcessorAccount = valueobject.AccountID("a-ghost")
				return r
			}(),
			accounts:       accounts,
			expectedResult: service.SplitLines{},
			expectedError:  entity.NewError("PROCESSOR_ACCOUNT_NOT_FOUND", "processor account is unknown"),
		},
		{
			name: "frozen connected account rejected",
			req: func() service.SplitRequest {
				r := baseReq
				r.ConnectedAccount = acctFrozen
				r.Grants = map[string]bool{"t-conn": true}
				return r
			}(),
			accounts:       accounts,
			expectedResult: service.SplitLines{},
			expectedError:  entity.NewError("ACCOUNT_FROZEN", "account is frozen"),
		},
		{
			name: "closed platform account rejected",
			req: func() service.SplitRequest {
				r := baseReq
				r.PlatformAccount = acctClosed
				return r
			}(),
			accounts:       accounts,
			expectedResult: service.SplitLines{},
			expectedError:  entity.NewError("ACCOUNT_CLOSED", "account is closed"),
		},
		{
			name: "missing hierarchy grant rejected",
			req: func() service.SplitRequest {
				r := baseReq
				r.Grants = map[string]bool{}
				return r
			}(),
			accounts:       accounts,
			expectedResult: service.SplitLines{},
			expectedError:  entity.NewError("HIERARCHY_GRANT_REQUIRED", "cross-child transfer requires an explicit hierarchy grant"),
		},
		{
			name: "grant for another tenant rejected",
			req: func() service.SplitRequest {
				r := baseReq
				r.Grants = map[string]bool{"t-other": true}
				return r
			}(),
			accounts:       accounts,
			expectedResult: service.SplitLines{},
			expectedError:  entity.NewError("HIERARCHY_GRANT_REQUIRED", "cross-child transfer requires an explicit hierarchy grant"),
		},
		{
			name: "processor leg outside charge asset rejected",
			req: func() service.SplitRequest {
				r := baseReq
				r.ProcessorAccount = acctConnEUR
				return r
			}(),
			accounts:       accounts,
			expectedResult: service.SplitLines{},
			expectedError:  entity.NewError("CURRENCY_MISMATCH", "split processor and platform legs must post in the charge asset"),
		},
		{
			name: "cross-currency without rate rejected",
			req: func() service.SplitRequest {
				r := baseReq
				r.ConnectedAccount = acctConnEUR
				return r
			}(),
			accounts:       accounts,
			expectedResult: service.SplitLines{},
			expectedError:  entity.NewError("FX_RATE_MISSING", "split requires an FX rate for the settlement asset"),
		},
		{
			name: "cross-currency mismatched pair rejected",
			req: func() service.SplitRequest {
				r := baseReq
				r.ConnectedAccount = acctConnEUR
				r.FXRate = splitTestRate(currEUR, currUSD, 10850, 10000, at)
				r.FXRatePresent = true
				r.At = at
				return r
			}(),
			accounts:       accounts,
			expectedResult: service.SplitLines{},
			expectedError:  entity.NewError("FX_RATE_MISMATCH", "fx rate pair must convert charge asset to settlement asset"),
		},
		{
			name: "cross-currency stale rate rejected",
			req: func() service.SplitRequest {
				r := baseReq
				r.ConnectedAccount = acctConnEUR
				r.FXRate = splitTestRate(currUSD, currEUR, 9200, 10000, at.Add(-3*time.Hour))
				r.FXRatePresent = true
				r.At = at
				return r
			}(),
			accounts:       accounts,
			expectedResult: service.SplitLines{},
			expectedError:  entity.NewError("FX_RATE_STALE", "fx rate is stale"),
		},
		{
			name: "cross-currency capture converts every leg",
			req: func() service.SplitRequest {
				r := baseReq
				r.ConnectedAccount = acctConnEUR
				r.FXRate = splitTestRate(currUSD, currEUR, 9200, 10000, at)
				r.FXRatePresent = true
				r.At = at
				return r
			}(),
			accounts: accounts,
			expectedResult: service.SplitLines{
				DebitAccount:        acctProcessor,
				DebitAmountMinor:    10000,
				ConnectedAccount:    acctConnEUR,
				NetMinor:            9710,
				PlatformAccount:     acctPlatform,
				FeeMinor:            290,
				AssetCode:           currUSD,
				SettlementAsset:     currEUR,
				ConvertedGrossMinor: 9200,
				ConvertedNetMinor:   8933,
				ConvertedFeeMinor:   267,
				GainLossMinor:       0,
				FXRateID:            "USDEUR-1",
			},
			expectedError: nil,
		},
		{
			name: "cross-currency rounding drift posts explicitly",
			req: func() service.SplitRequest {
				r := baseReq
				r.ConnectedAccount = acctConnEUR
				r.FXRate = splitTestRate(currUSD, currEUR, 1, 3, at)
				r.FXRatePresent = true
				r.At = at
				return r
			}(),
			accounts: accounts,
			expectedResult: service.SplitLines{
				DebitAccount:        acctProcessor,
				DebitAmountMinor:    10000,
				ConnectedAccount:    acctConnEUR,
				NetMinor:            9710,
				PlatformAccount:     acctPlatform,
				FeeMinor:            290,
				AssetCode:           currUSD,
				SettlementAsset:     currEUR,
				ConvertedGrossMinor: 3333,
				ConvertedNetMinor:   3237,
				ConvertedFeeMinor:   97,
				GainLossMinor:       -1,
				FXRateID:            "USDEUR-1",
			},
			expectedError: nil,
		},
		{
			name: "maximum gross converts without wrapping",
			req: func() service.SplitRequest {
				r := baseReq
				r.GrossMinor = math.MaxInt64
				r.FeeMinor = 290
				return r
			}(),
			accounts: accounts,
			expectedResult: service.SplitLines{
				DebitAccount:        acctProcessor,
				DebitAmountMinor:    math.MaxInt64,
				ConnectedAccount:    acctConnected,
				NetMinor:            math.MaxInt64 - 290,
				PlatformAccount:     acctPlatform,
				FeeMinor:            290,
				AssetCode:           currUSD,
				SettlementAsset:     currUSD,
				ConvertedGrossMinor: math.MaxInt64,
				ConvertedNetMinor:   math.MaxInt64 - 290,
				ConvertedFeeMinor:   290,
				GainLossMinor:       0,
				FXRateID:            "",
			},
			expectedError: nil,
		},
		{
			name: "maximum gross cross-currency overflows conversion",
			req: func() service.SplitRequest {
				r := baseReq
				r.ConnectedAccount = acctConnEUR
				r.GrossMinor = math.MaxInt64
				r.FeeMinor = 0
				r.FXRate = splitTestRate(currUSD, currEUR, 9200, 10000, at)
				r.FXRatePresent = true
				r.At = at
				return r
			}(),
			accounts:       accounts,
			expectedResult: service.SplitLines{},
			expectedError:  entity.NewError("FX_OVERFLOW", "fx conversion overflowed"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.ValidateSplit(tc.req, tc.accounts)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestValidateSplitRefund(t *testing.T) {
	t.Parallel()

	const (
		acctPlatform  = valueobject.AccountID("a-platform")
		acctConnected = valueobject.AccountID("a-connected")
		acctProcessor = valueobject.AccountID("a-processor")
		acctConnEUR   = valueobject.AccountID("a-connected-eur")
		currUSD       = valueobject.AssetCode("USD")
		currEUR       = valueobject.AssetCode("EUR")
	)

	at := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)

	baseOriginal := service.SplitLines{
		DebitAccount:        acctProcessor,
		DebitAmountMinor:    10000,
		ConnectedAccount:    acctConnected,
		NetMinor:            9710,
		PlatformAccount:     acctPlatform,
		FeeMinor:            290,
		AssetCode:           currUSD,
		SettlementAsset:     currUSD,
		ConvertedGrossMinor: 10000,
		ConvertedNetMinor:   9710,
		ConvertedFeeMinor:   290,
		GainLossMinor:       0,
		FXRateID:            "",
	}

	baseFXOriginal := service.SplitLines{
		DebitAccount:        acctProcessor,
		DebitAmountMinor:    10000,
		ConnectedAccount:    acctConnEUR,
		NetMinor:            9710,
		PlatformAccount:     acctPlatform,
		FeeMinor:            290,
		AssetCode:           currUSD,
		SettlementAsset:     currEUR,
		ConvertedGrossMinor: 9200,
		ConvertedNetMinor:   8933,
		ConvertedFeeMinor:   267,
		GainLossMinor:       0,
		FXRateID:            "USDEUR-1",
	}

	type testCase struct {
		name           string
		req            service.SplitRefundRequest
		expectedResult service.SplitRefundLines
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "full refund reverses all three legs",
			req: service.SplitRefundRequest{
				Original:         baseOriginal,
				RefundGrossMinor: 10000,
				FeeRefundMinor:   290,
			},
			expectedResult: service.SplitRefundLines{
				ProcessorAccount:          acctProcessor,
				MerchantAccount:           acctConnected,
				PlatformAccount:           acctPlatform,
				RefundGrossMinor:          10000,
				RefundNetMinor:            9710,
				FeeRefundMinor:            290,
				AssetCode:                 currUSD,
				SettlementAsset:           currUSD,
				ConvertedRefundGrossMinor: 10000,
				ConvertedRefundNetMinor:   9710,
				ConvertedFeeRefundMinor:   290,
				GainLossMinor:             0,
			},
			expectedError: nil,
		},
		{
			name: "partial refund with explicit fee policy",
			req: service.SplitRefundRequest{
				Original:         baseOriginal,
				RefundGrossMinor: 4000,
				FeeRefundMinor:   100,
			},
			expectedResult: service.SplitRefundLines{
				ProcessorAccount:          acctProcessor,
				MerchantAccount:           acctConnected,
				PlatformAccount:           acctPlatform,
				RefundGrossMinor:          4000,
				RefundNetMinor:            3900,
				FeeRefundMinor:            100,
				AssetCode:                 currUSD,
				SettlementAsset:           currUSD,
				ConvertedRefundGrossMinor: 4000,
				ConvertedRefundNetMinor:   3900,
				ConvertedFeeRefundMinor:   100,
				GainLossMinor:             0,
			},
			expectedError: nil,
		},
		{
			name: "refund keeping the full fee",
			req: service.SplitRefundRequest{
				Original:         baseOriginal,
				RefundGrossMinor: 4000,
				FeeRefundMinor:   0,
			},
			expectedResult: service.SplitRefundLines{
				ProcessorAccount:          acctProcessor,
				MerchantAccount:           acctConnected,
				PlatformAccount:           acctPlatform,
				RefundGrossMinor:          4000,
				RefundNetMinor:            4000,
				FeeRefundMinor:            0,
				AssetCode:                 currUSD,
				SettlementAsset:           currUSD,
				ConvertedRefundGrossMinor: 4000,
				ConvertedRefundNetMinor:   4000,
				ConvertedFeeRefundMinor:   0,
				GainLossMinor:             0,
			},
			expectedError: nil,
		},
		{
			name: "zero refund gross rejected",
			req: service.SplitRefundRequest{
				Original:         baseOriginal,
				RefundGrossMinor: 0,
				FeeRefundMinor:   0,
			},
			expectedResult: service.SplitRefundLines{},
			expectedError:  entity.NewError("INVALID_REFUND_AMOUNT", "split refund gross must be positive"),
		},
		{
			name: "negative refund gross rejected",
			req: service.SplitRefundRequest{
				Original:         baseOriginal,
				RefundGrossMinor: -50,
				FeeRefundMinor:   0,
			},
			expectedResult: service.SplitRefundLines{},
			expectedError:  entity.NewError("INVALID_REFUND_AMOUNT", "split refund gross must be positive"),
		},
		{
			name: "refund exceeding original rejected",
			req: service.SplitRefundRequest{
				Original:         baseOriginal,
				RefundGrossMinor: 10001,
				FeeRefundMinor:   290,
			},
			expectedResult: service.SplitRefundLines{},
			expectedError:  entity.NewError("REFUND_EXCEEDS_ORIGINAL", "split refund exceeds the original gross"),
		},
		{
			name: "fee refund exceeding refund gross rejected",
			req: service.SplitRefundRequest{
				Original:         baseOriginal,
				RefundGrossMinor: 4000,
				FeeRefundMinor:   4001,
			},
			expectedResult: service.SplitRefundLines{},
			expectedError:  entity.NewError("FEE_REFUND_INVALID", "split fee refund must satisfy 0 <= fee <= min(refund gross, original fee)"),
		},
		{
			name: "fee refund exceeding original fee rejected",
			req: service.SplitRefundRequest{
				Original:         baseOriginal,
				RefundGrossMinor: 10000,
				FeeRefundMinor:   291,
			},
			expectedResult: service.SplitRefundLines{},
			expectedError:  entity.NewError("FEE_REFUND_INVALID", "split fee refund must satisfy 0 <= fee <= min(refund gross, original fee)"),
		},
		{
			name: "negative fee refund rejected",
			req: service.SplitRefundRequest{
				Original:         baseOriginal,
				RefundGrossMinor: 4000,
				FeeRefundMinor:   -1,
			},
			expectedResult: service.SplitRefundLines{},
			expectedError:  entity.NewError("FEE_REFUND_INVALID", "split fee refund must satisfy 0 <= fee <= min(refund gross, original fee)"),
		},
		{
			name: "inconsistent original rejected",
			req: func() service.SplitRefundRequest {
				orig := baseOriginal
				orig.NetMinor = 9000
				return service.SplitRefundRequest{
					Original:         orig,
					RefundGrossMinor: 4000,
					FeeRefundMinor:   100,
				}
			}(),
			expectedResult: service.SplitRefundLines{},
			expectedError:  entity.NewError("SPLIT_MISMATCH", "original split lines are inconsistent"),
		},
		{
			name: "inconsistent settlement legs rejected",
			req: func() service.SplitRefundRequest {
				orig := baseFXOriginal
				orig.GainLossMinor = 5
				return service.SplitRefundRequest{
					Original:         orig,
					RefundGrossMinor: 4000,
					FeeRefundMinor:   100,
				}
			}(),
			expectedResult: service.SplitRefundLines{},
			expectedError:  entity.NewError("SPLIT_MISMATCH", "original split settlement legs are inconsistent"),
		},
		{
			name: "cross-currency refund reconverts every leg",
			req: service.SplitRefundRequest{
				Original:         baseFXOriginal,
				RefundGrossMinor: 4000,
				FeeRefundMinor:   100,
				FXRate:           splitTestRate(currUSD, currEUR, 9200, 10000, at),
				FXRatePresent:    true,
				At:               at,
			},
			expectedResult: service.SplitRefundLines{
				ProcessorAccount:          acctProcessor,
				MerchantAccount:           acctConnEUR,
				PlatformAccount:           acctPlatform,
				RefundGrossMinor:          4000,
				RefundNetMinor:            3900,
				FeeRefundMinor:            100,
				AssetCode:                 currUSD,
				SettlementAsset:           currEUR,
				ConvertedRefundGrossMinor: 3680,
				ConvertedRefundNetMinor:   3588,
				ConvertedFeeRefundMinor:   92,
				GainLossMinor:             0,
			},
			expectedError: nil,
		},
		{
			name: "cross-currency refund without rate rejected",
			req: service.SplitRefundRequest{
				Original:         baseFXOriginal,
				RefundGrossMinor: 4000,
				FeeRefundMinor:   100,
			},
			expectedResult: service.SplitRefundLines{},
			expectedError:  entity.NewError("FX_RATE_MISSING", "split refund requires an FX rate for the settlement asset"),
		},
		{
			name: "cross-currency refund mismatched pair rejected",
			req: service.SplitRefundRequest{
				Original:         baseFXOriginal,
				RefundGrossMinor: 4000,
				FeeRefundMinor:   100,
				FXRate:           splitTestRate(currEUR, currUSD, 10850, 10000, at),
				FXRatePresent:    true,
				At:               at,
			},
			expectedResult: service.SplitRefundLines{},
			expectedError:  entity.NewError("FX_RATE_MISMATCH", "fx rate pair must convert charge asset to settlement asset"),
		},
		{
			name: "cross-currency refund stale rate rejected",
			req: service.SplitRefundRequest{
				Original:         baseFXOriginal,
				RefundGrossMinor: 4000,
				FeeRefundMinor:   100,
				FXRate:           splitTestRate(currUSD, currEUR, 9200, 10000, at.Add(-3*time.Hour)),
				FXRatePresent:    true,
				At:               at,
			},
			expectedResult: service.SplitRefundLines{},
			expectedError:  entity.NewError("FX_RATE_STALE", "fx rate is stale"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.ValidateSplitRefund(tc.req)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
