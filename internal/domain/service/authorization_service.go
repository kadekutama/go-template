package service

import (
	"strconv"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// DefaultAuthExpiryDays is the default authorization hold window.
const DefaultAuthExpiryDays = 7

// Authorization is the authorize-now/capture-later workflow aggregate: a
// hold plus capture accounting. It is never a posting.
type Authorization struct {
	ID               string
	AmountMinor      int64
	CapturedMinor    int64
	Status           valueobject.AuthStatus
	ExpiresAt        time.Time
	MultipleCaptures bool
	RequiresAction   bool
	HoldID           string
}

// Authorize places a hold for the full amount with an expiry.
func Authorize(id string, amountMinor int64, holdID string, now time.Time, expiryDays int, multipleCaptures bool) (Authorization, error) {
	if id == "" {
		return Authorization{}, entity.NewError("AUTH_ID_REQUIRED", "authorization id is required")
	}
	if amountMinor <= 0 {
		return Authorization{}, entity.NewError("INVALID_AUTH_AMOUNT", "authorization amount must be positive")
	}
	if holdID == "" {
		return Authorization{}, entity.NewError("AUTH_HOLD_REQUIRED", "authorization requires a hold reference")
	}
	if expiryDays <= 0 {
		expiryDays = DefaultAuthExpiryDays
	}
	return Authorization{
		ID:               id,
		AmountMinor:      amountMinor,
		Status:           valueobject.AuthAuthorized,
		ExpiresAt:        now.Add(time.Duration(expiryDays) * 24 * time.Hour),
		MultipleCaptures: multipleCaptures,
		HoldID:           holdID,
	}, nil
}

// Capture enforces the transition table plus captured_total + capture ≤
// authorized and the single-vs-multiple partial policy.
func Capture(a Authorization, amountMinor int64) (Authorization, error) {
	if amountMinor <= 0 {
		return a, entity.NewError("INVALID_CAPTURE_AMOUNT", "capture amount must be positive")
	}
	target := valueobject.AuthPartiallyCaptured
	if a.CapturedMinor+amountMinor == a.AmountMinor {
		target = valueobject.AuthCaptured
	}
	remaining := a.AmountMinor - a.CapturedMinor
	if amountMinor > remaining {
		return a, &entity.Error{Code: "CAPTURE_EXCEEDS_AUTHORIZED", Message: "capture exceeds authorized amount; remaining=" + strconv.FormatInt(remaining, 10)}
	}
	if !valueobject.CanTransitionAuth(a.Status, target) {
		return a, entity.NewError("AUTH_NOT_CAPTURABLE", "authorization cannot capture in its current status")
	}
	if a.CapturedMinor > 0 && !a.MultipleCaptures {
		return a, entity.NewError("MULTIPLE_CAPTURES_FORBIDDEN", "network forbids multiple partial captures")
	}
	a.CapturedMinor += amountMinor
	a.Status = target
	return a, nil
}

// SweepExpired marks expired authorizations EXPIRED exactly once (idempotent:
// already-EXPIRED/VOIDED/CAPTURED inputs return unchanged with released=false
// unless newly expired).
func SweepExpired(auths []Authorization, now time.Time) (updated []Authorization, released []string) {
	updated = make([]Authorization, 0, len(auths))
	for _, a := range auths {
		if !now.Before(a.ExpiresAt) && valueobject.CanTransitionAuth(a.Status, valueobject.AuthExpired) {
			a.Status = valueobject.AuthExpired
			updated = append(updated, a)
			released = append(released, a.HoldID)
			continue
		}
		updated = append(updated, a)
	}
	return updated, released
}

// RequireAction enters the SCA challenge state from a live authorization.
func RequireAction(a Authorization) (Authorization, error) {
	if !valueobject.CanTransitionAuth(a.Status, valueobject.AuthRequiresAction) {
		return a, entity.NewError("SCA_STATE_INVALID", "challenge can start only from an authorized payment")
	}
	a.Status = valueobject.AuthRequiresAction
	a.RequiresAction = true
	return a, nil
}

// ResolveAction resolves the SCA challenge to succeeded/failed. Resolving an
// authorization that is not awaiting a challenge fails closed.
func ResolveAction(a Authorization, succeeded bool) (Authorization, error) {
	if a.Status != valueobject.AuthRequiresAction || !a.RequiresAction {
		return a, entity.NewError("SCA_STATE_INVALID", "no pending challenge to resolve")
	}
	a.RequiresAction = false
	if succeeded {
		a.Status = valueobject.AuthAuthorized
	} else {
		a.Status = valueobject.AuthVoided
	}
	return a, nil
}
