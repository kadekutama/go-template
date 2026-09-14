package service

import (
	"strconv"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// RecurrenceRule is a lightweight recurrence descriptor. Expansion is a pure
// key derivation; calendaring lives with the scheduler (E14).
type RecurrenceRule struct {
	// Occurrences is the total number of occurrences including the first.
	Occurrences int
}

// ScheduleRequest validates everything except funds at schedule time. Funds
// and a durable reservation are enforced atomically at execution.
type ScheduleRequest struct {
	TransferRequest
	ExecuteAt       time.Time
	Recurrence      *RecurrenceRule
	IdempotencyRoot string
}

// ValidateSchedule checks schedule-time rules: account existence, status,
// scope, asset/FX shape, future execution time, idempotency root. It MUST NOT
// check funds.
func ValidateSchedule(req ScheduleRequest, accounts map[valueobject.AccountID]entity.AccountData, now time.Time) error {
	if err := validateScheduleBasics(req, now); err != nil {
		return err
	}
	if err := validateScheduleAccounts(req, accounts); err != nil {
		return err
	}
	if req.Recurrence != nil && req.Recurrence.Occurrences < 1 {
		return entity.NewError("RECURRENCE_INVALID", "recurrence occurrences must be positive")
	}
	return nil
}

func validateScheduleBasics(req ScheduleRequest, now time.Time) error {
	if req.AmountMinor <= 0 {
		return entity.NewError("INVALID_TRANSFER_AMOUNT", "transfer amount must be positive")
	}
	if req.IdempotencyRoot == "" {
		return entity.NewError("IDEMPOTENCY_ROOT_REQUIRED", "scheduled transfer requires an idempotency root")
	}
	if req.ExecuteAt.IsZero() || !req.ExecuteAt.After(now) {
		return entity.NewError("EXECUTE_AT_INVALID", "scheduled transfer must execute in the future")
	}
	if req.Source == req.Dest {
		return entity.NewError("SELF_TRANSFER_REJECTED", "transfer source and destination must differ")
	}
	return nil
}

func validateScheduleAccounts(req ScheduleRequest, accounts map[valueobject.AccountID]entity.AccountData) error {
	src, ok := accounts[req.Source]
	if !ok {
		return entity.NewError("SOURCE_ACCOUNT_NOT_FOUND", "source account is unknown")
	}
	dst, ok := accounts[req.Dest]
	if !ok {
		return entity.NewError("DEST_ACCOUNT_NOT_FOUND", "destination account is unknown")
	}
	if err := checkTransferScope(req.TransferRequest, src, dst); err != nil {
		return err
	}
	if err := checkTransferAccountActive(src); err != nil {
		return err
	}
	if err := checkTransferAccountActive(dst); err != nil {
		return err
	}
	return checkTransferAssets(req.TransferRequest, src, dst)
}

// ValidateExecution enforces execution-time rules: the full immediate check
// including funds against the supplied strong-read snapshot.
func ValidateExecution(req ScheduleRequest, accounts map[valueobject.AccountID]entity.AccountData, sourceAvailableMinor int64) (TransferLines, error) {
	return ValidateImmediate(req.TransferRequest, accounts, sourceAvailableMinor)
}

// ExpandRecurrence derives deterministic occurrence keys `root:{occurrence}`
// for occurrences 1..n.
func ExpandRecurrence(root string, n int) ([]string, error) {
	if root == "" {
		return nil, entity.NewError("IDEMPOTENCY_ROOT_REQUIRED", "recurrence root is required")
	}
	if n < 1 {
		return nil, entity.NewError("RECURRENCE_INVALID", "recurrence occurrences must be positive")
	}
	keys := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		keys = append(keys, root+":"+strconv.Itoa(i))
	}
	return keys, nil
}
