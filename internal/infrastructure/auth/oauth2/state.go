package oauth2

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// statePayload is the signed login handshake: tenant|provider|nonce|exp.
func signState(secret []byte, tenant, provider, nonce string, exp time.Time) string {
	payload := strings.Join([]string{tenant, provider, nonce, strconv.FormatInt(exp.Unix(), 10)}, "|")
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))

	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// verifyState checks HMAC, tenant/provider binding, and expiry via clock.
func verifyState(secret []byte, state, wantTenant, wantProvider string, now time.Time) error {
	parts := strings.Split(state, ".")
	if len(parts) != 2 {
		return ErrStateInvalid
	}

	rawPayload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return fmt.Errorf("%w: decode", ErrStateInvalid)
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("%w: decode", ErrStateInvalid)
	}

	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(rawPayload)

	if !hmac.Equal(mac.Sum(nil), sig) {
		return ErrStateInvalid
	}

	fields := strings.Split(string(rawPayload), "|")
	if len(fields) != 4 {
		return ErrStateInvalid
	}

	if fields[0] != wantTenant || fields[1] != wantProvider {
		return ErrStateInvalid
	}

	expUnix, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		return fmt.Errorf("%w: expiry", ErrStateInvalid)
	}

	if now.After(time.Unix(expUnix, 0)) {
		return ErrStateExpired
	}

	return nil
}

// newVerifier returns one PKCE code verifier (43-128 chars base64url).
func newVerifier() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf[:]), nil
}

// challengeS256 maps one verifier onto its S256 challenge.
func challengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))

	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// newNonce returns one random handshake nonce.
func newNonce() string {
	var buf [16]byte

	_, _ = rand.Read(buf[:])

	return base64.RawURLEncoding.EncodeToString(buf[:])
}
