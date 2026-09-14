package service_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestValidateRefund(t *testing.T) {
	t.Parallel()

	const (
		acctMerchant = valueobject.AccountID("m-1")
		acctRefunds  = valueobject.AccountID("r-1")
		acctCash     = valueobject.AccountID("c-1")
		acctFrozen   = valueobject.AccountID("frz-1")
		currUSD      = valueobject.AssetCode("USD")
		currEUR      = valueobject.AssetCode("EUR")
	)

	accounts := map[valueobject.AccountID]entity.AccountData{
		acctMerchant: {ID: acctMerchant, TenantID: "t-1", LedgerID: "l-1", Number: "2000", Name: "merchant", Class: valueobject.ClassLiability, AssetCode: currUSD, Status: valueobject.StatusActive, Version: 1},
		acctRefunds:  {ID: acctRefunds, TenantID: "t-1", LedgerID: "l-1", Number: "2100", Name: "refunds", Class: valueobject.ClassLiability, AssetCode: currUSD, Status: valueobject.StatusActive, Version: 1},
		acctCash:     {ID: acctCash, TenantID: "t-1", LedgerID: "l-1", Number: "1000", Name: "cash", Class: valueobject.ClassAsset, AssetCode: currUSD, Status: valueobject.StatusActive, Version: 1},
		acctFrozen:   {ID: acctFrozen, TenantID: "t-1", LedgerID: "l-1", Number: "1001", Name: "frozen", Class: valueobject.ClassAsset, AssetCode: currUSD, Status: valueobject.StatusFrozen, Version: 1},
	}

	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	baseReq := service.RefundRequest{
		OriginalPostingID:   "p-orig",
		OriginalAmountMinor: 10000,
		PriorRefundedMinor:  3000,
		AmountMinor:         7000,
		OriginalAt:          now.Add(-24 * time.Hour),
		Now:                 now,
		WindowDays:          90,
		MerchantPayable:     acctMerchant,
		RefundsPayable:      acctRefunds,
		CashAccount:         acctCash,
		AssetCode:           currUSD,
	}

	okLines := service.RefundLines{
		OriginalPostingID: "p-orig",
		AcceptanceDebit:   acctMerchant,
		AcceptanceCredit:  acctRefunds,
		SettlementDebit:   acctRefunds,
		SettlementCredit:  acctCash,
		AmountMinor:       7000,
		AssetCode:         currUSD,
	}

	type testCase struct {
		name           string
		req            service.RefundRequest
		originalExists bool
		accounts       map[valueobject.AccountID]entity.AccountData
		expectedResult service.RefundLines
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "ok valid refund",
			req:            baseReq,
			originalExists: true,
			accounts:       accounts,
			expectedResult: okLines,
			expectedError:  nil,
		},
		{
			name: "exceeds original remaining",
			req: func() service.RefundRequest {
				r := baseReq
				r.AmountMinor = 7001
				return r
			}(),
			originalExists: true,
			accounts:       accounts,
			expectedResult: service.RefundLines{},
			expectedError:  entity.NewError("REFUND_EXCEEDS_ORIGINAL", "refund exceeds remaining refundable amount"),
		},
		{
			name: "priors over original",
			req: func() service.RefundRequest {
				r := baseReq
				r.PriorRefundedMinor = 20000
				return r
			}(),
			originalExists: true,
			accounts:       accounts,
			expectedResult: service.RefundLines{},
			expectedError:  entity.NewError("REFUND_EXCEEDS_ORIGINAL", "prior refunds exceed original amount"),
		},
		{
			name: "negative prior refund",
			req: func() service.RefundRequest {
				r := baseReq
				r.PriorRefundedMinor = -1
				return r
			}(),
			originalExists: true,
			accounts:       accounts,
			expectedResult: service.RefundLines{},
			expectedError:  entity.NewError("REFUND_EXCEEDS_ORIGINAL", "prior refunds exceed original amount"),
		},
		{
			name: "zero refund amount",
			req: func() service.RefundRequest {
				r := baseReq
				r.AmountMinor = 0
				return r
			}(),
			originalExists: true,
			accounts:       accounts,
			expectedResult: service.RefundLines{},
			expectedError:  entity.NewError("INVALID_REFUND_AMOUNT", "refund amount must be positive"),
		},
		{
			name: "zero original amount",
			req: func() service.RefundRequest {
				r := baseReq
				r.OriginalAmountMinor = 0
				return r
			}(),
			originalExists: true,
			accounts:       accounts,
			expectedResult: service.RefundLines{},
			expectedError:  entity.NewError("ORIGINAL_AMOUNT_INVALID", "original amount must be positive"),
		},
		{
			name: "window expired",
			req: func() service.RefundRequest {
				r := baseReq
				r.OriginalAt = r.Now.Add(-91 * 24 * time.Hour)
				return r
			}(),
			originalExists: true,
			accounts:       accounts,
			expectedResult: service.RefundLines{},
			expectedError:  entity.NewError("REFUND_WINDOW_EXPIRED", "refund window has expired"),
		},
		{
			name: "default window allows 89 days",
			req: func() service.RefundRequest {
				r := baseReq
				r.WindowDays = 0
				r.OriginalAt = r.Now.Add(-89 * 24 * time.Hour)
				return r
			}(),
			originalExists: true,
			accounts:       accounts,
			expectedResult: okLines,
			expectedError:  nil,
		},
		{
			name:           "missing original payment",
			req:            baseReq,
			originalExists: false,
			accounts:       accounts,
			expectedResult: service.RefundLines{},
			expectedError:  entity.NewError("ORIGINAL_NOT_FOUND", "original payment does not exist"),
		},
		{
			name: "negative fee policy",
			req: func() service.RefundRequest {
				r := baseReq
				r.FeePolicy.ProcessorFeeRefundMinor = -1
				return r
			}(),
			originalExists: true,
			accounts:       accounts,
			expectedResult: service.RefundLines{},
			expectedError:  entity.NewError("FEE_POLICY_INVALID", "fee refunds must be explicit non-negative amounts"),
		},
		{
			name: "identical refund accounts",
			req: func() service.RefundRequest {
				r := baseReq
				r.RefundsPayable = r.MerchantPayable
				return r
			}(),
			originalExists: true,
			accounts:       accounts,
			expectedResult: service.RefundLines{},
			expectedError:  entity.NewError("REFUND_ACCOUNT_INVALID", "refund posting accounts must be distinct"),
		},
		{
			name: "missing cash account",
			req: func() service.RefundRequest {
				r := baseReq
				r.CashAccount = ""
				return r
			}(),
			originalExists: true,
			accounts:       accounts,
			expectedResult: service.RefundLines{},
			expectedError:  entity.NewError("REFUND_ACCOUNT_REQUIRED", "refund requires merchant, refunds-payable, and cash accounts"),
		},
		{
			name: "frozen account",
			req: func() service.RefundRequest {
				r := baseReq
				r.CashAccount = acctFrozen
				return r
			}(),
			originalExists: true,
			accounts:       accounts,
			expectedResult: service.RefundLines{},
			expectedError:  entity.NewError("ACCOUNT_FROZEN", "refund account is frozen"),
		},
		{
			name: "unknown cash account",
			req: func() service.RefundRequest {
				r := baseReq
				r.CashAccount = "unknown-cash"
				return r
			}(),
			originalExists: true,
			accounts:       accounts,
			expectedResult: service.RefundLines{},
			expectedError:  entity.NewError("REFUND_ACCOUNT_NOT_FOUND", "refund account is unknown"),
		},
		{
			name: "asset mismatch",
			req: func() service.RefundRequest {
				r := baseReq
				r.AssetCode = currEUR
				return r
			}(),
			originalExists: true,
			accounts:       accounts,
			expectedResult: service.RefundLines{},
			expectedError:  entity.NewError("CURRENCY_MISMATCH", "refund account asset must match the refund asset"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.ValidateRefund(tc.req, tc.originalExists, tc.accounts)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
