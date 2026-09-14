package service

import (
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// RecoveryStatus is the durable recovery workflow state.
type RecoveryStatus = valueobject.RecoveryStatus

// Recovery states (aliases of the valueobject lifecycle).
const (
	RecoveryPending   = valueobject.RecoveryPending
	RecoveryCollected = valueobject.RecoveryCollected
	RecoveryFailed    = valueobject.RecoveryFailed
	RecoveryCanceled  = valueobject.RecoveryCanceled
)

// RecoveryAttempt is the durable, idempotent recovery workflow: one attempt
// per (tenant, account, asset, idempotency key). Provider timeouts become
// OUTCOME_UNKNOWN and require status lookup before retry.
type RecoveryAttempt struct {
	TenantID        valueobject.TenantID
	AccountID       valueobject.AccountID
	AssetCode       valueobject.AssetCode
	IdempotencyKey  string
	ProviderKey     string
	ProviderTraceID string
	Fingerprint     string
	InstrumentID    string
	AmountMinor     int64
	Status          RecoveryStatus
	Outcome         ProviderOutcome
	Actor           string
	EvidenceURI     string
	CreatedAt       time.Time
}

// RecoveryKey scopes idempotency to tenant/account/asset/key.
func RecoveryKey(tenant valueobject.TenantID, account valueobject.AccountID, asset valueobject.AssetCode, key string) string {
	return tenant.String() + "|" + account.String() + "|" + string(asset) + "|" + key
}

// StartRecovery opens (or replays) a recovery attempt. Same fingerprint
// replays the original result; a different fingerprint for the same key
// conflicts. Unknown provider outcome is sticky until resolved.
func StartRecovery(existing map[string]RecoveryAttempt, attempt RecoveryAttempt, fingerprint string) (RecoveryAttempt, error) {
	if attempt.TenantID.String() == "" || attempt.AccountID.String() == "" || attempt.AssetCode == "" {
		return RecoveryAttempt{}, entity.NewError("RECOVERY_SCOPE_REQUIRED", "recovery requires tenant, account, and asset scope")
	}
	if attempt.IdempotencyKey == "" || attempt.ProviderKey == "" {
		return RecoveryAttempt{}, entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "recovery requires idempotency and provider keys")
	}
	if attempt.InstrumentID == "" {
		return RecoveryAttempt{}, entity.NewError("ACCOUNT_UNVERIFIED", "recovery requires a verified external bank instrument")
	}
	if attempt.AmountMinor <= 0 {
		return RecoveryAttempt{}, entity.NewError("INVALID_RECOVERY_AMOUNT", "recovery amount must be positive")
	}
	if attempt.Actor == "" || attempt.EvidenceURI == "" {
		return RecoveryAttempt{}, entity.NewError("EVIDENCE_REQUIRED", "recovery requires actor and evidence")
	}
	k := RecoveryKey(attempt.TenantID, attempt.AccountID, attempt.AssetCode, attempt.IdempotencyKey)
	if prev, ok := existing[k]; ok {
		if fingerprint != prev.Fingerprint {
			return prev, entity.NewError("IDEMPOTENCY_CONFLICT", "idempotency key reuse with different fingerprint")
		}
		return prev, nil
	}
	attempt.Status = RecoveryPending
	attempt.Outcome = OutcomeUnknown
	attempt.Fingerprint = fingerprint
	existing[k] = attempt
	return attempt, nil
}

// ResolveRecoveryOutcome records a provider status lookup result.
func ResolveRecoveryOutcome(attempt RecoveryAttempt, confirmed bool, traceID string) RecoveryAttempt {
	if confirmed {
		attempt.Outcome = OutcomeConfirmed
	} else {
		attempt.Outcome = OutcomeFailed
	}
	if traceID != "" {
		attempt.ProviderTraceID = traceID
	}
	return attempt
}

// RecoveryPosting is the confirmed-collection funding/offset shape: a new
// balanced posting plus audit/outbox facts. It never mutates posted rows or
// a balance column.
type RecoveryPosting struct {
	DebitAccount  valueobject.AccountID
	CreditAccount valueobject.AccountID
	AmountMinor   int64
	AssetCode     valueobject.AssetCode
	AuditRef      string
	OutboxEvent   string
}

// ConfirmRecovery commits a confirmed collection: requires a PENDING attempt
// with CONFIRMED outcome (no blind retry from UNKNOWN) and yields the
// balanced posting shape crediting the recovery account.
func ConfirmRecovery(attempt RecoveryAttempt, debit, credit valueobject.AccountID, auditRef string) (RecoveryAttempt, RecoveryPosting, error) {
	if attempt.Status != valueobject.RecoveryPending {
		return attempt, RecoveryPosting{}, entity.NewError("RECOVERY_STATE_INVALID", "recovery is not pending")
	}
	if attempt.Outcome != OutcomeConfirmed {
		return attempt, RecoveryPosting{}, entity.NewError("OUTCOME_UNKNOWN", "recovery outcome unknown: status lookup required before commit")
	}
	if debit.String() == "" || credit.String() == "" {
		return attempt, RecoveryPosting{}, entity.NewError("RECOVERY_ACCOUNT_REQUIRED", "recovery requires debit and credit accounts")
	}
	if debit == credit {
		return attempt, RecoveryPosting{}, entity.NewError("RECOVERY_ACCOUNT_INVALID", "recovery posting accounts must be distinct")
	}
	if credit != attempt.AccountID {
		return attempt, RecoveryPosting{}, entity.NewError("RECOVERY_ACCOUNT_MISMATCH", "recovery must credit the recovery account")
	}
	if auditRef == "" {
		return attempt, RecoveryPosting{}, entity.NewError("EVIDENCE_REQUIRED", "recovery commit requires an audit reference")
	}
	attempt.Status = RecoveryCollected
	posting := RecoveryPosting{
		DebitAccount:  debit,
		CreditAccount: credit,
		AmountMinor:   attempt.AmountMinor,
		AssetCode:     attempt.AssetCode,
		AuditRef:      auditRef,
		OutboxEvent:   "recovery.collected.v1",
	}
	return attempt, posting, nil
}

// CancelRecovery cancels a PENDING attempt explicitly. Terminal attempts
// reject cancel.
func CancelRecovery(attempt RecoveryAttempt) (RecoveryAttempt, error) {
	if !valueobject.CanTransitionRecovery(attempt.Status, valueobject.RecoveryCanceled) {
		return attempt, entity.NewError("RECOVERY_STATE_INVALID", "recovery can be canceled only while PENDING")
	}
	attempt.Status = RecoveryCanceled
	return attempt, nil
}
