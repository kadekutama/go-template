package admin

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode"
)

// VerifiedBearer is the wrapper's view of a validated E09 bearer token. Only
// the opaque non-PII subject and scope/expiry facts cross into this package;
// the source token itself is never stored, logged, or set in a cookie.
type VerifiedBearer struct {
	// Subject is the operator's opaque actor ID. It must still pass
	// validActorID before it may form a limiter key or session.
	Subject string
	// Scopes carries the source token scopes (must include cron:read).
	Scopes []string
	// ExpiresAt bounds the down-scoped UI session: it is never later than
	// the source token expiry.
	ExpiresAt time.Time
}

// TokenVerifier validates the E09 bearer JWT carried in the Authorization
// header. The concrete E09 adapter lives behind the composition root.
type TokenVerifier interface {
	VerifyBearer(ctx context.Context, authorizationHeader string) (VerifiedBearer, error)
}

// Session is a validated down-scoped UI session. It carries only the
// cron:read scope by construction.
type Session struct {
	Subject   string
	ExpiresAt time.Time
}

// SessionTokenIssuer mints and verifies the short-lived UI session tokens
// that travel only in the __Host- cookie. Tokens never leave the cookie.
type SessionTokenIssuer interface {
	// IssueReadSession mints a cron:read-only token that expires no later
	// than notAfter.
	IssueReadSession(subject string, notAfter time.Time) (token string, expiresAt time.Time, err error)
	// VerifySessionToken authenticates a presented session token.
	VerifySessionToken(ctx context.Context, token string) (Session, error)
}

// SessionAuthenticator resolves the request session from the cookie. The
// package ships the cookie implementation; the interface stays narrow so
// tests and future transports can substitute it.
type SessionAuthenticator interface {
	Authenticate(r *http.Request) (Session, error)
}

// Authorizer enforces the cron:read-only policy with default deny. Only
// explicitly approved operator principals hold cron:read.
type Authorizer interface {
	Authorize(ctx context.Context, subject, action string) (bool, error)
}

// cookieSessionAuthenticator is the standard cookie-bound implementation.
type cookieSessionAuthenticator struct {
	issuer SessionTokenIssuer
	cookie string
}

// NewCookieSessionAuthenticator binds session resolution to the session
// cookie and the issuer seam.
func NewCookieSessionAuthenticator(issuer SessionTokenIssuer, cookieName string) SessionAuthenticator {
	return &cookieSessionAuthenticator{issuer: issuer, cookie: cookieName}
}

// Authenticate reads the session cookie and verifies it through the issuer.
// Any failure is authentication failure; callers map it to 401 without
// details and without reaching upstream.
func (a *cookieSessionAuthenticator) Authenticate(r *http.Request) (Session, error) {
	if a.issuer == nil || a.cookie == "" {
		return Session{}, errNoSession
	}

	cookie, err := r.Cookie(a.cookie)
	if err != nil || cookie.Value == "" {
		return Session{}, errNoSession
	}

	session, err := a.issuer.VerifySessionToken(r.Context(), cookie.Value)
	if err != nil {
		return Session{}, errNoSession
	}

	return session, nil
}

// ErrAdminNoSession marks absent or unverifiable sessions.
var ErrAdminNoSession = errors.New("admin: no session")

// errNoSession marks absent or unverifiable sessions. It carries no detail.
var errNoSession = ErrAdminNoSession

// maxActorIDLength bounds opaque actor IDs in key segments (E05 rule).
const maxActorIDLength = 128

// validActorID reports whether an authenticated actor ID may form a limiter
// key segment: stable, opaque, non-PII, already canonical, 1–128 bytes, with
// no colon, no surrounding whitespace, and no control characters. Anything
// else must use only the fixed anonymous limiter key and be denied before any
// admin action; values are never truncated, escaped, or repaired.
func validActorID(subject string) bool {
	if len(subject) == 0 || len(subject) > maxActorIDLength {
		return false
	}

	if strings.Contains(subject, ":") {
		return false
	}

	if strings.TrimSpace(subject) != subject {
		return false
	}

	for _, r := range subject {
		if unicode.IsControl(r) {
			return false
		}
	}

	return true
}

// hasScope reports whether the bearer carries a scope.
func hasScope(scopes []string, scope string) bool {
	return slices.Contains(scopes, scope)
}
