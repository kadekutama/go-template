package jwt

import "errors"

// Sentinel errors for JWT issuance and verification. Callers use errors.Is
// to distinguish expired, tampered, and revoked tokens.
var (
	ErrConfigRequired   = errors.New("jwt: config is required")
	ErrKeyRequired      = errors.New("jwt: key provider is required")
	ErrClockRequired    = errors.New("jwt: clock is required")
	ErrNoActiveKey      = errors.New("jwt: no active signing key")
	ErrUnknownKeyID     = errors.New("jwt: unknown key id")
	ErrExpired          = errors.New("jwt: token is expired")
	ErrInvalidSignature = errors.New("jwt: invalid signature")
	ErrInvalidToken     = errors.New("jwt: invalid token")
	ErrInvalidClaims    = errors.New("jwt: invalid claims")
	ErrNotInitialized   = errors.New("jwt: not initialized")
	ErrRefreshReuse     = errors.New("jwt: refresh token reuse detected")
	ErrRefreshRevoked   = errors.New("jwt: refresh family revoked")
	ErrRefreshNotFound  = errors.New("jwt: refresh token not found")
)
