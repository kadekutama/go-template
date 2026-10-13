package admin

import (
	"bufio"
	"errors"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	appport "github.com/kadekutama/go-template/internal/application/port"
	domainservice "github.com/kadekutama/go-template/internal/domain/service"
)

// denyMethods lists the state-changing methods denied on scheduler surfaces.
// GET shapes are registered separately as forwards; everything else on those
// shapes — including exotic methods via the error handler — is denied.
func denyMethods() []string {
	return []string{
		http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodHead, http.MethodOptions,
	}
}

// allMethods covers every standard method for shapes denied regardless of
// method (run-now, scheduler start/stop).
func allMethods() []string {
	return []string{
		http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodHead, http.MethodOptions,
	}
}

// sessionContextKey carries the resolved actor through the middleware chain.
const sessionContextKey = "admin.session"

// actorState is the resolved request actor: validated subject when the
// session verifies, otherwise anonymous.
type actorState struct {
	subject       string
	authenticated bool
	valid         bool
}

// isAPIPath reports whether denials for the path are audited. Every denied
// /api/* request records exactly one audit event; other surfaces deny
// silently with a generic status.
func isAPIPath(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}

// isControlShape reports whether method+path is a scheduler-control attempt
// (a mutation in any method, or an unknown /api/* path) as opposed to an
// allowed read. Reads, UI assets, and the WebSocket feed are not control
// shapes. Denied control attempts are always audited; denied reads fail
// with plain statuses.
func isControlShape(method, path string) bool {
	if !isAPIPath(path) {
		return false
	}

	if method != http.MethodGet {
		return true
	}

	switch path {
	case "/api/config", "/api/jobs":
		return false
	}

	rest, ok := strings.CutPrefix(path, "/api/jobs/")
	if !ok {
		return true
	}
	// Only a bare single-segment job id is a read; deeper shapes (run-now,
	// subpaths) and the empty id are control/unknown shapes. The delimiter
	// presence matters: without it, "a\x00b"-style component pairs could
	// alias, and multi-segment paths would read as allowed.
	segment, _, hasMore := strings.Cut(rest, "/")

	return segment == "" || hasMore
}

// adminHandler is the authenticated read-only wrapper around the upstream
// dashboard handler. It holds no scheduler, cache, or policy state beyond
// its narrow seams.
type adminHandler struct {
	origin       string
	originHost   string
	upstream     http.Handler
	sessions     SessionAuthenticator
	tokens       TokenVerifier
	sessionToken SessionTokenIssuer
	authorizer   Authorizer
	audit        AuditLogger
	limiter      appport.RateLimiter
	budget       int64
	window       time.Duration
	sessionCap   time.Duration
	scope        string
	cookie       string
	namespace    string
	anonymous    string
	anonymousKey string
}

// NewAdminHandler builds the read-only wrapper on the repository Echo
// router and returns it as an http.Handler. A disabled config yields
// ErrAdminDisabled without invoking upstreamFactory, so the upstream
// broadcaster goroutine is never started. All seams are required when
// enabled.
func NewAdminHandler(
	cfg Config,
	upstreamFactory func() (http.Handler, error),
	sessions SessionAuthenticator,
	tokens TokenVerifier,
	sessionTokens SessionTokenIssuer,
	authorizer Authorizer,
	audit AuditLogger,
	limiter appport.RateLimiter,
) (http.Handler, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if !cfg.Enabled {
		return nil, ErrAdminDisabled
	}

	if err := checkAdminSeams(upstreamFactory, sessions, tokens, sessionTokens, authorizer, audit, limiter); err != nil {
		return nil, err
	}

	origin, err := cfg.originURL()
	if err != nil {
		return nil, err
	}

	upstream, err := upstreamFactory()
	if err != nil {
		return nil, err
	}

	if upstream == nil {
		return nil, ErrAdminUpstreamFactoryReturnedNoHandler
	}

	anonymousKey, err := domainservice.BuildRateLimitKey(cfg.LimiterNamespace, cfg.AnonymousSubject)
	if err != nil {
		return nil, err
	}

	h := &adminHandler{
		origin:       cfg.AllowedOrigin,
		originHost:   origin.Host,
		upstream:     upstream,
		sessions:     sessions,
		tokens:       tokens,
		sessionToken: sessionTokens,
		authorizer:   authorizer,
		audit:        audit,
		limiter:      limiter,
		budget:       cfg.RateLimitBudget,
		window:       cfg.RateLimitWindow,
		sessionCap:   cfg.SessionMaxTTL,
		scope:        cfg.SessionScope,
		cookie:       cfg.SessionCookieName,
		namespace:    cfg.LimiterNamespace,
		anonymous:    cfg.AnonymousSubject,
		anonymousKey: anonymousKey,
	}

	e := echo.New()
	e.HTTPErrorHandler = h.httpError
	e.Use(h.hardening, h.hostCheck)
	e.POST("/admin/session", h.bootstrap)
	e.DELETE("/admin/session", h.logout, h.sessionResolve, h.limitEnforce)

	protected := e.Group("", h.sessionResolve, h.originCheck, h.limitEnforce, h.authorize)
	protected.GET("/api/config", h.forward)
	protected.GET("/api/jobs", h.forward)
	protected.GET("/api/jobs/:id", h.forward)
	protected.GET("/", h.forward)
	protected.GET("/*", h.staticOrDeny)
	wss := e.Group("", h.sessionResolve, h.requireOrigin, h.limitEnforce, h.authorize)
	wss.GET("/ws", h.forward)
	protected.Match(denyMethods(), "/api/config", h.denyMutation)
	protected.Match(denyMethods(), "/api/jobs", h.denyMutation)
	protected.Match(denyMethods(), "/api/jobs/:id", h.denyMutation)
	protected.Match(allMethods(), "/api/jobs/:id/run", h.denyMutation)
	protected.Match(allMethods(), "/api/scheduler/start", h.denyMutation)
	protected.Match(allMethods(), "/api/scheduler/stop", h.denyMutation)
	e.RouteNotFound("/*", h.denyNotFound, h.sessionResolve, h.limitEnforce)

	return e, nil
}

// checkAdminSeams rejects missing collaborators before the upstream
// factory runs, so a half-wired handler can never serve traffic.
func checkAdminSeams(
	upstreamFactory func() (http.Handler, error),
	sessions SessionAuthenticator,
	tokens TokenVerifier,
	sessionTokens SessionTokenIssuer,
	authorizer Authorizer,
	audit AuditLogger,
	limiter appport.RateLimiter,
) error {
	if upstreamFactory == nil {
		return ErrAdminUpstreamFactoryIsRequired
	}

	if sessions == nil || tokens == nil || sessionTokens == nil {
		return ErrAdminSessionSeamsAreRequired
	}

	if authorizer == nil {
		return ErrAdminAuthorizerIsRequired
	}

	if audit == nil {
		return ErrAdminAuditLoggerIsRequired
	}

	if limiter == nil {
		return ErrAdminRateLimiterIsRequired
	}

	return nil
}

// NewAdminServer builds the dedicated internal HTTPS listener. E14-T01 owns
// starting and shutting it down with the cron process lifecycle. A disabled
// config (or nil handler) yields ErrAdminDisabled instead of a server.
func NewAdminServer(handler http.Handler, cfg Config) (*http.Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if !cfg.Enabled || handler == nil {
		return nil, ErrAdminDisabled
	}

	return &http.Server{
		Addr:              cfg.BindAddr,
		Handler:           handler,
		TLSConfig:         cfg.TLSConfig,
		ReadHeaderTimeout: readHeaderTimeout10 * time.Second,
	}, nil
}

// setHardeningHeaders applies baseline response hardening. Bodies stay
// generic; nothing sensitive is ever rendered.
func setHardeningHeaders(header http.Header) {
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Frame-Options", "DENY")
}

// WriteHeader strips CORS headers before the status line is written.
func (w *corsStripWriter) WriteHeader(code int) {
	w.strip()
	w.ResponseWriter.WriteHeader(code)
}

// Write strips CORS headers before the first body bytes. A direct Write
// without a prior WriteHeader implicitly sends 200 on the underlying
// writer, which would otherwise bypass stripping.
func (w *corsStripWriter) Write(body []byte) (int, error) {
	w.strip()

	return w.ResponseWriter.Write(body)
}

// Hijack passes WebSocket upgrades to the underlying connection.
func (w *corsStripWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, ErrAdminHijackNotSupported
	}

	return hijacker.Hijack()
}

// hardening sets response hardening headers on every response.
func (h *adminHandler) hardening(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		setHardeningHeaders(c.Response().Header())

		return next(c)
	}
}

// hostCheck rejects requests whose Host is not the exact configured origin
// host before any session, limiter, or upstream work.
func (h *adminHandler) hostCheck(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if c.Request().Host != h.originHost {
			return c.String(http.StatusForbidden, "forbidden")
		}

		return next(c)
	}
}

// checkOrigin enforces the exact same-origin policy. Browser state-changing
// and WebSocket requests must carry the exact origin; plain GET navigations
// without an Origin header are allowed through to session auth.
func (h *adminHandler) checkOrigin(r *http.Request, requireOrigin bool) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return !requireOrigin
	}

	return origin == h.origin
}

// stateOf returns the stashed actor, defaulting to anonymous.
func stateOf(c *echo.Context) actorState {
	if state, ok := c.Get(sessionContextKey).(actorState); ok {
		return state
	}

	return actorState{}
}

// originCheck rejects reads carrying a foreign Origin header. Mutations
// and unknown routes intentionally skip this middleware to preserve the
// deny-before-upstream audit order: they fail closed with an audit event
// instead.
func (h *adminHandler) originCheck(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if !h.checkOrigin(c.Request(), false) {
			return c.String(http.StatusForbidden, "forbidden")
		}

		return next(c)
	}
}

// requireOrigin rejects WebSocket upgrades without the exact origin. Plain
// reads allow originless navigation; upgrades never occur without a browser
// Origin header.
func (h *adminHandler) requireOrigin(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if !h.checkOrigin(c.Request(), true) {
			return c.String(http.StatusForbidden, "forbidden")
		}

		return next(c)
	}
}

// sessionResolve verifies the session cookie and stashes the actor for
// downstream middleware and handlers. It never writes a response: denials
// stay route-aware (audited for /api/*, silent otherwise).
func (h *adminHandler) sessionResolve(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		state := actorState{}
		if session, err := h.sessions.Authenticate(c.Request()); err == nil {
			state = actorState{subject: session.Subject, authenticated: true, valid: validActorID(session.Subject)}
		}

		c.Set(sessionContextKey, state)

		return next(c)
	}
}

// limitEnforce consults the shared limiter once per request, after Host/Origin
// validation and session resolution. Denial returns 429 with Retry-After and
// store failures return 503; neither reaches the authorizer or upstream, and
// the authorizer is not consulted on denial.
func (h *adminHandler) limitEnforce(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		state := stateOf(c)
		key, _ := h.actorKey(state.subject)

		allowed, retryAfter, storeErr := h.checkLimit(c.Request(), key)
		if storeErr || !allowed {
			actor := h.anonymous
			if state.valid {
				actor = state.subject
			}

			if isControlShape(c.Request().Method, c.Request().URL.Path) {
				h.denyLimitedAPI(c.Response(), c.Request(), actor, retryAfter, storeErr)

				return nil
			}

			if storeErr {
				return c.String(http.StatusServiceUnavailable, "temporarily unavailable")
			}

			denyLimited(c.Response(), retryAfter)

			return nil
		}

		return next(c)
	}
}

// denyLimited writes 429 with Retry-After for limiter denials.
func denyLimited(w http.ResponseWriter, retryAfter time.Duration) {
	secs := max(int(math.Ceil(retryAfter.Seconds())), 1)
	w.Header().Set("Retry-After", strconv.Itoa(secs))
	http.Error(w, "rate limited", http.StatusTooManyRequests)
}

// authorize enforces the cron:read-only policy with default deny. Requests
// without a usable session pass through so route handlers can deny them
// with route-appropriate audit behavior; only explicitly approved operator
// principals hold cron:read. Scope denials on control shapes are audited;
// denied reads fail with plain statuses.
func (h *adminHandler) authorize(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		state := stateOf(c)
		if !state.authenticated || !state.valid {
			return next(c)
		}

		allowed, err := h.authorizer.Authorize(c.Request().Context(), state.subject, h.scope)
		if err != nil || !allowed {
			if isControlShape(c.Request().Method, c.Request().URL.Path) {
				h.denyAPI(c.Response(), c.Request(), state.subject, http.StatusForbidden, ReasonForbidden, "forbidden")

				return nil
			}

			return c.String(http.StatusForbidden, "forbidden")
		}

		return next(c)
	}
}

// actorKey resolves the limiter key: the canonical actor ID when valid,
// otherwise only the fixed anonymous key. Callers deny invalid actors after
// the limiter call so malformed subjects are still charged to the anonymous
// bucket and never reach an admin action.
func (h *adminHandler) actorKey(subject string) (string, bool) {
	if validActorID(subject) {
		key, err := domainservice.BuildRateLimitKey(h.namespace, subject)
		if err == nil {
			return key, true
		}
	}

	return h.anonymousKey, false
}

// checkLimit queries the shared limiter without writing a response. Callers
// map store errors to 503 and denials to 429 with Retry-After, auditing
// mutation routes before writing the denial so every denied /api/* request
// records exactly one event and every denial writes exactly one response.
func (h *adminHandler) checkLimit(r *http.Request, key string) (bool, time.Duration, bool) {
	decision, err := h.limiter.Allow(r.Context(), key, h.budget, h.window)
	if err != nil {
		return false, 0, true
	}

	return decision.Allowed, decision.RetryAfter, false
}

// denyAPI denies an /api/* request, auditing exactly one event first. Audit
// failure fails closed with 500 instead of the denial.
func (h *adminHandler) denyAPI(w http.ResponseWriter, r *http.Request, actor string, status int, reason, body string) {
	if !h.auditDenied(w, r, actor, reason) {
		return
	}

	http.Error(w, body, status)
}

// denyLimitedAPI maps a limiter outcome for an /api/* request: audit first,
// then exactly one of 500 (audit failure), 503, or 429.
func (h *adminHandler) denyLimitedAPI(w http.ResponseWriter, r *http.Request, actor string, retryAfter time.Duration, storeErr bool) {
	reason := ReasonRateLimited
	if storeErr {
		reason = ReasonLimiterUnavailable
	}

	if !h.auditDenied(w, r, actor, reason) {
		return
	}

	if storeErr {
		http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)

		return
	}

	denyLimited(w, retryAfter)
}

// auditDenied records one rejected control attempt without credentials or
// request secrets. Audit failure fails closed before any upstream call.
func (h *adminHandler) auditDenied(w http.ResponseWriter, r *http.Request, actor, reason string) bool {
	err := h.audit.LogDeniedControl(r.Context(), DeniedControlEvent{
		OccurredAt: time.Now(),
		Actor:      actor,
		Method:     r.Method,
		Path:       r.URL.Path,
		Reason:     reason,
	})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)

		return false
	}

	return true
}

// admitBootstrap maps a limiter outcome on a bootstrap denial path: store
// error → 503, denial → 429 with Retry-After, admission → the caller writes
// its own denial. Exactly one Allow call happens per bootstrap attempt on
// every path. It returns false when it wrote a response.
func (h *adminHandler) admitBootstrap(c *echo.Context, key string) bool {
	allowed, retryAfter, storeErr := h.checkLimit(c.Request(), key)
	if storeErr {
		_ = c.String(http.StatusServiceUnavailable, "temporarily unavailable")

		return false
	}

	if !allowed {
		denyLimited(c.Response(), retryAfter)

		return false
	}

	return true
}

// bootstrapTTL bounds the session lifetime by the configured cap and the
// source token expiry. It reports false when nothing issuable remains.
func (h *adminHandler) bootstrapTTL(sourceExpiry time.Time) (time.Duration, bool) {
	ttl := min(time.Until(sourceExpiry), h.sessionCap)
	if ttl <= 0 {
		return 0, false
	}

	return ttl, true
}

// denyBootstrapUnauthorized meters a failed bearer verification against
// the fixed anonymous bucket, then denies. Limiter outages fail closed.
func (h *adminHandler) denyBootstrapUnauthorized(w http.ResponseWriter, r *http.Request) {
	allowed, retryAfter, storeErr := h.checkLimit(r, h.anonymousKey)
	if storeErr {
		http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)

		return
	}

	if !allowed {
		denyLimited(w, retryAfter)

		return
	}

	http.Error(w, "unauthorized", http.StatusUnauthorized)
}

// bootstrap validates an E09 bearer JWT and mints only a down-scoped
// cron:read UI session in the __Host- cookie. The source token is never
// stored, logged, or returned in the body.
func (h *adminHandler) bootstrap(c *echo.Context) error {
	r := c.Request()
	if !h.checkOrigin(r, true) {
		return c.String(http.StatusForbidden, "forbidden")
	}

	bearer, err := h.tokens.VerifyBearer(r.Context(), r.Header.Get("Authorization"))
	if err == nil {
		return h.serveBootstrapAuthenticated(c, r, bearer)
	}

	// Authentication failure is exactly what the fixed anonymous bucket
	// exists to meter: charge it before denying so bearer-guessing
	// pressure is visible to the shared quota (R05/S10). The denial
	// answers the request, so report success to echo.
	h.denyBootstrapUnauthorized(c.Response(), r)

	return nil
}

// serveBootstrapAuthenticated mints the down-scoped session for a verified
// bearer. It runs only after VerifyBearer succeeds.
func (h *adminHandler) serveBootstrapAuthenticated(c *echo.Context, r *http.Request, bearer VerifiedBearer) error {
	key, valid := h.actorKey(bearer.Subject)
	if !valid || !hasScope(bearer.Scopes, h.scope) {
		if !h.admitBootstrap(c, key) {
			return nil
		}

		return c.String(http.StatusForbidden, "forbidden")
	}

	allowed, retryAfter, storeErr := h.checkLimit(r, key)
	if storeErr {
		return c.String(http.StatusServiceUnavailable, "temporarily unavailable")
	}

	if !allowed {
		denyLimited(c.Response(), retryAfter)

		return nil
	}

	allowedScope, err := h.authorizer.Authorize(r.Context(), bearer.Subject, h.scope)
	if err != nil || !allowedScope {
		return c.String(http.StatusForbidden, "forbidden")
	}

	ttl, ok := h.bootstrapTTL(bearer.ExpiresAt)
	if !ok {
		return c.String(http.StatusForbidden, "forbidden")
	}

	expiresAt := time.Now().Add(ttl)

	token, _, err := h.sessionToken.IssueReadSession(bearer.Subject, expiresAt)
	if err != nil {
		return c.String(http.StatusInternalServerError, "internal error")
	}

	http.SetCookie(c.Response(), &http.Cookie{
		Name:     h.cookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		Expires:  expiresAt,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	return c.NoContent(http.StatusNoContent)
}

// logout clears the session cookie. It requires the exact origin so a
// cross-origin page cannot end another operator's session.
func (h *adminHandler) logout(c *echo.Context) error {
	r := c.Request()
	if !h.checkOrigin(r, true) {
		return c.String(http.StatusForbidden, "forbidden")
	}

	state := stateOf(c)
	if !state.authenticated || !state.valid {
		return c.String(http.StatusUnauthorized, "unauthorized")
	}

	http.SetCookie(c.Response(), &http.Cookie{
		Name:     h.cookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	return c.NoContent(http.StatusNoContent)
}

// forward passes a validated read to upstream with the Origin header removed
// and upstream wildcard CORS response headers suppressed. Upstream WebSocket
// origin checks are never relied upon: exact Host/Origin validation already
// ran in middleware.
func (h *adminHandler) forward(c *echo.Context) error {
	state := stateOf(c)
	if !state.authenticated || !state.valid {
		return c.String(http.StatusUnauthorized, "unauthorized")
	}

	r := c.Request()
	clone := r.Clone(r.Context())
	clone.Header.Del("Origin")
	clone.RequestURI = ""
	h.upstream.ServeHTTP(&corsStripWriter{ResponseWriter: c.Response()}, clone)

	return nil
}

// denyMutation rejects scheduler control attempts with exactly one audit
// event. Requests without a usable session are denied as unauthenticated;
// anything else reaching here passed authorization and is forbidden.
func (h *adminHandler) denyMutation(c *echo.Context) error {
	state := stateOf(c)
	if !state.authenticated || !state.valid {
		h.denyAPI(c.Response(), c.Request(), h.anonymous, http.StatusUnauthorized, ReasonAuthRequired, "unauthorized")

		return nil
	}

	h.denyAPI(c.Response(), c.Request(), state.subject, http.StatusForbidden, ReasonForbidden, "forbidden")

	return nil
}

// staticOrDeny serves embedded UI assets and denies unknown /api/* reads.
// It runs after authentication, limiting, and authorization.
func (h *adminHandler) staticOrDeny(c *echo.Context) error {
	state := stateOf(c)
	if !state.authenticated || !state.valid {
		if isAPIPath(c.Request().URL.Path) {
			h.denyAPI(c.Response(), c.Request(), h.anonymous, http.StatusUnauthorized, ReasonAuthRequired, "unauthorized")

			return nil
		}

		return c.String(http.StatusUnauthorized, "unauthorized")
	}

	if isAPIPath(c.Request().URL.Path) {
		h.denyAPI(c.Response(), c.Request(), state.subject, http.StatusNotFound, ReasonUnknownRoute, "not found")

		return nil
	}

	return h.forward(c)
}

// denyNotFound denies unmatched routes: unknown /api/* paths with one audit
// event, everything else silently. It runs after session resolution and
// limiting via its own middleware chain.
func (h *adminHandler) denyNotFound(c *echo.Context) error {
	state := stateOf(c)
	if isAPIPath(c.Request().URL.Path) {
		actor := h.anonymous
		if state.valid {
			actor = state.subject
		}

		status := http.StatusNotFound
		reason := ReasonUnknownRoute
		body := "not found"

		if !state.authenticated || !state.valid {
			status = http.StatusUnauthorized
			reason = ReasonAuthRequired
			body = "unauthorized"
		}

		h.denyAPI(c.Response(), c.Request(), actor, status, reason, body)

		return nil
	}

	return c.String(http.StatusForbidden, "forbidden")
}

// httpError maps router-level failures (method mismatches on known shapes)
// to the deny matrix: 403 with an audit event for /api/* paths, silent 403
// elsewhere, generic 500 for anything unexpected.
func (h *adminHandler) httpError(c *echo.Context, err error) {
	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) && httpErr.Code == http.StatusMethodNotAllowed {
		path := c.Request().URL.Path
		if isAPIPath(path) {
			state := stateOf(c)

			actor := h.anonymous
			if state.valid {
				actor = state.subject
			}

			h.denyAPI(c.Response(), c.Request(), actor, http.StatusForbidden, ReasonForbidden, "forbidden")

			return
		}

		_ = c.String(http.StatusForbidden, "forbidden")

		return
	}

	_ = c.String(http.StatusInternalServerError, "internal error")
}

// corsStripWriter removes upstream Access-Control-* response headers so the
// upstream wildcard CORS policy cannot authorize cross-origin access. It
// passes Hijack through for WebSocket upgrades.
type corsStripWriter struct {
	http.ResponseWriter

	stripped bool
}

// strip removes CORS headers once, before any status line or body bytes.
func (w *corsStripWriter) strip() {
	if w.stripped {
		return
	}

	w.stripped = true

	header := w.Header()
	for name := range header {
		if strings.HasPrefix(strings.ToLower(name), "access-control-") {
			header.Del(name)
		}
	}
}
