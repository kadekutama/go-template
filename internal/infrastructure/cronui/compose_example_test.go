// Example_assembleAdminUI shows the E14 composition root exactly how to
// assemble the read-only admin surface: a real scheduler feeds the upstream
// factory, narrow adapters bind the production seams, and the admin handler
// wraps it all. No listener starts here; E14-T01 owns starting and shutting
// down the optional internal listener in cmd/cron with the cron process
// lifecycle (disabled by default).
package cronui_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	gocron "github.com/go-co-op/gocron/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/cronui"
	"github.com/kadekutama/go-template/internal/interface/cron/admin"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
)

// sessionAdapter binds the HMAC session tokens to the admin session seam.
// E14 carries an identical adapter (or its own issuer) in cmd/cron.
type sessionAdapter struct {
	tokens *cronui.SessionTokens
}

func (a sessionAdapter) IssueReadSession(subject string, notAfter time.Time) (string, time.Time, error) {
	token, err := a.tokens.Issue(subject, notAfter)
	if err != nil {
		return "", time.Time{}, err
	}

	return token, notAfter, nil
}

func (a sessionAdapter) VerifySessionToken(ctx context.Context, token string) (admin.Session, error) {
	session, err := a.tokens.Verify(ctx, token)
	if err != nil {
		return admin.Session{}, err
	}

	return admin.Session{Subject: session.Subject, ExpiresAt: session.ExpiresAt}, nil
}

// bearerAdapter binds the token port to the admin bearer seam.
type bearerAdapter struct {
	tokens *cronui.BearerTokens
}

func (a bearerAdapter) VerifyBearer(ctx context.Context, header string) (admin.VerifiedBearer, error) {
	bearer, err := a.tokens.VerifyBearer(ctx, header)
	if err != nil {
		return admin.VerifiedBearer{}, err
	}

	return admin.VerifiedBearer{Subject: bearer.Subject, Scopes: bearer.Scopes, ExpiresAt: bearer.ExpiresAt}, nil
}

// allowAuthorizer grants cron:read in this example. Production binds the
// E09 Casbin enforcer with operator policies (E09-T03 owns the model).
type allowAuthorizer struct{}

func (allowAuthorizer) Authorize(context.Context, string, string) (bool, error) {
	return true, nil
}

// discardAudit drops audit events in this example. Production binds the E09
// audit logger so denied controls enter the audit chain.
type discardAudit struct{}

func (discardAudit) LogDeniedControl(context.Context, admin.DeniedControlEvent) error {
	return nil
}

func Example_assembleAdminUI() {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		fmt.Println("scheduler:", false)

		return
	}
	defer func() { _ = scheduler.Shutdown() }()

	upstream, err := cronui.NewUpstreamHandler(scheduler)
	if err != nil || upstream == nil {
		fmt.Println("upstream:", false)

		return
	}

	// The key must satisfy the issuer's own minimum-length guard (32 bytes);
	// production loads it from the secrets backend, never from code. E14 must
	// keep the non-nil check: a nil issuer fails every future bootstrap.
	sessions, err := cronui.NewSessionTokens([]byte("example-operator-session-key-for-cron-ui!!"))
	if err != nil || sessions == nil {
		fmt.Println("sessions:", false)

		return
	}

	issuer := &mockapplication.MockTokenIssuer{}
	issuer.On("Verify", mock.Anything, mock.Anything).Return(appport.TokenClaims{}, assert.AnError)

	bearers, err := cronui.NewBearerTokens(issuer)
	if err != nil || bearers == nil {
		fmt.Println("bearers:", false)

		return
	}

	limiter := &mockapplication.MockRateLimiter{}
	limiter.On("Allow", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		appport.RateLimitDecision{Allowed: true}, nil,
	)

	handler, err := admin.NewAdminHandler(
		admin.Config{
			Enabled:           true,
			BindAddr:          "127.0.0.1:8443",
			AllowedOrigin:     "https://cron-admin.internal:8443",
			TLSConfig:         &tls.Config{},
			RateLimitBudget:   100,
			RateLimitWindow:   time.Minute,
			SessionMaxTTL:     5 * time.Minute,
			SessionScope:      "cron:read",
			SessionCookieName: "__Host-cron_ui_session",
			LimiterNamespace:  "cron-ui",
			AnonymousSubject:  "anonymous",
		},
		func() (http.Handler, error) { return upstream, nil },
		admin.NewCookieSessionAuthenticator(sessionAdapter{tokens: sessions}, "__Host-cron_ui_session"), bearerAdapter{tokens: bearers},
		sessionAdapter{tokens: sessions},
		allowAuthorizer{},
		discardAudit{},
		limiter,
	)
	fmt.Println(handler != nil, err == nil)
	// Output: true true
}
