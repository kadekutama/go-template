package service_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestValidateImmediate(t *testing.T) {
	t.Parallel()

	const (
		acctSrc = valueobject.AccountID("a-src")
		acctDst = valueobject.AccountID("a-dst")
		acctFrz = valueobject.AccountID("a-frz")
		acctCls = valueobject.AccountID("a-cls")
		acctEur = valueobject.AccountID("a-eur")
		currUSD = valueobject.AssetCode("USD")
		currEUR = valueobject.AssetCode("EUR")
	)

	accounts := map[valueobject.AccountID]entity.AccountData{
		acctSrc: {ID: acctSrc, TenantID: "t-1", LedgerID: "l-1", Number: "2000", Name: "src", Class: valueobject.ClassLiability, AssetCode: currUSD, Status: valueobject.StatusActive, Version: 1},
		acctDst: {ID: acctDst, TenantID: "t-1", LedgerID: "l-1", Number: "2001", Name: "dst", Class: valueobject.ClassLiability, AssetCode: currUSD, Status: valueobject.StatusActive, Version: 1},
		acctFrz: {ID: acctFrz, TenantID: "t-1", LedgerID: "l-1", Number: "2002", Name: "frz", Class: valueobject.ClassLiability, AssetCode: currUSD, Status: valueobject.StatusFrozen, Version: 1},
		acctCls: {ID: acctCls, TenantID: "t-1", LedgerID: "l-1", Number: "2004", Name: "cls", Class: valueobject.ClassLiability, AssetCode: currUSD, Status: valueobject.StatusClosed, Version: 1},
		acctEur: {ID: acctEur, TenantID: "t-1", LedgerID: "l-1", Number: "2003", Name: "eur", Class: valueobject.ClassLiability, AssetCode: currEUR, Status: valueobject.StatusActive, Version: 1},
	}

	baseReq := service.TransferRequest{
		TenantID:    "t-1",
		LedgerID:    "l-1",
		Source:      acctSrc,
		Dest:        acctDst,
		AssetCode:   currUSD,
		AmountMinor: 5000,
	}

	type testCase struct {
		name                 string
		req                  service.TransferRequest
		accounts             map[valueobject.AccountID]entity.AccountData
		sourceAvailableMinor int64
		expectedResult       service.TransferLines
		expectedError        error
	}

	testCases := []testCase{
		{
			name:                 "ok",
			req:                  baseReq,
			accounts:             accounts,
			sourceAvailableMinor: 9000,
			expectedResult: service.TransferLines{
				DebitAccount:  acctSrc,
				CreditAccount: acctDst,
				AmountMinor:   5000,
				AssetCode:     currUSD,
			},
			expectedError: nil,
		},
		{
			name:                 "insufficient funds",
			req:                  baseReq,
			accounts:             accounts,
			sourceAvailableMinor: 100,
			expectedResult:       service.TransferLines{},
			expectedError:        entity.NewError("INSUFFICIENT_FUNDS", "source available balance below transfer amount"),
		},
		{
			name: "zero amount",
			req: func() service.TransferRequest {
				r := baseReq
				r.AmountMinor = 0
				return r
			}(),
			accounts:             accounts,
			sourceAvailableMinor: 100,
			expectedResult:       service.TransferLines{},
			expectedError:        entity.NewError("INVALID_TRANSFER_AMOUNT", "transfer amount must be positive"),
		},
		{
			name: "self-transfer",
			req: func() service.TransferRequest {
				r := baseReq
				r.Dest = r.Source
				return r
			}(),
			accounts:             accounts,
			sourceAvailableMinor: 999999,
			expectedResult:       service.TransferLines{},
			expectedError:        entity.NewError("SELF_TRANSFER_REJECTED", "transfer source and destination must differ"),
		},
		{
			name: "frozen source",
			req: func() service.TransferRequest {
				r := baseReq
				r.Source = acctFrz
				return r
			}(),
			accounts:             accounts,
			sourceAvailableMinor: 999999,
			expectedResult:       service.TransferLines{},
			expectedError:        entity.NewError("ACCOUNT_FROZEN", "account is frozen"),
		},
		{
			name: "closed dest",
			req: func() service.TransferRequest {
				r := baseReq
				r.Dest = acctCls
				return r
			}(),
			accounts:             accounts,
			sourceAvailableMinor: 999999,
			expectedResult:       service.TransferLines{},
			expectedError:        entity.NewError("ACCOUNT_CLOSED", "account is closed"),
		},
		{
			name: "tenant mismatch",
			req: func() service.TransferRequest {
				r := baseReq
				r.TenantID = "t-other"
				return r
			}(),
			accounts:             accounts,
			sourceAvailableMinor: 999999,
			expectedResult:       service.TransferLines{},
			expectedError:        entity.NewError("TENANT_MISMATCH", "transfer accounts must share the request tenant"),
		},
		{
			name: "ledger mismatch",
			req: func() service.TransferRequest {
				r := baseReq
				r.LedgerID = "l-other"
				return r
			}(),
			accounts:             accounts,
			sourceAvailableMinor: 999999,
			expectedResult:       service.TransferLines{},
			expectedError:        entity.NewError("LEDGER_MISMATCH", "transfer accounts must share the request ledger"),
		},
		{
			name: "unknown source",
			req: func() service.TransferRequest {
				r := baseReq
				r.Source = "unknown-acct"
				return r
			}(),
			accounts:             accounts,
			sourceAvailableMinor: 999999,
			expectedResult:       service.TransferLines{},
			expectedError:        entity.NewError("SOURCE_ACCOUNT_NOT_FOUND", "source account is unknown"),
		},
		{
			name: "unknown dest",
			req: func() service.TransferRequest {
				r := baseReq
				r.Dest = "unknown-acct"
				return r
			}(),
			accounts:             accounts,
			sourceAvailableMinor: 999999,
			expectedResult:       service.TransferLines{},
			expectedError:        entity.NewError("DEST_ACCOUNT_NOT_FOUND", "destination account is unknown"),
		},
		{
			name: "cross-currency without fx",
			req: func() service.TransferRequest {
				r := baseReq
				r.Dest = acctEur
				return r
			}(),
			accounts:             accounts,
			sourceAvailableMinor: 999999,
			expectedResult:       service.TransferLines{},
			expectedError:        entity.NewError("CURRENCY_MISMATCH", "transfer asset must match account assets"),
		},
		{
			name: "cross-currency with fx",
			req: func() service.TransferRequest {
				r := baseReq
				r.Dest = acctEur
				r.FXRatePresent = true
				return r
			}(),
			accounts:             accounts,
			sourceAvailableMinor: 999999,
			expectedResult: service.TransferLines{
				DebitAccount:  acctSrc,
				CreditAccount: acctEur,
				AmountMinor:   5000,
				AssetCode:     currUSD,
			},
			expectedError: nil,
		},
		{
			name: "fx matching neither leg",
			req: func() service.TransferRequest {
				r := baseReq
				r.AssetCode = currEUR
				r.FXRatePresent = true
				return r
			}(),
			accounts:             accounts,
			sourceAvailableMinor: 999999,
			expectedResult:       service.TransferLines{},
			expectedError:        entity.NewError("CURRENCY_MISMATCH", "transfer asset must match at least one leg"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.ValidateImmediate(tc.req, tc.accounts, tc.sourceAvailableMinor)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestValidateSchedule(t *testing.T) {
	t.Parallel()

	const (
		acctSrc = valueobject.AccountID("a-src")
		acctDst = valueobject.AccountID("a-dst")
		currUSD = valueobject.AssetCode("USD")
	)

	accounts := map[valueobject.AccountID]entity.AccountData{
		acctSrc: {ID: acctSrc, TenantID: "t-1", LedgerID: "l-1", Number: "2000", Name: "src", Class: valueobject.ClassLiability, AssetCode: currUSD, Status: valueobject.StatusActive, Version: 1},
		acctDst: {ID: acctDst, TenantID: "t-1", LedgerID: "l-1", Number: "2001", Name: "dst", Class: valueobject.ClassLiability, AssetCode: currUSD, Status: valueobject.StatusActive, Version: 1},
	}

	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	baseSchedule := service.ScheduleRequest{
		TransferRequest: service.TransferRequest{
			TenantID:    "t-1",
			LedgerID:    "l-1",
			Source:      acctSrc,
			Dest:        acctDst,
			AssetCode:   currUSD,
			AmountMinor: 5000,
		},
		ExecuteAt:       now.Add(time.Hour),
		IdempotencyRoot: "sched-1",
	}

	type testCase struct {
		name          string
		req           service.ScheduleRequest
		accounts      map[valueobject.AccountID]entity.AccountData
		now           time.Time
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "ok without funds check",
			req:           baseSchedule,
			accounts:      accounts,
			now:           now,
			expectedError: nil,
		},
		{
			name: "zero amount",
			req: func() service.ScheduleRequest {
				s := baseSchedule
				s.AmountMinor = 0
				return s
			}(),
			accounts:      accounts,
			now:           now,
			expectedError: entity.NewError("INVALID_TRANSFER_AMOUNT", "transfer amount must be positive"),
		},
		{
			name: "missing root",
			req: func() service.ScheduleRequest {
				s := baseSchedule
				s.IdempotencyRoot = ""
				return s
			}(),
			accounts:      accounts,
			now:           now,
			expectedError: entity.NewError("IDEMPOTENCY_ROOT_REQUIRED", "scheduled transfer requires an idempotency root"),
		},
		{
			name: "past execution",
			req: func() service.ScheduleRequest {
				s := baseSchedule
				s.ExecuteAt = now.Add(-time.Hour)
				return s
			}(),
			accounts:      accounts,
			now:           now,
			expectedError: entity.NewError("EXECUTE_AT_INVALID", "scheduled transfer must execute in the future"),
		},
		{
			name: "bad recurrence",
			req: func() service.ScheduleRequest {
				s := baseSchedule
				s.Recurrence = &service.RecurrenceRule{Occurrences: 0}
				return s
			}(),
			accounts:      accounts,
			now:           now,
			expectedError: entity.NewError("RECURRENCE_INVALID", "recurrence occurrences must be positive"),
		},
		{
			name: "self-transfer",
			req: func() service.ScheduleRequest {
				s := baseSchedule
				s.Dest = s.Source
				return s
			}(),
			accounts:      accounts,
			now:           now,
			expectedError: entity.NewError("SELF_TRANSFER_REJECTED", "transfer source and destination must differ"),
		},
		{
			name: "unknown source",
			req: func() service.ScheduleRequest {
				s := baseSchedule
				s.Source = "unknown-acct"
				return s
			}(),
			accounts:      accounts,
			now:           now,
			expectedError: entity.NewError("SOURCE_ACCOUNT_NOT_FOUND", "source account is unknown"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.ValidateSchedule(tc.req, tc.accounts, tc.now)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestValidateExecution(t *testing.T) {
	t.Parallel()

	const (
		acctSrc = valueobject.AccountID("a-src")
		acctDst = valueobject.AccountID("a-dst")
		currUSD = valueobject.AssetCode("USD")
	)

	accounts := map[valueobject.AccountID]entity.AccountData{
		acctSrc: {ID: acctSrc, TenantID: "t-1", LedgerID: "l-1", Number: "2000", Name: "src", Class: valueobject.ClassLiability, AssetCode: currUSD, Status: valueobject.StatusActive, Version: 1},
		acctDst: {ID: acctDst, TenantID: "t-1", LedgerID: "l-1", Number: "2001", Name: "dst", Class: valueobject.ClassLiability, AssetCode: currUSD, Status: valueobject.StatusActive, Version: 1},
	}

	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	sched := service.ScheduleRequest{
		TransferRequest: service.TransferRequest{
			TenantID:    "t-1",
			LedgerID:    "l-1",
			Source:      acctSrc,
			Dest:        acctDst,
			AssetCode:   currUSD,
			AmountMinor: 5000,
		},
		ExecuteAt:       now.Add(time.Hour),
		IdempotencyRoot: "sched-1",
	}

	type testCase struct {
		name                 string
		req                  service.ScheduleRequest
		accounts             map[valueobject.AccountID]entity.AccountData
		sourceAvailableMinor int64
		expectedResult       service.TransferLines
		expectedError        error
	}

	testCases := []testCase{
		{
			name:                 "funds enforced at execution",
			req:                  sched,
			accounts:             accounts,
			sourceAvailableMinor: 9000,
			expectedResult: service.TransferLines{
				DebitAccount:  acctSrc,
				CreditAccount: acctDst,
				AmountMinor:   5000,
				AssetCode:     currUSD,
			},
			expectedError: nil,
		},
		{
			name:                 "insufficient future funds fail at execution",
			req:                  sched,
			accounts:             accounts,
			sourceAvailableMinor: 10,
			expectedResult:       service.TransferLines{},
			expectedError:        entity.NewError("INSUFFICIENT_FUNDS", "source available balance below transfer amount"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.ValidateExecution(tc.req, tc.accounts, tc.sourceAvailableMinor)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestExpandRecurrence(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		root           string
		n              int
		expectedResult []string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "keys",
			root:           "r1",
			n:              3,
			expectedResult: []string{"r1:1", "r1:2", "r1:3"},
			expectedError:  nil,
		},
		{
			name:           "empty root",
			root:           "",
			n:              3,
			expectedResult: []string(nil),
			expectedError:  entity.NewError("IDEMPOTENCY_ROOT_REQUIRED", "recurrence root is required"),
		},
		{
			name:           "zero occurrences",
			root:           "r1",
			n:              0,
			expectedResult: []string(nil),
			expectedError:  entity.NewError("RECURRENCE_INVALID", "recurrence occurrences must be positive"),
		},
		{
			name:           "negative occurrences",
			root:           "r1",
			n:              -2,
			expectedResult: []string(nil),
			expectedError:  entity.NewError("RECURRENCE_INVALID", "recurrence occurrences must be positive"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.ExpandRecurrence(tc.root, tc.n)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestEvaluateBatch(t *testing.T) {
	t.Parallel()

	baseTransfer := service.TransferRequest{
		TenantID:    "t-1",
		LedgerID:    "l-1",
		Source:      "a-src",
		Dest:        "a-dst",
		AssetCode:   "USD",
		AmountMinor: 5000,
	}

	accounts := map[valueobject.AccountID]entity.AccountData{
		"a-src": {ID: "a-src", TenantID: "t-1", LedgerID: "l-1", Number: "2000", Name: "src", Class: valueobject.ClassLiability, AssetCode: "USD", Status: valueobject.StatusActive, Version: 1},
		"a-dst": {ID: "a-dst", TenantID: "t-1", LedgerID: "l-1", Number: "2001", Name: "dst", Class: valueobject.ClassLiability, AssetCode: "USD", Status: valueobject.StatusActive, Version: 1},
	}

	validate := func(it service.BatchItem) error {
		_, err := service.ValidateImmediate(it.Request, accounts, 999999)
		return err
	}
	failFn := func(service.BatchItem) error { return errors.New("boom") }

	type testCase struct {
		name           string
		batchID        string
		items          []service.BatchItem
		fn             func(service.BatchItem) error
		expectedResult service.BatchResult
	}

	testCases := []testCase{
		{
			name:    "partial batch",
			batchID: "b1",
			items: []service.BatchItem{
				{Key: "i1", Request: baseTransfer},
				{
					Key: "i2",
					Request: func() service.TransferRequest {
						r := baseTransfer
						r.AmountMinor = 0
						return r
					}(),
				},
			},
			fn: validate,
			expectedResult: service.BatchResult{
				BatchID: "b1",
				Status:  service.BatchPartial,
				Items: []service.BatchItemResult{
					{Key: "i1", OK: true, Code: "OK", Message: "ok"},
					{Key: "i2", OK: false, Code: "INVALID_TRANSFER_AMOUNT", Message: "INVALID_TRANSFER_AMOUNT: transfer amount must be positive"},
				},
			},
		},
		{
			name:    "all ok batch",
			batchID: "b1",
			items:   []service.BatchItem{{Key: "i1", Request: baseTransfer}},
			fn:      validate,
			expectedResult: service.BatchResult{
				BatchID: "b1",
				Status:  service.BatchCompleted,
				Items:   []service.BatchItemResult{{Key: "i1", OK: true, Code: "OK", Message: "ok"}},
			},
		},
		{
			name:    "all fail batch",
			batchID: "b1",
			items:   []service.BatchItem{{Key: "i1"}},
			fn:      failFn,
			expectedResult: service.BatchResult{
				BatchID: "b1",
				Status:  service.BatchFailed,
				Items:   []service.BatchItemResult{{Key: "i1", OK: false, Code: "BATCH_ITEM_FAILED", Message: "boom"}},
			},
		},
		{
			name:    "empty batch id",
			batchID: "",
			items:   []service.BatchItem{{Key: "i1", Request: baseTransfer}},
			fn:      validate,
			expectedResult: service.BatchResult{
				BatchID: "",
				Status:  service.BatchFailed,
				Items:   []service.BatchItemResult{{Key: "", OK: false, Code: "BATCH_ID_REQUIRED", Message: "batch id is required"}},
			},
		},
		{
			name:           "empty items list",
			batchID:        "b1",
			items:          nil,
			fn:             validate,
			expectedResult: service.BatchResult{BatchID: "b1", Status: service.BatchFailed},
		},
		{
			name:    "missing item key",
			batchID: "b1",
			items:   []service.BatchItem{{Key: ""}},
			fn:      validate,
			expectedResult: service.BatchResult{
				BatchID: "b1",
				Status:  service.BatchFailed,
				Items:   []service.BatchItemResult{{Key: "", OK: false, Code: "BATCH_ITEM_REQUIRED", Message: "batch item key is required"}},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := service.EvaluateBatch(tc.batchID, tc.items, tc.fn)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestBatchItemKey(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		batchID        string
		itemKey        string
		expectedResult string
	}

	testCases := []testCase{
		{
			name:           "scoped key",
			batchID:        "b1",
			itemKey:        "i1",
			expectedResult: "b1:i1",
		},
		{
			name:           "other scope",
			batchID:        "batch-99",
			itemKey:        "item-88",
			expectedResult: "batch-99:item-88",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := service.BatchItemKey(tc.batchID, tc.itemKey)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestValidateTemplate(t *testing.T) {
	t.Parallel()

	base := service.TransferTemplate{
		ID:          "tpl-1",
		TenantID:    "t-1",
		LedgerID:    "l-1",
		Source:      "a-src",
		Dest:        "a-dst",
		AssetCode:   "USD",
		AmountMinor: 100,
	}

	type testCase struct {
		name          string
		template      service.TransferTemplate
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid template",
			template:      base,
			expectedError: nil,
		},
		{
			name: "missing template id",
			template: func() service.TransferTemplate {
				t := base
				t.ID = ""
				return t
			}(),
			expectedError: entity.NewError("TEMPLATE_ID_REQUIRED", "template id is required"),
		},
		{
			name: "missing tenant scope",
			template: func() service.TransferTemplate {
				t := base
				t.TenantID = ""
				return t
			}(),
			expectedError: entity.NewError("TENANT_MISMATCH", "template requires tenant and ledger scope"),
		},
		{
			name: "missing ledger scope",
			template: func() service.TransferTemplate {
				t := base
				t.LedgerID = ""
				return t
			}(),
			expectedError: entity.NewError("TENANT_MISMATCH", "template requires tenant and ledger scope"),
		},
		{
			name: "missing source account",
			template: func() service.TransferTemplate {
				t := base
				t.Source = ""
				return t
			}(),
			expectedError: entity.NewError("TEMPLATE_ACCOUNT_REQUIRED", "template requires source and destination accounts"),
		},
		{
			name: "missing dest account",
			template: func() service.TransferTemplate {
				t := base
				t.Dest = ""
				return t
			}(),
			expectedError: entity.NewError("TEMPLATE_ACCOUNT_REQUIRED", "template requires source and destination accounts"),
		},
		{
			name: "missing asset code",
			template: func() service.TransferTemplate {
				t := base
				t.AssetCode = ""
				return t
			}(),
			expectedError: entity.NewError("TEMPLATE_ASSET_REQUIRED", "template requires an asset code"),
		},
		{
			name: "zero amount",
			template: func() service.TransferTemplate {
				t := base
				t.AmountMinor = 0
				return t
			}(),
			expectedError: entity.NewError("INVALID_TRANSFER_AMOUNT", "template amount must be positive"),
		},
		{
			name: "negative amount",
			template: func() service.TransferTemplate {
				t := base
				t.AmountMinor = -10
				return t
			}(),
			expectedError: entity.NewError("INVALID_TRANSFER_AMOUNT", "template amount must be positive"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.ValidateTemplate(tc.template)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestApplyTemplate(t *testing.T) {
	t.Parallel()

	base := service.TransferTemplate{
		ID:          "tpl-1",
		TenantID:    "t-1",
		LedgerID:    "l-1",
		Source:      "a-src",
		Dest:        "a-dst",
		AssetCode:   "USD",
		AmountMinor: 100,
	}

	baseExpected := service.TransferRequest{
		TenantID:    "t-1",
		LedgerID:    "l-1",
		Source:      "a-src",
		Dest:        "a-dst",
		AssetCode:   "USD",
		AmountMinor: 100,
	}

	type testCase struct {
		name           string
		template       service.TransferTemplate
		amountMinor    int64
		fxRatePresent  bool
		expectedResult service.TransferRequest
		expectedError  error
	}

	testCases := []testCase{
		{
			name:          "override amount",
			template:      base,
			amountMinor:   250,
			fxRatePresent: false,
			expectedResult: func() service.TransferRequest {
				r := baseExpected
				r.AmountMinor = 250
				return r
			}(),
			expectedError: nil,
		},
		{
			name:           "default amount from template",
			template:       base,
			amountMinor:    0,
			fxRatePresent:  false,
			expectedResult: baseExpected,
			expectedError:  nil,
		},
		{
			name:           "invalid template rejected",
			template:       service.TransferTemplate{},
			amountMinor:    0,
			fxRatePresent:  false,
			expectedResult: service.TransferRequest{},
			expectedError:  entity.NewError("TEMPLATE_ID_REQUIRED", "template id is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.ApplyTemplate(tc.template, tc.amountMinor, tc.fxRatePresent)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
