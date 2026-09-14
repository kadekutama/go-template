package service_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestValidateTopup(t *testing.T) {
	t.Parallel()

	baseReq := service.TopupRequest{
		ID:                 "tp-1",
		TenantID:           "t-1",
		LedgerID:           "l-1",
		CreditAccount:      "m-1",
		AmountMinor:        5000,
		AssetCode:          "USD",
		InstrumentID:       "bank-1",
		InstrumentVerified: true,
		IdempotencyKey:     "k-1",
	}

	type testCase struct {
		name           string
		req            service.TopupRequest
		expectedResult service.TopupFunding
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "ok",
			req:  baseReq,
			expectedResult: service.TopupFunding{
				MerchantAccount: "m-1",
				AmountMinor:     5000,
				AssetCode:       "USD",
			},
			expectedError: nil,
		},
		{
			name: "unverified instrument",
			req: func() service.TopupRequest {
				r := baseReq
				r.InstrumentVerified = false
				return r
			}(),
			expectedResult: service.TopupFunding{},
			expectedError:  &entity.Error{Code: "ACCOUNT_UNVERIFIED", Message: "top-up requires a verified external bank instrument"},
		},
		{
			name: "zero amount",
			req: func() service.TopupRequest {
				r := baseReq
				r.AmountMinor = 0
				return r
			}(),
			expectedResult: service.TopupFunding{},
			expectedError:  &entity.Error{Code: "INVALID_TOPUP_AMOUNT", Message: "top-up amount must be positive"},
		},
		{
			name: "missing key",
			req: func() service.TopupRequest {
				r := baseReq
				r.IdempotencyKey = ""
				return r
			}(),
			expectedResult: service.TopupFunding{},
			expectedError:  &entity.Error{Code: "IDEMPOTENCY_KEY_REQUIRED", Message: "top-up requires an idempotency key"},
		},
		{
			name: "missing account",
			req: func() service.TopupRequest {
				r := baseReq
				r.CreditAccount = ""
				return r
			}(),
			expectedResult: service.TopupFunding{},
			expectedError:  &entity.Error{Code: "TOPUP_ACCOUNT_REQUIRED", Message: "top-up requires a credit account"},
		},
		{
			name: "missing asset",
			req: func() service.TopupRequest {
				r := baseReq
				r.AssetCode = ""
				return r
			}(),
			expectedResult: service.TopupFunding{},
			expectedError:  &entity.Error{Code: "TOPUP_ASSET_REQUIRED", Message: "top-up requires an asset code"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := service.ValidateTopup(tc.req)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestSettleTopup(t *testing.T) {
	t.Parallel()

	funding := service.TopupFunding{
		MerchantAccount: "m-1",
		AmountMinor:     5000,
		AssetCode:       "USD",
	}

	type testCase struct {
		name            string
		status          valueobject.TopupStatus
		funding         service.TopupFunding
		bankCash        valueobject.AccountID
		traceID         string
		succeeded       bool
		expectedResult1 valueobject.TopupStatus
		expectedResult2 *service.TopupFunding
		expectedError   error
	}

	testCases := []testCase{
		{
			name:            "succeeded",
			status:          valueobject.TopupPending,
			funding:         funding,
			bankCash:        "bank-cash",
			traceID:         "tr-1",
			succeeded:       true,
			expectedResult1: valueobject.TopupSucceeded,
			expectedResult2: &service.TopupFunding{
				MerchantAccount: "m-1",
				BankCash:        "bank-cash",
				AmountMinor:     5000,
				AssetCode:       "USD",
				TraceID:         "tr-1",
			},
			expectedError: nil,
		},
		{
			name:            "failed moves nothing",
			status:          valueobject.TopupPending,
			funding:         funding,
			bankCash:        "bank-cash",
			traceID:         "tr-1",
			succeeded:       false,
			expectedResult1: valueobject.TopupFailed,
			expectedResult2: nil,
			expectedError:   nil,
		},
		{
			name:            "re-settle rejected",
			status:          valueobject.TopupSucceeded,
			funding:         funding,
			bankCash:        "bank-cash",
			traceID:         "tr-1",
			succeeded:       true,
			expectedResult1: valueobject.TopupSucceeded,
			expectedResult2: nil,
			expectedError:   &entity.Error{Code: "TOPUP_STATE_INVALID", Message: "top-up can settle only while PENDING"},
		},
		{
			name:            "missing cash",
			status:          valueobject.TopupPending,
			funding:         funding,
			bankCash:        "",
			traceID:         "tr-1",
			succeeded:       true,
			expectedResult1: valueobject.TopupFailed,
			expectedResult2: nil,
			expectedError:   &entity.Error{Code: "TOPUP_ACCOUNT_REQUIRED", Message: "settlement requires a bank cash account"},
		},
		{
			name:            "zero funding",
			status:          valueobject.TopupPending,
			funding:         service.TopupFunding{},
			bankCash:        "bank",
			traceID:         "t",
			succeeded:       true,
			expectedResult1: valueobject.TopupFailed,
			expectedResult2: nil,
			expectedError:   &entity.Error{Code: "INVALID_TOPUP_AMOUNT", Message: "top-up amount must be positive"},
		},
		{
			name:            "self-dealing",
			status:          valueobject.TopupPending,
			funding:         funding,
			bankCash:        "m-1",
			traceID:         "tr-1",
			succeeded:       true,
			expectedResult1: valueobject.TopupFailed,
			expectedResult2: nil,
			expectedError:   &entity.Error{Code: "TOPUP_ACCOUNT_INVALID", Message: "settlement accounts must be distinct"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult1, actualResult2, err := service.SettleTopup(tc.status, tc.funding, tc.bankCash, tc.traceID, tc.succeeded)
			assert.Equal(t, tc.expectedResult1, actualResult1)
			assert.Equal(t, tc.expectedResult2, actualResult2)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestCancelTopup(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		status         valueobject.TopupStatus
		expectedResult valueobject.TopupStatus
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "pending cancels",
			status:         valueobject.TopupPending,
			expectedResult: valueobject.TopupCanceled,
			expectedError:  nil,
		},
		{
			name:           "settled rejected",
			status:         valueobject.TopupSucceeded,
			expectedResult: valueobject.TopupSucceeded,
			expectedError:  &entity.Error{Code: "TOPUP_CANCEL_REJECTED", Message: "top-up can be canceled only while PENDING"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := service.CancelTopup(tc.status)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestEvaluateEligibility(t *testing.T) {
	t.Parallel()

	basePolicy := valueobject.PayoutPolicy{
		TenantID:            "t-1",
		AssetCode:           "USD",
		MinimumMinor:        1000,
		FirstPayoutHoldDays: 7,
		ReserveBPS:          1000,
		InstantEligible:     true,
		Version:             "v1",
	}

	baseIn := service.EligibilityInput{
		Policy:              basePolicy,
		AvailableMinor:      20000,
		RequestedMinor:      5000,
		AssetCode:           "USD",
		TenantAgeDays:       30,
		DestinationVerified: true,
		Method:              valueobject.PayoutACH,
		Cursor:              "cur-1",
	}

	type testCase struct {
		name           string
		in             service.EligibilityInput
		expectedResult service.EligibilityResult
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "eligible",
			in:             baseIn,
			expectedResult: service.EligibilityResult{Eligible: true},
			expectedError:  nil,
		},
		{
			name: "first hold",
			in: func() service.EligibilityInput {
				in := baseIn
				in.TenantAgeDays = 3
				return in
			}(),
			expectedResult: service.EligibilityResult{Reason: service.ReasonFirstPayoutHold},
			expectedError:  nil,
		},
		{
			name: "below minimum",
			in: func() service.EligibilityInput {
				in := baseIn
				in.RequestedMinor = 100
				return in
			}(),
			expectedResult: service.EligibilityResult{Reason: service.ReasonBelowMinimum},
			expectedError:  nil,
		},
		{
			name: "reserve shortfall",
			in: func() service.EligibilityInput {
				in := baseIn
				in.AvailableMinor = 5000
				in.RequestedMinor = 4600
				return in
			}(),
			expectedResult: service.EligibilityResult{Reason: service.ReasonReserveShortfall},
			expectedError:  nil,
		},
		{
			name: "negative",
			in: func() service.EligibilityInput {
				in := baseIn
				in.AvailableMinor = -100
				return in
			}(),
			expectedResult: service.EligibilityResult{Reason: service.ReasonNegativeAvailable},
			expectedError:  nil,
		},
		{
			name: "unverified dest",
			in: func() service.EligibilityInput {
				in := baseIn
				in.DestinationVerified = false
				return in
			}(),
			expectedResult: service.EligibilityResult{Reason: service.ReasonDestinationUnverified},
			expectedError:  nil,
		},
		{
			name: "instant ineligible",
			in: func() service.EligibilityInput {
				in := baseIn
				in.Method = valueobject.PayoutRTP
				in.Policy.InstantEligible = false
				return in
			}(),
			expectedResult: service.EligibilityResult{Reason: service.ReasonInstantIneligible},
			expectedError:  nil,
		},
		{
			name: "bad policy",
			in: func() service.EligibilityInput {
				in := baseIn
				in.Policy = valueobject.PayoutPolicy{}
				return in
			}(),
			expectedResult: service.EligibilityResult{},
			expectedError:  &entity.Error{Code: "PAYOUT_POLICY_INVALID", Message: "payout policy is invalid"},
		},
		{
			name: "missing cursor",
			in: func() service.EligibilityInput {
				in := baseIn
				in.Cursor = ""
				return in
			}(),
			expectedResult: service.EligibilityResult{},
			expectedError:  &entity.Error{Code: "CURSOR_REQUIRED", Message: "eligibility requires the strong-projection cursor"},
		},
		{
			name: "asset mismatch",
			in: func() service.EligibilityInput {
				in := baseIn
				in.AssetCode = "EUR"
				return in
			}(),
			expectedResult: service.EligibilityResult{},
			expectedError:  &entity.Error{Code: "CURRENCY_MISMATCH", Message: "payout asset differs from policy asset"},
		},
		{
			name: "zero amount",
			in: func() service.EligibilityInput {
				in := baseIn
				in.RequestedMinor = 0
				return in
			}(),
			expectedResult: service.EligibilityResult{},
			expectedError:  &entity.Error{Code: "INVALID_PAYOUT_AMOUNT", Message: "payout amount must be positive"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := service.EvaluateEligibility(tc.in)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestBlockedError(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		result         service.EligibilityResult
		expectedResult *entity.Error
	}

	testCases := []testCase{
		{
			name:           "eligible yields nil",
			result:         service.EligibilityResult{Eligible: true},
			expectedResult: nil,
		},
		{
			name:           "blocked maps reason",
			result:         service.EligibilityResult{Reason: service.ReasonFirstPayoutHold},
			expectedResult: &entity.Error{Code: "PAYOUT_BLOCKED", Message: "payout blocked (FIRST_PAYOUT_HOLD)"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult := tc.result.BlockedError()
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestStartRecovery(t *testing.T) {
	t.Parallel()

	baseAttempt := service.RecoveryAttempt{
		TenantID:       "t-1",
		AccountID:      "m-1",
		AssetCode:      "USD",
		IdempotencyKey: "rec-1",
		ProviderKey:    "pk-1",
		InstrumentID:   "bank-1",
		AmountMinor:    5000,
		Actor:          "ops",
		EvidenceURI:    "ev://1",
	}

	opened := baseAttempt
	opened.Status = valueobject.RecoveryPending
	opened.Outcome = service.OutcomeUnknown
	opened.Fingerprint = "fp-1"

	seededStore := map[string]service.RecoveryAttempt{
		service.RecoveryKey("t-1", "m-1", "USD", "rec-1"): opened,
	}

	type testCase struct {
		name           string
		existing       map[string]service.RecoveryAttempt
		attempt        service.RecoveryAttempt
		fingerprint    string
		expectedResult service.RecoveryAttempt
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "open unknown",
			existing:       map[string]service.RecoveryAttempt{},
			attempt:        baseAttempt,
			fingerprint:    "fp-1",
			expectedResult: opened,
			expectedError:  nil,
		},
		{
			name:           "replay returns original",
			existing:       seededStore,
			attempt:        baseAttempt,
			fingerprint:    "fp-1",
			expectedResult: opened,
			expectedError:  nil,
		},
		{
			name:           "fingerprint conflict",
			existing:       seededStore,
			attempt:        baseAttempt,
			fingerprint:    "fp-2",
			expectedResult: opened,
			expectedError:  &entity.Error{Code: "IDEMPOTENCY_CONFLICT", Message: "idempotency key reuse with different fingerprint"},
		},
		{
			name:     "missing scope",
			existing: map[string]service.RecoveryAttempt{},
			attempt: func() service.RecoveryAttempt {
				a := baseAttempt
				a.TenantID = ""
				return a
			}(),
			fingerprint:    "fp",
			expectedResult: service.RecoveryAttempt{},
			expectedError:  &entity.Error{Code: "RECOVERY_SCOPE_REQUIRED", Message: "recovery requires tenant, account, and asset scope"},
		},
		{
			name:     "missing keys",
			existing: map[string]service.RecoveryAttempt{},
			attempt: func() service.RecoveryAttempt {
				a := baseAttempt
				a.IdempotencyKey = ""
				return a
			}(),
			fingerprint:    "fp",
			expectedResult: service.RecoveryAttempt{},
			expectedError:  &entity.Error{Code: "IDEMPOTENCY_KEY_REQUIRED", Message: "recovery requires idempotency and provider keys"},
		},
		{
			name:     "unverified instrument",
			existing: map[string]service.RecoveryAttempt{},
			attempt: func() service.RecoveryAttempt {
				a := baseAttempt
				a.InstrumentID = ""
				return a
			}(),
			fingerprint:    "fp",
			expectedResult: service.RecoveryAttempt{},
			expectedError:  &entity.Error{Code: "ACCOUNT_UNVERIFIED", Message: "recovery requires a verified external bank instrument"},
		},
		{
			name:     "zero amount",
			existing: map[string]service.RecoveryAttempt{},
			attempt: func() service.RecoveryAttempt {
				a := baseAttempt
				a.AmountMinor = 0
				return a
			}(),
			fingerprint:    "fp",
			expectedResult: service.RecoveryAttempt{},
			expectedError:  &entity.Error{Code: "INVALID_RECOVERY_AMOUNT", Message: "recovery amount must be positive"},
		},
		{
			name:     "missing evidence",
			existing: map[string]service.RecoveryAttempt{},
			attempt: func() service.RecoveryAttempt {
				a := baseAttempt
				a.Actor = ""
				return a
			}(),
			fingerprint:    "fp",
			expectedResult: service.RecoveryAttempt{},
			expectedError:  &entity.Error{Code: "EVIDENCE_REQUIRED", Message: "recovery requires actor and evidence"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := service.StartRecovery(tc.existing, tc.attempt, tc.fingerprint)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestResolveRecoveryOutcome(t *testing.T) {
	t.Parallel()

	pending := service.RecoveryAttempt{
		TenantID:       "t-1",
		AccountID:      "m-1",
		AssetCode:      "USD",
		IdempotencyKey: "rec-1",
		ProviderKey:    "pk-1",
		InstrumentID:   "bank-1",
		AmountMinor:    5000,
		Status:         valueobject.RecoveryPending,
		Outcome:        service.OutcomeUnknown,
		Fingerprint:    "fp-1",
		Actor:          "ops",
		EvidenceURI:    "ev://1",
	}

	type testCase struct {
		name           string
		attempt        service.RecoveryAttempt
		confirmed      bool
		traceID        string
		expectedResult service.RecoveryAttempt
	}

	testCases := []testCase{
		{
			name:      "confirmed",
			attempt:   pending,
			confirmed: true,
			traceID:   "tr-9",
			expectedResult: func() service.RecoveryAttempt {
				r := pending
				r.Outcome = service.OutcomeConfirmed
				r.ProviderTraceID = "tr-9"
				return r
			}(),
		},
		{
			name:      "failed",
			attempt:   pending,
			confirmed: false,
			traceID:   "",
			expectedResult: func() service.RecoveryAttempt {
				r := pending
				r.Outcome = service.OutcomeFailed
				return r
			}(),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult := service.ResolveRecoveryOutcome(tc.attempt, tc.confirmed, tc.traceID)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestConfirmRecovery(t *testing.T) {
	t.Parallel()

	pending := service.RecoveryAttempt{
		TenantID:       "t-1",
		AccountID:      "m-1",
		AssetCode:      "USD",
		IdempotencyKey: "rec-1",
		ProviderKey:    "pk-1",
		InstrumentID:   "bank-1",
		AmountMinor:    5000,
		Status:         valueobject.RecoveryPending,
		Outcome:        service.OutcomeUnknown,
		Fingerprint:    "fp-1",
		Actor:          "ops",
		EvidenceURI:    "ev://1",
	}

	resolved := pending
	resolved.Outcome = service.OutcomeConfirmed
	resolved.ProviderTraceID = "tr-9"

	type testCase struct {
		name            string
		attempt         service.RecoveryAttempt
		debit           valueobject.AccountID
		credit          valueobject.AccountID
		auditRef        string
		expectedResult1 service.RecoveryAttempt
		expectedResult2 service.RecoveryPosting
		expectedError   error
	}

	testCases := []testCase{
		{
			name:     "confirmed collects",
			attempt:  resolved,
			debit:    "bank-cash",
			credit:   "m-1",
			auditRef: "audit-1",
			expectedResult1: func() service.RecoveryAttempt {
				r := resolved
				r.Status = valueobject.RecoveryCollected
				return r
			}(),
			expectedResult2: service.RecoveryPosting{
				DebitAccount:  "bank-cash",
				CreditAccount: "m-1",
				AmountMinor:   5000,
				AssetCode:     "USD",
				AuditRef:      "audit-1",
				OutboxEvent:   "recovery.collected.v1",
			},
			expectedError: nil,
		},
		{
			name:            "blind commit rejected",
			attempt:         pending,
			debit:           "bank-cash",
			credit:          "m-1",
			auditRef:        "audit-1",
			expectedResult1: pending,
			expectedResult2: service.RecoveryPosting{},
			expectedError:   &entity.Error{Code: "OUTCOME_UNKNOWN", Message: "recovery outcome unknown: status lookup required before commit"},
		},
		{
			name:            "wrong credit rejected",
			attempt:         resolved,
			debit:           "bank-cash",
			credit:          "other",
			auditRef:        "audit-1",
			expectedResult1: resolved,
			expectedResult2: service.RecoveryPosting{},
			expectedError:   &entity.Error{Code: "RECOVERY_ACCOUNT_MISMATCH", Message: "recovery must credit the recovery account"},
		},
		{
			name:            "self-dealing rejected",
			attempt:         resolved,
			debit:           "m-1",
			credit:          "m-1",
			auditRef:        "audit-1",
			expectedResult1: resolved,
			expectedResult2: service.RecoveryPosting{},
			expectedError:   &entity.Error{Code: "RECOVERY_ACCOUNT_INVALID", Message: "recovery posting accounts must be distinct"},
		},
		{
			name:            "missing accounts",
			attempt:         resolved,
			debit:           "",
			credit:          "m-1",
			auditRef:        "audit-1",
			expectedResult1: resolved,
			expectedResult2: service.RecoveryPosting{},
			expectedError:   &entity.Error{Code: "RECOVERY_ACCOUNT_REQUIRED", Message: "recovery requires debit and credit accounts"},
		},
		{
			name:            "missing audit",
			attempt:         resolved,
			debit:           "bank-cash",
			credit:          "m-1",
			auditRef:        "",
			expectedResult1: resolved,
			expectedResult2: service.RecoveryPosting{},
			expectedError:   &entity.Error{Code: "EVIDENCE_REQUIRED", Message: "recovery commit requires an audit reference"},
		},
		{
			name: "terminal rejected",
			attempt: func() service.RecoveryAttempt {
				r := resolved
				r.Status = valueobject.RecoveryCollected
				return r
			}(),
			debit:    "bank-cash",
			credit:   "m-1",
			auditRef: "audit-1",
			expectedResult1: func() service.RecoveryAttempt {
				r := resolved
				r.Status = valueobject.RecoveryCollected
				return r
			}(),
			expectedResult2: service.RecoveryPosting{},
			expectedError:   &entity.Error{Code: "RECOVERY_STATE_INVALID", Message: "recovery is not pending"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult1, actualResult2, err := service.ConfirmRecovery(tc.attempt, tc.debit, tc.credit, tc.auditRef)
			assert.Equal(t, tc.expectedResult1, actualResult1)
			assert.Equal(t, tc.expectedResult2, actualResult2)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestCancelRecovery(t *testing.T) {
	t.Parallel()

	pending := service.RecoveryAttempt{
		TenantID:       "t-1",
		AccountID:      "m-9",
		AssetCode:      "USD",
		IdempotencyKey: "rec-9",
		ProviderKey:    "pk-1",
		InstrumentID:   "bank-1",
		AmountMinor:    5000,
		Status:         valueobject.RecoveryPending,
		Outcome:        service.OutcomeUnknown,
		Fingerprint:    "fp-1",
		Actor:          "ops",
		EvidenceURI:    "ev://1",
	}

	type testCase struct {
		name           string
		attempt        service.RecoveryAttempt
		expectedResult service.RecoveryAttempt
		expectedError  error
	}

	testCases := []testCase{
		{
			name:    "pending cancels",
			attempt: pending,
			expectedResult: func() service.RecoveryAttempt {
				r := pending
				r.Status = valueobject.RecoveryCanceled
				return r
			}(),
			expectedError: nil,
		},
		{
			name: "collected rejected",
			attempt: func() service.RecoveryAttempt {
				r := pending
				r.Status = valueobject.RecoveryCollected
				return r
			}(),
			expectedResult: func() service.RecoveryAttempt {
				r := pending
				r.Status = valueobject.RecoveryCollected
				return r
			}(),
			expectedError: &entity.Error{Code: "RECOVERY_STATE_INVALID", Message: "recovery can be canceled only while PENDING"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := service.CancelRecovery(tc.attempt)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestRecoveryKey(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		tenant         valueobject.TenantID
		account        valueobject.AccountID
		asset          valueobject.AssetCode
		key            string
		expectedResult string
	}

	testCases := []testCase{
		{
			name:           "scoped key",
			tenant:         "t-1",
			account:        "m-1",
			asset:          "USD",
			key:            "rec-1",
			expectedResult: "t-1|m-1|USD|rec-1",
		},
		{
			name:           "other scope",
			tenant:         "t-2",
			account:        "a-9",
			asset:          "EUR",
			key:            "k",
			expectedResult: "t-2|a-9|EUR|k",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult := service.RecoveryKey(tc.tenant, tc.account, tc.asset, tc.key)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}
