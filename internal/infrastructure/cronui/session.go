package cronui

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// sessionPayload is the signed UI session body. It carries only the opaque
// operator subject and the expiry; scope is fixed to cron:read by the admin
// wrapper that consumes these tokens, never by the token itself.
type sessionPayload struct {
	Subject   string `json:"sub"`
	ExpiresAt int64  `json:"exp"`
}

// SessionTokens mints and verifies short-lived UI session tokens with
// HMAC-SHA256. It is deliberately seam-agnostic (plain types only) so this
// infrastructure package never imports the interface layer it serves: the
// E14 composition root adapts it to the admin session seam in a few lines.
// Tokens never leave the __Host- cookie.
type SessionTokens struct {
	key []byte
}

// NewSessionTokens builds the issuer. Keys shorter than 32 bytes are
// rejected: session keys come from the secrets backend via configuration,
// never from code or defaults.
func NewSessionTokens(key []byte) (*SessionTokens, error) {
	if len(key) < minSessionKeyLen {
		return nil, ErrCronuiSessionKeyMustBeAtLeast32Bytes
	}

	return &SessionTokens{key: append([]byte(nil), key...)}, nil
}

// Issue mints a token for subject expiring no later than notAfter. Callers
// cap notAfter at the session TTL and the source token expiry.
func (s *SessionTokens) Issue(subject string, notAfter time.Time) (string, error) {
	if s == nil || len(s.key) == 0 {
		return "", ErrCronuiSessionIssuerIsNotInitialized
	}

	if subject == "" {
		return "", ErrCronuiSubjectIsRequired
	}

	body, err := jsonparser.Marshal(sessionPayload{Subject: subject, ExpiresAt: notAfter.Unix()})
	if err != nil {
		return "", fmt.Errorf("cronui: encode session: %w", err)
	}

	encoded := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(encoded))

	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

// VerifiedSession is a validated UI session in plain types.
type VerifiedSession struct {
	Subject   string
	ExpiresAt time.Time
}

// Verify authenticates a presented token with constant-time MAC comparison
// and enforces expiry. Any failure is authentication failure without detail.
func (s *SessionTokens) Verify(_ context.Context, token string) (VerifiedSession, error) {
	fail := func() (VerifiedSession, error) {
		return VerifiedSession{}, ErrCronuiBadSessionToken
	}
	if s == nil || len(s.key) == 0 {
		return fail()
	}

	encoded, signature, found := strings.Cut(token, ".")
	if !found || !s.validMAC(encoded, signature) {
		return fail()
	}

	payload, ok := decodeSession(encoded)
	if !ok {
		return fail()
	}

	expiresAt := time.Unix(payload.ExpiresAt, 0)
	if !time.Now().Before(expiresAt) {
		return fail()
	}

	return VerifiedSession{Subject: payload.Subject, ExpiresAt: expiresAt}, nil
}

// validMAC compares the presented signature in constant time.
func (s *SessionTokens) validMAC(encoded, signature string) bool {
	if encoded == "" || signature == "" || strings.Contains(signature, ".") {
		return false
	}

	wantMAC := hmac.New(sha256.New, s.key)
	_, _ = wantMAC.Write([]byte(encoded))

	gotMAC, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return false
	}

	return subtle.ConstantTimeCompare(gotMAC, wantMAC.Sum(nil)) == 1
}

// decodeSession parses the signed payload body.
func decodeSession(encoded string) (sessionPayload, bool) {
	var payload sessionPayload

	body, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return payload, false
	}

	if err := jsonparser.Unmarshal(body, &payload); err != nil {
		return payload, false
	}

	if payload.Subject == "" {
		return payload, false
	}

	return payload, true
}
