package service_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestValidateClose(t *testing.T) {
	t.Parallel()

	basePeriod := entity.PeriodData{
		ID:       valueobject.PeriodID("11111111-1111-1111-1111-111111111111"),
		TenantID: valueobject.TenantID("22222222-2222-2222-2222-222222222222"),
		LedgerID: valueobject.LedgerID("33333333-3333-3333-3333-333333333333"),
		Start:    time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		End:      time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		Timezone: "UTC",
		Status:   entity.PeriodOpen,
		Version:  1,
	}
	baseInput := service.CloseInput{
		Period:                  basePeriod,
		UnresolvedWorkflowCount: 0,
		OpenBreakCount:          0,
		SubledgerDeltas:         map[string]int64{"USD": 0},
		FXRevalued:              true,
	}

	type testCase struct {
		name           string
		in             service.CloseInput
		expectedErrors []error
	}

	testCases := []testCase{
		{
			name:           "clean close passes",
			in:             baseInput,
			expectedErrors: nil,
		},
		{
			name: "all four blockers reported together",
			in: func() service.CloseInput {
				in := baseInput
				in.UnresolvedWorkflowCount = 2
				in.OpenBreakCount = 1
				in.SubledgerDeltas = map[string]int64{"USD": 5}
				in.FXRevalued = false
				return in
			}(),
			expectedErrors: []error{
				entity.NewError("WORKFLOWS_PENDING", "period has unresolved workflows"),
				entity.NewError("BREAKS_OPEN", "period has open reconciliation breaks"),
				entity.NewError("SUBLEDGERS_UNBALANCED", "sub-ledgers do not balance"),
				entity.NewError("FX_NOT_REVALUED", "foreign balances are not revalued"),
			},
		},
		{
			name: "nil deltas treated as balanced",
			in: func() service.CloseInput {
				in := baseInput
				in.SubledgerDeltas = nil
				return in
			}(),
			expectedErrors: nil,
		},
		{
			name: "single workflow blocker",
			in: func() service.CloseInput {
				in := baseInput
				in.UnresolvedWorkflowCount = 1
				return in
			}(),
			expectedErrors: []error{
				entity.NewError("WORKFLOWS_PENDING", "period has unresolved workflows"),
			},
		},
		{
			name: "closed period blocks close",
			in: func() service.CloseInput {
				in := baseInput
				in.Period = func() entity.PeriodData {
					p := basePeriod
					p.Status = entity.PeriodClosed
					return p
				}()
				return in
			}(),
			expectedErrors: []error{
				entity.NewError("PERIOD_CLOSED", "period close requires an open period"),
			},
		},
		{
			name: "negative counts rejected",
			in: func() service.CloseInput {
				in := baseInput
				in.UnresolvedWorkflowCount = -1
				return in
			}(),
			expectedErrors: []error{
				entity.NewError("CLOSE_INPUT_INVALID", "workflow and break counts must be non-negative"),
			},
		},
		{
			name: "closed plus four blockers reports five",
			in: func() service.CloseInput {
				in := baseInput
				in.Period = func() entity.PeriodData {
					p := basePeriod
					p.Status = entity.PeriodClosed
					return p
				}()
				in.UnresolvedWorkflowCount = 1
				in.OpenBreakCount = 1
				in.SubledgerDeltas = map[string]int64{"USD": 1}
				in.FXRevalued = false
				return in
			}(),
			expectedErrors: []error{
				entity.NewError("PERIOD_CLOSED", "period close requires an open period"),
				entity.NewError("WORKFLOWS_PENDING", "period has unresolved workflows"),
				entity.NewError("BREAKS_OPEN", "period has open reconciliation breaks"),
				entity.NewError("SUBLEDGERS_UNBALANCED", "sub-ledgers do not balance"),
				entity.NewError("FX_NOT_REVALUED", "foreign balances are not revalued"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			errs := service.ValidateClose(tc.in)
			assert.Equal(t, tc.expectedErrors, errs)
		})
	}
}

func TestBuildClosingEntries(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                    string
		incomeSummaryAccount    string
		retainedEarningsAccount string
		amountMinor             int64
		assetCode               string
		expectedResult          [2]service.ClosingLine
		expectedError           error
	}

	testCases := []testCase{
		{
			name:                    "balanced closing",
			incomeSummaryAccount:    "income-summary",
			retainedEarningsAccount: "retained-earnings",
			amountMinor:             10000,
			assetCode:               "USD",
			expectedResult: [2]service.ClosingLine{
				{AccountID: "income-summary", Side: "DEBIT", AmountMinor: 10000, AssetCode: "USD"},
				{AccountID: "retained-earnings", Side: "CREDIT", AmountMinor: 10000, AssetCode: "USD"},
			},
			expectedError: nil,
		},
		{
			name:                    "minimum amount closing",
			incomeSummaryAccount:    "income-summary",
			retainedEarningsAccount: "retained-earnings",
			amountMinor:             1,
			assetCode:               "USD",
			expectedResult: [2]service.ClosingLine{
				{AccountID: "income-summary", Side: "DEBIT", AmountMinor: 1, AssetCode: "USD"},
				{AccountID: "retained-earnings", Side: "CREDIT", AmountMinor: 1, AssetCode: "USD"},
			},
			expectedError: nil,
		},
		{
			name:                    "large amount closing",
			incomeSummaryAccount:    "income-summary",
			retainedEarningsAccount: "retained-earnings",
			amountMinor:             9223372036854775800,
			assetCode:               "USD",
			expectedResult: [2]service.ClosingLine{
				{AccountID: "income-summary", Side: "DEBIT", AmountMinor: 9223372036854775800, AssetCode: "USD"},
				{AccountID: "retained-earnings", Side: "CREDIT", AmountMinor: 9223372036854775800, AssetCode: "USD"},
			},
			expectedError: nil,
		},
		{
			name:                    "same accounts rejected",
			incomeSummaryAccount:    "income-summary",
			retainedEarningsAccount: "income-summary",
			amountMinor:             10000,
			assetCode:               "USD",
			expectedResult:          [2]service.ClosingLine{},
			expectedError:           entity.NewError("CLOSING_UNBALANCED", "closing accounts must differ"),
		},
		{
			name:                    "whitespace same accounts rejected",
			incomeSummaryAccount:    "income-summary ",
			retainedEarningsAccount: " income-summary",
			amountMinor:             10000,
			assetCode:               "USD",
			expectedResult:          [2]service.ClosingLine{},
			expectedError:           entity.NewError("CLOSING_UNBALANCED", "closing accounts must differ"),
		},
		{
			name:                    "zero amount rejected",
			incomeSummaryAccount:    "income-summary",
			retainedEarningsAccount: "retained-earnings",
			amountMinor:             0,
			assetCode:               "USD",
			expectedResult:          [2]service.ClosingLine{},
			expectedError:           entity.NewError("INVALID_ENTRY_AMOUNT", "closing amount must be positive"),
		},
		{
			name:                    "negative amount rejected",
			incomeSummaryAccount:    "income-summary",
			retainedEarningsAccount: "retained-earnings",
			amountMinor:             -100,
			assetCode:               "USD",
			expectedResult:          [2]service.ClosingLine{},
			expectedError:           entity.NewError("INVALID_ENTRY_AMOUNT", "closing amount must be positive"),
		},
		{
			name:                    "missing asset rejected",
			incomeSummaryAccount:    "income-summary",
			retainedEarningsAccount: "retained-earnings",
			amountMinor:             100,
			assetCode:               "",
			expectedResult:          [2]service.ClosingLine{},
			expectedError:           entity.NewError("CLOSING_ASSET_REQUIRED", "closing asset is required"),
		},
		{
			name:                    "whitespace income account rejected",
			incomeSummaryAccount:    "   ",
			retainedEarningsAccount: "retained-earnings",
			amountMinor:             100,
			assetCode:               "USD",
			expectedResult:          [2]service.ClosingLine{},
			expectedError:           entity.NewError("CLOSING_ACCOUNT_REQUIRED", "closing requires income summary and retained earnings accounts"),
		},
		{
			name:                    "whitespace retained account rejected",
			incomeSummaryAccount:    "income-summary",
			retainedEarningsAccount: "   ",
			amountMinor:             100,
			assetCode:               "USD",
			expectedResult:          [2]service.ClosingLine{},
			expectedError:           entity.NewError("CLOSING_ACCOUNT_REQUIRED", "closing requires income summary and retained earnings accounts"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.BuildClosingEntries(tc.incomeSummaryAccount, tc.retainedEarningsAccount, tc.amountMinor, tc.assetCode)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestValidateReopen(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		maker         string
		checker       string
		reason        string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid maker checker",
			maker:         "maker-1",
			checker:       "checker-1",
			reason:        "audit fix",
			expectedError: nil,
		},
		{
			name:          "self approval forbidden",
			maker:         "maker-1",
			checker:       "maker-1",
			reason:        "audit fix",
			expectedError: entity.NewError("SELF_APPROVAL_FORBIDDEN", "period reopen checker must differ from maker"),
		},
		{
			name:          "whitespace self approval forbidden",
			maker:         "maker-1",
			checker:       " maker-1 ",
			reason:        "audit fix",
			expectedError: entity.NewError("SELF_APPROVAL_FORBIDDEN", "period reopen checker must differ from maker"),
		},
		{
			name:          "missing reason",
			maker:         "maker-1",
			checker:       "checker-1",
			reason:        "",
			expectedError: entity.NewError("REOPEN_REASON_REQUIRED", "period reopen requires a reason"),
		},
		{
			name:          "whitespace reason",
			maker:         "maker-1",
			checker:       "checker-1",
			reason:        "   ",
			expectedError: entity.NewError("REOPEN_REASON_REQUIRED", "period reopen requires a reason"),
		},
		{
			name:          "missing checker",
			maker:         "maker-1",
			checker:       "",
			reason:        "audit fix",
			expectedError: entity.NewError("REOPEN_APPROVER_REQUIRED", "period reopen requires maker and checker"),
		},
		{
			name:          "whitespace maker rejected",
			maker:         "   ",
			checker:       "checker-1",
			reason:        "audit fix",
			expectedError: entity.NewError("REOPEN_APPROVER_REQUIRED", "period reopen requires maker and checker"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.ValidateReopen(tc.maker, tc.checker, tc.reason)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestValidateLateCorrection(t *testing.T) {
	t.Parallel()

	baseTargetPeriod := entity.PeriodData{
		ID:       valueobject.PeriodID("11111111-1111-1111-1111-111111111111"),
		TenantID: valueobject.TenantID("22222222-2222-2222-2222-222222222222"),
		LedgerID: valueobject.LedgerID("33333333-3333-3333-3333-333333333333"),
		Start:    time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		End:      time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		Timezone: "UTC",
		Status:   entity.PeriodOpen,
		Version:  1,
	}
	baseOriginalAt := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)

	type testCase struct {
		name                 string
		originalEffectiveAt  time.Time
		targetPeriod         entity.PeriodData
		originalDateRetained bool
		expectedError        error
	}

	testCases := []testCase{
		{
			name:                 "valid next open correction",
			originalEffectiveAt:  baseOriginalAt,
			targetPeriod:         baseTargetPeriod,
			originalDateRetained: true,
			expectedError:        nil,
		},
		{
			name:                "closed target rejected",
			originalEffectiveAt: baseOriginalAt,
			targetPeriod: func() entity.PeriodData {
				p := baseTargetPeriod
				p.Status = entity.PeriodClosed
				return p
			}(),
			originalDateRetained: true,
			expectedError:        entity.NewError("LATE_CORRECTION_PERIOD_REQUIRED", "late corrections must target the next open period"),
		},
		{
			name:                 "missing context rejected",
			originalEffectiveAt:  baseOriginalAt,
			targetPeriod:         baseTargetPeriod,
			originalDateRetained: false,
			expectedError:        entity.NewError("CORRECTION_CONTEXT_REQUIRED", "late correction must retain original effective-date context"),
		},
		{
			name:                 "missing original date rejected",
			originalEffectiveAt:  time.Time{},
			targetPeriod:         baseTargetPeriod,
			originalDateRetained: true,
			expectedError:        entity.NewError("CORRECTION_DATE_REQUIRED", "late correction requires the original effective date"),
		},
		{
			name:                 "original date in future relative to target period rejected",
			originalEffectiveAt:  baseTargetPeriod.End.Add(24 * time.Hour),
			targetPeriod:         baseTargetPeriod,
			originalDateRetained: true,
			expectedError:        entity.NewError("LATE_CORRECTION_PERIOD_REQUIRED", "late correction target must follow the original date"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.ValidateLateCorrection(tc.originalEffectiveAt, tc.targetPeriod, tc.originalDateRetained)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
