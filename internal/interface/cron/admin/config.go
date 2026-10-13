// Package admin implements the read-only, authenticated internal operations
// surface for the scheduler dashboard (E09.1-T02). It wraps the pinned
// upstream gocron-ui handler with a strict method/path allow-list,
// same-origin enforcement, short-lived down-scoped sessions, and the shared
// Valkey rate limiter. Scheduler mutations are denied before they can reach
// the upstream handler; this package never starts or stops a scheduler.
package admin

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// Config wires the admin handler and its dedicated internal listener.
// The zero value is disabled; E14 owns starting and shutting down the
// listener built from this config in cmd/cron.
//
// Every policy value is explicit configuration when enabled — there are no
// defaults. Validation preserves the security invariants instead: the scope
// stays read-only, the cookie keeps its __Host- protections, sessions stay
// short-lived, binds stay loopback without private-listener mode, and
// limiter keys stay canonical.
type Config struct {
	// Enabled constructs the upstream UI only when true. Disabled yields
	// nil handler/server without invoking the upstream factory.
	Enabled bool
	// BindAddr is the listener host:port, required when enabled. Loopback
	// is the safe choice; any non-loopback bind requires PrivateListener
	// with verified mTLS client certificates.
	BindAddr string
	// PrivateListener opts into non-loopback bind for a private network.
	// It requires TLS client-certificate verification and the E17 private
	// network policy; public ingress must never route here.
	PrivateListener bool
	// AllowedOrigin is the dedicated exact HTTPS origin of the UI
	// (separate from the public API origin), e.g.
	// https://cron-admin.internal:8443.
	AllowedOrigin string
	// TLSConfig secures the dedicated listener. Required when enabled.
	TLSConfig *tls.Config
	// RateLimitBudget and RateLimitWindow bound UI requests through the
	// shared E08-T07 limiter. Both must be positive.
	RateLimitBudget int64
	RateLimitWindow time.Duration
	// SessionMaxTTL caps issued UI session lifetime, required when enabled.
	// Larger values are rejected so the packet's short-lived-session
	// invariant (at most 5 minutes) always holds.
	SessionMaxTTL time.Duration
	// SessionScope is the only scope UI sessions carry, required when
	// enabled. Only cron:read is supported (any future privileged scope
	// needs a separate contract), so other values are rejected.
	SessionScope string
	// SessionCookieName is the session cookie, required when enabled. It
	// must keep the __Host- prefix so browser prefix protections (Secure,
	// no Domain, Path=/) keep applying.
	SessionCookieName string
	// LimiterNamespace scopes the E05 rate-limit keys, required when
	// enabled. It must be a canonical key segment; changing it moves all UI
	// limiter keys.
	LimiterNamespace string
	// AnonymousSubject is the anonymous limiter bucket ID, required when
	// enabled. It must be a canonical key segment.
	AnonymousSubject string
}

// Validate rejects unsafe or incomplete listener configuration. Disabled
// config needs nothing and validates clean.
func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	if _, err := c.originURL(); err != nil {
		return err
	}

	if c.TLSConfig == nil {
		return ErrAdminTLSConfigIsRequiredWhenEnabled
	}

	if err := c.validateBind(); err != nil {
		return err
	}

	return c.validatePolicy()
}

// isLoopbackHost reports whether a bind host is loopback-only.
func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}

	ip := net.ParseIP(host)

	return ip != nil && ip.IsLoopback()
}

// originURL parses the configured exact origin.
func (c Config) originURL() (*url.URL, error) {
	u, err := url.Parse(c.AllowedOrigin)
	if err != nil {
		return nil, fmt.Errorf("admin: invalid allowed origin: %w", err)
	}

	if u.Scheme != "https" || u.Host == "" {
		return nil, ErrAdminAllowedOriginMustBeAnExactHTTPSURLWithAHost
	}

	return u, nil
}

// validateBind requires an explicit host:port and enforces loopback unless
// private-listener mode with mTLS client certificates is configured.
func (c Config) validateBind() error {
	if c.BindAddr == "" {
		return ErrAdminBindAddressIsRequiredWhenEnabled
	}

	host, _, err := net.SplitHostPort(c.BindAddr)
	if err != nil {
		return fmt.Errorf("admin: bind address must be host:port: %w", err)
	}

	if isLoopbackHost(host) {
		return nil
	}

	if !c.PrivateListener {
		return ErrAdminNonLoopbackBindRequiresPrivateListenerMode
	}

	if c.TLSConfig.ClientAuth != tls.RequireAndVerifyClientCert || c.TLSConfig.ClientCAs == nil {
		return ErrAdminPrivateListenerModeRequiresVerifiedMTLSClientCertifi
	}

	return nil
}

// validatePolicy enforces positive limiter budgets and the required policy
// values with their invariants. The 5-minute session ceiling and the
// cron:read-only scope come from the packet; validation below keeps them
// mechanical rather than conventional.
func (c Config) validatePolicy() error {
	if err := c.validateQuotas(); err != nil {
		return err
	}

	if c.SessionScope == "" {
		return ErrAdminSessionScopeIsRequiredWhenEnabled
	}

	if c.SessionScope != "cron:read" {
		return ErrAdminOnlyTheCronReadScopeIsSupported
	}

	if c.SessionCookieName == "" {
		return ErrAdminSessionCookieNameIsRequiredWhenEnabled
	}

	if !strings.HasPrefix(c.SessionCookieName, "__Host-") {
		return ErrAdminSessionCookieMustKeepTheHostPrefix
	}

	if c.LimiterNamespace == "" {
		return ErrAdminLimiterNamespaceIsRequiredWhenEnabled
	}

	if !validActorID(c.LimiterNamespace) {
		return ErrAdminLimiterNamespaceMustBeACanonicalKeySegment
	}

	if c.AnonymousSubject == "" {
		return ErrAdminAnonymousSubjectIsRequiredWhenEnabled
	}

	if !validActorID(c.AnonymousSubject) {
		return ErrAdminAnonymousSubjectMustBeACanonicalKeySegment
	}

	return nil
}

// validateQuotas enforces positive limiter budgets and the session TTL cap:
// sessions stay short-lived at no more than 5 minutes.
func (c Config) validateQuotas() error {
	if c.RateLimitBudget <= 0 {
		return ErrAdminRateLimitBudgetMustBePositive
	}

	if c.RateLimitWindow <= 0 {
		return ErrAdminRateLimitWindowMustBePositive
	}

	if c.SessionMaxTTL <= 0 || c.SessionMaxTTL > 5*time.Minute {
		return ErrAdminSessionMaxTTLMustBeWithin05m
	}

	return nil
}
