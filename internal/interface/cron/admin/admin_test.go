package admin_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/interface/cron/admin"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
)

const (
	testOrigin = "https://cron-admin.test:8443"
	testHost   = "cron-admin.test:8443"
)

func testConfig() admin.Config {
	return admin.Config{
		Enabled:           true,
		BindAddr:          "127.0.0.1:8443",
		AllowedOrigin:     testOrigin,
		TLSConfig:         &tls.Config{},
		RateLimitBudget:   100,
		RateLimitWindow:   time.Minute,
		SessionMaxTTL:     5 * time.Minute,
		SessionScope:      "cron:read",
		SessionCookieName: "__Host-cron_ui_session",
		LimiterNamespace:  "cron-ui",
		AnonymousSubject:  "anonymous",
	}
}

// mockIssuer scripts session issuance and verification through testify
// expectations. Sessions are identified by fixed token strings per test.
type mockIssuer struct {
	mock.Mock
}

func (m *mockIssuer) IssueReadSession(subject string, notAfter time.Time) (string, time.Time, error) {
	args := m.Called(subject, notAfter)
	token, _ := args.Get(0).(string)
	expiresAt, _ := args.Get(1).(time.Time)

	return token, expiresAt, args.Error(2)
}

func (m *mockIssuer) VerifySessionToken(ctx context.Context, token string) (admin.Session, error) {
	args := m.Called(ctx, token)
	session, _ := args.Get(0).(admin.Session)

	return session, args.Error(1)
}

// mockVerifier scripts bearer verification through testify expectations.
type mockVerifier struct {
	mock.Mock
}

func (m *mockVerifier) VerifyBearer(ctx context.Context, header string) (admin.VerifiedBearer, error) {
	args := m.Called(ctx, header)
	bearer, _ := args.Get(0).(admin.VerifiedBearer)

	return bearer, args.Error(1)
}

// mockAuthorizer scripts scope decisions through testify expectations.
type mockAuthorizer struct {
	mock.Mock
}

func (m *mockAuthorizer) Authorize(ctx context.Context, subject, action string) (bool, error) {
	args := m.Called(ctx, subject, action)
	allowed, _ := args.Get(0).(bool)

	return allowed, args.Error(1)
}

// mockAudit records denied-control events through testify expectations.
type mockAudit struct {
	mock.Mock
}

func (m *mockAudit) LogDeniedControl(ctx context.Context, event admin.DeniedControlEvent) error {
	args := m.Called(ctx, event)

	return args.Error(0)
}

// mockUpstream records forwarded requests and replays a canned dashboard
// response with a wildcard CORS header (proving the wrapper suppresses it).
type mockUpstream struct {
	mock.Mock

	directWrite bool
}

func (m *mockUpstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.Called(w, r)
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if m.directWrite {
		_, _ = w.Write([]byte(`{"jobs":[]}`))

		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"jobs":[]}`))
}

type fixture struct {
	handler    http.Handler
	issuer     *mockIssuer
	verifier   *mockVerifier
	authorizer *mockAuthorizer
	audit      *mockAudit
	limiter    *mockapplication.MockRateLimiter
	upstream   *mockUpstream
}

func newFixture(t *testing.T, cfg admin.Config) *fixture {
	t.Helper()

	fx := &fixture{
		issuer:     &mockIssuer{},
		verifier:   &mockVerifier{},
		authorizer: &mockAuthorizer{},
		audit:      &mockAudit{},
		limiter:    &mockapplication.MockRateLimiter{},
		upstream:   &mockUpstream{},
	}
	fx.authorizer.On(
		"Authorize",
		mock.Anything,
		"op-reader",
		"cron:read",
	).Return(true, nil)
	fx.authorizer.On(
		"Authorize",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(false, nil)
	fx.audit.On(
		"LogDeniedControl",
		mock.Anything,
		mock.Anything,
	).Return(nil)
	fx.upstream.On(
		"ServeHTTP",
		mock.Anything,
		mock.Anything,
	).Return()

	cookieName := cfg.SessionCookieName
	handler, err := admin.NewAdminHandler(
		cfg,
		func() (http.Handler, error) { return fx.upstream, nil },
		admin.NewCookieSessionAuthenticator(fx.issuer, cookieName),
		fx.verifier,
		fx.issuer,
		fx.authorizer,
		fx.audit,
		fx.limiter,
	)
	require.NoError(t, err)
	require.NotNil(t, handler)
	fx.handler = handler

	return fx
}

func sessionCookie(token string) string {
	return sessionCookieNamed("__Host-cron_ui_session", token)
}

func sessionCookieNamed(name, token string) string {
	return name + "=" + token
}

// stubAllowAll programs the generated limiter mock to admit every check.
func stubAllowAll(fx *fixture) {
	stubLimiter(fx, appport.RateLimitDecision{Allowed: true, Remaining: 99}, nil)
}

// stubLimiter programs the generated limiter mock with one decision,
// replacing any prior expectation so each subtest owns its limiter behavior.
func stubLimiter(fx *fixture, decision appport.RateLimitDecision, err error) {
	fx.limiter.ExpectedCalls = nil
	fx.limiter.On(
		"Allow",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(decision, err)
}

// limiterKeys returns every rate-limit key the generated mock observed, in
// call order.
func limiterKeys(fx *fixture) []string {
	keys := make([]string, 0, len(fx.limiter.Calls))

	for _, call := range fx.limiter.Calls {
		key, _ := call.Arguments.Get(1).(string)
		keys = append(keys, key)
	}

	return keys
}

func doRequest(fx *fixture, method, target, cookie, origin string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), method, target, nil)
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	if origin != "" {
		req.Header.Set("Origin", origin)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	rec := httptest.NewRecorder()
	fx.handler.ServeHTTP(rec, req)

	return rec
}

func seedSession(t *testing.T, fx *fixture, subject string) string {
	t.Helper()

	// The token matches on anything: cookie transport trims surrounding
	// whitespace, so malformed subjects (not tokens) are what the wrapper
	// must reject. The session subject itself is always exact here.
	token := "tok-" + subject
	fx.issuer.On(
		"VerifySessionToken",
		mock.Anything,
		mock.Anything,
	).Return(admin.Session{Subject: subject, ExpiresAt: time.Now().Add(time.Hour)}, nil)

	return token
}

// auditEvents returns every denied-control event the audit mock observed.
func auditEvents(fx *fixture) []admin.DeniedControlEvent {
	events := make([]admin.DeniedControlEvent, 0, len(fx.audit.Calls))

	for _, call := range fx.audit.Calls {
		event, _ := call.Arguments.Get(1).(admin.DeniedControlEvent)
		events = append(events, event)
	}

	return events
}

// upstreamOrigin returns the Origin header the upstream mock observed on
// its first forwarded request, or "unforwarded" when nothing was forwarded.
func upstreamOrigin(fx *fixture) string {
	for _, call := range fx.upstream.Calls {
		if req, ok := call.Arguments.Get(1).(*http.Request); ok {
			return req.Header.Get("Origin")
		}
	}

	return "unforwarded"
}

func TestDisabledByDefault(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		config            admin.Config
		expectHandler     bool
		expectFactoryCall bool
	}

	testCases := []testCase{
		{
			name:              "disabled yields nothing without factory call",
			config:            admin.Config{},
			expectHandler:     false,
			expectFactoryCall: false,
		},
		{
			name:              "enabled constructs handler through factory",
			config:            testConfig(),
			expectHandler:     true,
			expectFactoryCall: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			issuer := &mockIssuer{}
			called := false

			handler, err := admin.NewAdminHandler(
				tc.config,
				func() (http.Handler, error) {
					called = true

					return &mockUpstream{}, nil
				},
				admin.NewCookieSessionAuthenticator(issuer, "__Host-cron_ui_session"),
				&mockVerifier{},
				issuer,
				&mockAuthorizer{},
				&mockAudit{},
				&mockapplication.MockRateLimiter{},
			)
			if tc.expectHandler {
				require.NoError(t, err)
				assert.NotNil(t, handler)
			} else {
				assert.ErrorIs(t, err, admin.ErrAdminDisabled)
				assert.Nil(t, handler)
			}

			assert.Equal(t, tc.expectFactoryCall, called)

			server, err := admin.NewAdminServer(handler, tc.config)
			if tc.expectHandler {
				require.NoError(t, err)
				assert.NotNil(t, server)
			} else {
				assert.ErrorIs(t, err, admin.ErrAdminDisabled)
				assert.Nil(t, server)
			}
		})
	}
}

func TestListenerSecurityConfig(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name        string
		config      admin.Config
		expectError bool
	}

	testCases := []testCase{
		{
			name:        "loopback bind accepted",
			config:      testConfig(),
			expectError: false,
		},
		{
			name: "ipv6 loopback accepted",
			config: func() admin.Config {
				cfg := testConfig()
				cfg.BindAddr = "[::1]:8443"

				return cfg
			}(),
			expectError: false,
		},
		{
			name: "non-loopback without private mode rejected",
			config: func() admin.Config {
				cfg := testConfig()
				cfg.BindAddr = "0.0.0.0:8443"

				return cfg
			}(),
			expectError: true,
		},
		{
			name: "non-loopback private without mtls rejected",
			config: func() admin.Config {
				cfg := testConfig()
				cfg.BindAddr = "10.0.0.5:8443"
				cfg.PrivateListener = true

				return cfg
			}(),
			expectError: true,
		},
		{
			name: "non-loopback private with mtls accepted",
			config: func() admin.Config {
				cfg := testConfig()
				cfg.BindAddr = "10.0.0.5:8443"
				cfg.PrivateListener = true
				cfg.TLSConfig = &tls.Config{
					ClientAuth: tls.RequireAndVerifyClientCert,
					ClientCAs:  x509.NewCertPool(),
				}

				return cfg
			}(),
			expectError: false,
		},
		{
			name: "missing tls rejected",
			config: func() admin.Config {
				cfg := testConfig()
				cfg.TLSConfig = nil

				return cfg
			}(),
			expectError: true,
		},
		{
			name: "non-https origin rejected",
			config: func() admin.Config {
				cfg := testConfig()
				cfg.AllowedOrigin = "http://cron-admin.test:8443"

				return cfg
			}(),
			expectError: true,
		},
		{
			name: "zero budget rejected",
			config: func() admin.Config {
				cfg := testConfig()
				cfg.RateLimitBudget = 0

				return cfg
			}(),
			expectError: true,
		},
		{
			name: "zero window rejected",
			config: func() admin.Config {
				cfg := testConfig()
				cfg.RateLimitWindow = 0

				return cfg
			}(),
			expectError: true,
		},
		{
			name: "session ttl above cap rejected",
			config: func() admin.Config {
				cfg := testConfig()
				cfg.SessionMaxTTL = 6 * time.Minute

				return cfg
			}(),
			expectError: true,
		},
		{
			name: "negative session ttl rejected",
			config: func() admin.Config {
				cfg := testConfig()
				cfg.SessionMaxTTL = -time.Minute

				return cfg
			}(),
			expectError: true,
		},
		{
			name: "lower session ttl accepted",
			config: func() admin.Config {
				cfg := testConfig()
				cfg.SessionMaxTTL = 2 * time.Minute

				return cfg
			}(),
			expectError: false,
		},
		{
			name: "zero session ttl rejected",
			config: func() admin.Config {
				cfg := testConfig()
				cfg.SessionMaxTTL = 0

				return cfg
			}(),
			expectError: true,
		},
		{
			name: "empty bind rejected",
			config: func() admin.Config {
				cfg := testConfig()
				cfg.BindAddr = ""

				return cfg
			}(),
			expectError: true,
		},
		{
			name: "empty scope rejected",
			config: func() admin.Config {
				cfg := testConfig()
				cfg.SessionScope = ""

				return cfg
			}(),
			expectError: true,
		},
		{
			name: "empty cookie rejected",
			config: func() admin.Config {
				cfg := testConfig()
				cfg.SessionCookieName = ""

				return cfg
			}(),
			expectError: true,
		},
		{
			name: "empty namespace rejected",
			config: func() admin.Config {
				cfg := testConfig()
				cfg.LimiterNamespace = ""

				return cfg
			}(),
			expectError: true,
		},
		{
			name: "empty anonymous rejected",
			config: func() admin.Config {
				cfg := testConfig()
				cfg.AnonymousSubject = ""

				return cfg
			}(),
			expectError: true,
		},
		{
			name: "non-cron scope rejected",
			config: func() admin.Config {
				cfg := testConfig()
				cfg.SessionScope = "cron:write"

				return cfg
			}(),
			expectError: true,
		},
		{
			name: "non-host cookie rejected",
			config: func() admin.Config {
				cfg := testConfig()
				cfg.SessionCookieName = "session"

				return cfg
			}(),
			expectError: true,
		},
		{
			name: "colon namespace rejected",
			config: func() admin.Config {
				cfg := testConfig()
				cfg.LimiterNamespace = "cron:ui"

				return cfg
			}(),
			expectError: true,
		},
		{
			name: "blank anonymous rejected",
			config: func() admin.Config {
				cfg := testConfig()
				cfg.AnonymousSubject = "  "

				return cfg
			}(),
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.config.Validate()
			if tc.expectError {
				assert.Error(t, err)

				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestRouteMatrix(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		method           string
		path             string
		sessionSubject   string
		haveSession      bool
		expectedStatus   int
		expectUpstream   bool
		expectedAudits   int
		expectedAuditWhy string
	}

	testCases := []testCase{
		{
			name:           "unauthenticated root denied",
			method:         http.MethodGet,
			path:           "/",
			haveSession:    false,
			expectedStatus: http.StatusUnauthorized,
			expectUpstream: false,
			expectedAudits: 0,
		},
		{
			name:           "unauthenticated api denied",
			method:         http.MethodGet,
			path:           "/api/jobs",
			haveSession:    false,
			expectedStatus: http.StatusUnauthorized,
			expectUpstream: false,
			expectedAudits: 0,
		},
		{
			name:           "reader lists config",
			method:         http.MethodGet,
			path:           "/api/config",
			sessionSubject: "op-reader",
			haveSession:    true,
			expectedStatus: http.StatusOK,
			expectUpstream: true,
			expectedAudits: 0,
		},
		{
			name:           "reader lists jobs",
			method:         http.MethodGet,
			path:           "/api/jobs",
			sessionSubject: "op-reader",
			haveSession:    true,
			expectedStatus: http.StatusOK,
			expectUpstream: true,
			expectedAudits: 0,
		},
		{
			name:           "reader views one job",
			method:         http.MethodGet,
			path:           "/api/jobs/abc-123",
			sessionSubject: "op-reader",
			haveSession:    true,
			expectedStatus: http.StatusOK,
			expectUpstream: true,
			expectedAudits: 0,
		},
		{
			name:             "unauthenticated mutation audited",
			method:           http.MethodPost,
			path:             "/api/jobs",
			haveSession:      false,
			expectedStatus:   http.StatusUnauthorized,
			expectUpstream:   false,
			expectedAudits:   1,
			expectedAuditWhy: admin.ReasonAuthRequired,
		},
		{
			name:             "create job denied and audited",
			method:           http.MethodPost,
			path:             "/api/jobs",
			sessionSubject:   "op-reader",
			haveSession:      true,
			expectedStatus:   http.StatusForbidden,
			expectUpstream:   false,
			expectedAudits:   1,
			expectedAuditWhy: admin.ReasonForbidden,
		},
		{
			name:             "run job denied and audited",
			method:           http.MethodPost,
			path:             "/api/jobs/abc-123/run",
			sessionSubject:   "op-reader",
			haveSession:      true,
			expectedStatus:   http.StatusForbidden,
			expectUpstream:   false,
			expectedAudits:   1,
			expectedAuditWhy: admin.ReasonForbidden,
		},
		{
			name:             "delete job denied and audited",
			method:           http.MethodDelete,
			path:             "/api/jobs/abc-123",
			sessionSubject:   "op-reader",
			haveSession:      true,
			expectedStatus:   http.StatusForbidden,
			expectUpstream:   false,
			expectedAudits:   1,
			expectedAuditWhy: admin.ReasonForbidden,
		},
		{
			name:             "scheduler start denied and audited",
			method:           http.MethodPost,
			path:             "/api/scheduler/start",
			sessionSubject:   "op-reader",
			haveSession:      true,
			expectedStatus:   http.StatusForbidden,
			expectUpstream:   false,
			expectedAudits:   1,
			expectedAuditWhy: admin.ReasonForbidden,
		},
		{
			name:             "scheduler stop denied and audited",
			method:           http.MethodPost,
			path:             "/api/scheduler/stop",
			sessionSubject:   "op-reader",
			haveSession:      true,
			expectedStatus:   http.StatusForbidden,
			expectUpstream:   false,
			expectedAudits:   1,
			expectedAuditWhy: admin.ReasonForbidden,
		},
		{
			name:             "unknown api denied without scheduler touch",
			method:           http.MethodGet,
			path:             "/api/unknown",
			sessionSubject:   "op-reader",
			haveSession:      true,
			expectedStatus:   http.StatusNotFound,
			expectUpstream:   false,
			expectedAudits:   1,
			expectedAuditWhy: admin.ReasonUnknownRoute,
		},
		{
			name:             "multi-segment job path denied as control shape",
			method:           http.MethodGet,
			path:             "/api/jobs/123/subpath",
			sessionSubject:   "op-noscope",
			haveSession:      true,
			expectedStatus:   http.StatusForbidden,
			expectUpstream:   false,
			expectedAudits:   1,
			expectedAuditWhy: admin.ReasonForbidden,
		},
		{
			name:           "reader without scope denied",
			method:         http.MethodGet,
			path:           "/api/jobs",
			sessionSubject: "op-noscope",
			haveSession:    true,
			expectedStatus: http.StatusForbidden,
			expectUpstream: false,
			expectedAudits: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fx := newFixture(t, testConfig())
			stubAllowAll(fx)

			cookie := ""
			if tc.haveSession {
				cookie = sessionCookie(seedSession(t, fx, tc.sessionSubject))
			}

			rec := doRequest(fx, tc.method, "https://"+testHost+tc.path, cookie, "", nil)
			assert.Equal(t, tc.expectedStatus, rec.Code)
			assert.Equal(t, tc.expectUpstream, len(fx.upstream.Calls) == 1)
			assert.Len(t, auditEvents(fx), tc.expectedAudits)

			if tc.expectedAudits == 1 {
				event := auditEvents(fx)[0]
				assert.Equal(t, tc.method, event.Method)
				assert.Equal(t, tc.path, event.Path)
				assert.NotContains(t, event.Actor, "@")

				if tc.expectedAuditWhy != "" {
					assert.Equal(t, tc.expectedAuditWhy, event.Reason)
				}
			}
		})
	}
}

func TestOriginEnforcement(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		method         string
		path           string
		origin         string
		expectUpstream bool
		expectedStatus int
	}

	testCases := []testCase{
		{
			name:           "cross-origin read rejected",
			method:         http.MethodGet,
			path:           "/api/jobs",
			origin:         "https://evil.test",
			expectUpstream: false,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "cross-origin websocket rejected",
			method:         http.MethodGet,
			path:           "/ws",
			origin:         "https://evil.test",
			expectUpstream: false,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "websocket without origin rejected",
			method:         http.MethodGet,
			path:           "/ws",
			origin:         "",
			expectUpstream: false,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "same-origin read allowed",
			method:         http.MethodGet,
			path:           "/api/jobs",
			origin:         testOrigin,
			expectUpstream: true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "originless navigation read allowed",
			method:         http.MethodGet,
			path:           "/api/jobs",
			origin:         "",
			expectUpstream: true,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fx := newFixture(t, testConfig())
			stubAllowAll(fx)
			cookie := sessionCookie(seedSession(t, fx, "op-reader"))

			headers := map[string]string{}
			if tc.path == "/ws" && tc.expectUpstream {
				headers["Upgrade"] = "websocket"
				headers["Connection"] = "Upgrade"
			}

			rec := doRequest(fx, tc.method, "https://"+testHost+tc.path, cookie, tc.origin, headers)
			assert.Equal(t, tc.expectedStatus, rec.Code)
			assert.Equal(t, tc.expectUpstream, len(fx.upstream.Calls) == 1)
			assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))

			if tc.expectUpstream {
				assert.Empty(t, upstreamOrigin(fx))
			}
		})
	}
}

func TestSessionBootstrap(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name               string
		method             string
		path               string
		sessionSubject     string
		bearerSubject      string
		bearerScopes       []string
		bearerErr          bool
		bearerExpiryOffset time.Duration
		origin             string
		sessionTTL         time.Duration
		expectedStatus     int
		expectedCookieName string
		expectCleared      bool
		maxCookieAge       int
	}

	testCases := []testCase{
		{
			name:           "valid bearer mints down-scoped cookie",
			method:         http.MethodPost,
			path:           "/admin/session",
			sessionSubject: "",
			bearerSubject:  "op-reader",
			bearerScopes: []string{
				"cron:read",
			},
			bearerErr:          false,
			bearerExpiryOffset: time.Hour,
			origin:             testOrigin,
			sessionTTL:         0,
			expectedStatus:     http.StatusNoContent,
			expectedCookieName: "__Host-cron_ui_session",
			expectCleared:      false,
			maxCookieAge:       300,
		},
		{
			name:           "configured ttl caps the cookie",
			method:         http.MethodPost,
			path:           "/admin/session",
			sessionSubject: "",
			bearerSubject:  "op-reader",
			bearerScopes: []string{
				"cron:read",
			},
			bearerErr:          false,
			bearerExpiryOffset: time.Hour,
			origin:             testOrigin,
			sessionTTL:         2 * time.Minute,
			expectedStatus:     http.StatusNoContent,
			expectedCookieName: "__Host-cron_ui_session",
			expectCleared:      false,
			maxCookieAge:       120,
		},
		{
			name:               "logout clears cookie",
			method:             http.MethodDelete,
			path:               "/admin/session",
			sessionSubject:     "op-reader",
			bearerSubject:      "",
			bearerScopes:       nil,
			bearerErr:          false,
			bearerExpiryOffset: 0,
			origin:             testOrigin,
			sessionTTL:         0,
			expectedStatus:     http.StatusNoContent,
			expectedCookieName: "__Host-cron_ui_session",
			expectCleared:      true,
			maxCookieAge:       0,
		},
		{
			name:           "bearer without scope denied",
			method:         http.MethodPost,
			path:           "/admin/session",
			sessionSubject: "",
			bearerSubject:  "op-reader",
			bearerScopes: []string{
				"other:read",
			},
			bearerErr:          false,
			bearerExpiryOffset: time.Hour,
			origin:             testOrigin,
			sessionTTL:         0,
			expectedStatus:     http.StatusForbidden,
			expectedCookieName: "",
			expectCleared:      false,
			maxCookieAge:       0,
		},
		{
			name:           "expired bearer denied",
			method:         http.MethodPost,
			path:           "/admin/session",
			sessionSubject: "",
			bearerSubject:  "op-reader",
			bearerScopes: []string{
				"cron:read",
			},
			bearerErr:          false,
			bearerExpiryOffset: -time.Minute,
			origin:             testOrigin,
			sessionTTL:         0,
			expectedStatus:     http.StatusForbidden,
			expectedCookieName: "",
			expectCleared:      false,
			maxCookieAge:       0,
		},
		{
			name:           "missing origin denied",
			method:         http.MethodPost,
			path:           "/admin/session",
			sessionSubject: "",
			bearerSubject:  "op-reader",
			bearerScopes: []string{
				"cron:read",
			},
			bearerErr:          false,
			bearerExpiryOffset: time.Hour,
			origin:             "",
			sessionTTL:         0,
			expectedStatus:     http.StatusForbidden,
			expectedCookieName: "",
			expectCleared:      false,
			maxCookieAge:       0,
		},
		{
			name:           "malformed subject bearer denied on anonymous key",
			method:         http.MethodPost,
			path:           "/admin/session",
			sessionSubject: "",
			bearerSubject:  "bad:subject",
			bearerScopes: []string{
				"cron:read",
			},
			bearerErr:          false,
			bearerExpiryOffset: time.Hour,
			origin:             testOrigin,
			sessionTTL:         0,
			expectedStatus:     http.StatusForbidden,
			expectedCookieName: "",
			expectCleared:      false,
			maxCookieAge:       0,
		},
		{
			name:               "unknown bearer denied",
			method:             http.MethodPost,
			path:               "/admin/session",
			sessionSubject:     "",
			bearerSubject:      "",
			bearerScopes:       nil,
			bearerErr:          true,
			bearerExpiryOffset: 0,
			origin:             testOrigin,
			sessionTTL:         0,
			expectedStatus:     http.StatusUnauthorized,
			expectedCookieName: "",
			expectCleared:      false,
			maxCookieAge:       0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := testConfig()
			if tc.sessionTTL != 0 {
				cfg.SessionMaxTTL = tc.sessionTTL
			}

			fx := newFixture(t, cfg)
			stubAllowAll(fx)

			cookie := ""
			if tc.sessionSubject != "" {
				cookie = sessionCookie(seedSession(t, fx, tc.sessionSubject))
			}

			header := "Bearer case-token"
			if tc.bearerErr {
				fx.verifier.On(
					"VerifyBearer",
					mock.Anything,
					header,
				).Return(admin.VerifiedBearer{}, admin.ErrBadBearer)
			} else {
				fx.verifier.On(
					"VerifyBearer",
					mock.Anything,
					header,
				).Return(admin.VerifiedBearer{
					Subject:   tc.bearerSubject,
					Scopes:    tc.bearerScopes,
					ExpiresAt: time.Now().Add(tc.bearerExpiryOffset),
				}, nil)
			}

			if tc.expectedCookieName != "" && !tc.expectCleared {
				fx.issuer.On(
					"IssueReadSession",
					tc.bearerSubject,
					mock.Anything,
				).Return("tok-issued", time.Now().Add(time.Hour), nil)
			}

			rec := doRequest(fx, tc.method, "https://"+testHost+tc.path, cookie, tc.origin, map[string]string{
				"Authorization": header,
			})
			assert.Equal(t, tc.expectedStatus, rec.Code)

			cookies := rec.Result().Cookies()
			if tc.expectedCookieName == "" {
				assert.Empty(t, cookies)
			} else {
				require.Len(t, cookies, 1)
				assert.Equal(t, tc.expectedCookieName, cookies[0].Name)

				if tc.expectCleared {
					assert.Negative(t, cookies[0].MaxAge)
				} else {
					assert.NotEmpty(t, cookies[0].Value)
					assert.True(t, cookies[0].Secure)
					assert.True(t, cookies[0].HttpOnly)
					assert.Equal(t, http.SameSiteStrictMode, cookies[0].SameSite)
					assert.Equal(t, "/", cookies[0].Path)
					assert.Empty(t, cookies[0].Domain)
					assert.Positive(t, cookies[0].MaxAge)
					assert.LessOrEqual(t, cookies[0].MaxAge, tc.maxCookieAge)
				}
			}

			assert.Empty(t, fx.upstream.Calls)
		})
	}
}

func TestCustomPolicySurface(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name               string
		method             string
		path               string
		cookieToken        string
		bearerSubject      string
		bearerScopes       []string
		expectedStatus     int
		expectedCookieName string
		expectedKey        string
	}

	testCases := []testCase{
		{
			name:          "custom cookie round-trips",
			method:        http.MethodPost,
			path:          "/admin/session",
			cookieToken:   "",
			bearerSubject: "op-reader",
			bearerScopes: []string{
				"cron:read",
			},
			expectedStatus:     http.StatusNoContent,
			expectedCookieName: "__Host-custom",
			expectedKey:        "ratelimit:cron-custom:op-reader",
		},
		{
			name:               "custom namespace keys reads and anonymous bucket",
			method:             http.MethodGet,
			path:               "/api/jobs",
			cookieToken:        "tok-x",
			bearerSubject:      "",
			bearerScopes:       nil,
			expectedStatus:     http.StatusOK,
			expectedCookieName: "",
			expectedKey:        "ratelimit:cron-custom:op-reader",
		},
		{
			name:               "custom anonymous bucket",
			method:             http.MethodGet,
			path:               "/api/jobs",
			cookieToken:        "",
			bearerSubject:      "",
			bearerScopes:       nil,
			expectedStatus:     http.StatusUnauthorized,
			expectedCookieName: "",
			expectedKey:        "ratelimit:cron-custom:nobody",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := testConfig()
			cfg.SessionCookieName = "__Host-custom"
			cfg.LimiterNamespace = "cron-custom"
			cfg.AnonymousSubject = "nobody"
			fx := newFixture(t, cfg)
			stubAllowAll(fx)

			cookie := ""
			if tc.cookieToken != "" {
				cookie = sessionCookieNamed("__Host-custom", tc.cookieToken)

				fx.issuer.On(
					"VerifySessionToken",
					mock.Anything,
					mock.Anything,
				).Return(admin.Session{Subject: "op-reader", ExpiresAt: time.Now().Add(time.Hour)}, nil)
			}

			headers := map[string]string{}
			if tc.bearerSubject != "" {
				headers["Authorization"] = "Bearer custom"

				fx.verifier.On(
					"VerifyBearer",
					mock.Anything,
					"Bearer custom",
				).Return(admin.VerifiedBearer{
					Subject:   tc.bearerSubject,
					Scopes:    tc.bearerScopes,
					ExpiresAt: time.Now().Add(time.Hour),
				}, nil)
				fx.issuer.On(
					"IssueReadSession",
					tc.bearerSubject,
					mock.Anything,
				).Return("tok-custom", time.Now().Add(time.Hour), nil)
			}

			origin := ""
			if tc.method == http.MethodPost {
				origin = testOrigin
			}

			rec := doRequest(fx, tc.method, "https://"+testHost+tc.path, cookie, origin, headers)
			assert.Equal(t, tc.expectedStatus, rec.Code)

			if tc.expectedCookieName != "" {
				cookies := rec.Result().Cookies()
				require.Len(t, cookies, 1)
				assert.Equal(t, tc.expectedCookieName, cookies[0].Name)
			}

			require.Len(t, limiterKeys(fx), 1)
			assert.Equal(t, tc.expectedKey, limiterKeys(fx)[0])
		})
	}
}

func TestRateLimit(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                  string
		limitDecision         appport.RateLimitDecision
		limitStoreError       bool
		method                string
		path                  string
		sessionSubject        string
		extraHeaders          map[string]string
		directWriteUpstream   bool
		expectedStatus        int
		expectRetryAfter      bool
		expectedAuthorizer    int
		expectedUpstream      int
		expectedAudits        int
		expectedAuditActor    string
		expectedKey           string
		expectedBodySubstring string
	}

	testCases := []testCase{
		{
			name: "denial returns retry-after without auth or upstream",
			limitDecision: appport.RateLimitDecision{
				Allowed:    false,
				RetryAfter: 30 * time.Second,
			},
			limitStoreError:       false,
			method:                http.MethodGet,
			path:                  "/api/jobs",
			sessionSubject:        "op-reader",
			extraHeaders:          nil,
			directWriteUpstream:   false,
			expectedStatus:        http.StatusTooManyRequests,
			expectRetryAfter:      true,
			expectedAuthorizer:    0,
			expectedUpstream:      0,
			expectedAudits:        0,
			expectedAuditActor:    "",
			expectedKey:           "",
			expectedBodySubstring: "",
		},
		{
			name:                  "store failure fails closed",
			limitDecision:         appport.RateLimitDecision{},
			limitStoreError:       true,
			method:                http.MethodGet,
			path:                  "/api/jobs",
			sessionSubject:        "op-reader",
			extraHeaders:          nil,
			directWriteUpstream:   false,
			expectedStatus:        http.StatusServiceUnavailable,
			expectRetryAfter:      false,
			expectedAuthorizer:    0,
			expectedUpstream:      0,
			expectedAudits:        0,
			expectedAuditActor:    "",
			expectedKey:           "",
			expectedBodySubstring: "",
		},
		{
			name: "denied mutation is audited even when limited",
			limitDecision: appport.RateLimitDecision{
				Allowed:    false,
				RetryAfter: 30 * time.Second,
			},
			limitStoreError:       false,
			method:                http.MethodPost,
			path:                  "/api/jobs",
			sessionSubject:        "op-reader",
			extraHeaders:          nil,
			directWriteUpstream:   false,
			expectedStatus:        http.StatusTooManyRequests,
			expectRetryAfter:      true,
			expectedAuthorizer:    0,
			expectedUpstream:      0,
			expectedAudits:        1,
			expectedAuditActor:    "op-reader",
			expectedKey:           "",
			expectedBodySubstring: "",
		},
		{
			name: "forwarded headers never form the key",
			limitDecision: appport.RateLimitDecision{
				Allowed:   true,
				Remaining: 99,
			},
			limitStoreError: false,
			method:          http.MethodGet,
			path:            "/api/jobs",
			sessionSubject:  "op-reader",
			extraHeaders: map[string]string{
				"X-Forwarded-For": "spoofed-ip",
				"X-Real-Ip":       "spoofed-ip",
			},
			directWriteUpstream:   false,
			expectedStatus:        http.StatusOK,
			expectRetryAfter:      false,
			expectedAuthorizer:    1,
			expectedUpstream:      1,
			expectedAudits:        0,
			expectedAuditActor:    "",
			expectedKey:           "ratelimit:cron-ui:op-reader",
			expectedBodySubstring: "",
		},
		{
			name: "direct write upstream still strips CORS",
			limitDecision: appport.RateLimitDecision{
				Allowed:   true,
				Remaining: 99,
			},
			limitStoreError:       false,
			method:                http.MethodGet,
			path:                  "/api/jobs",
			sessionSubject:        "op-reader",
			extraHeaders:          nil,
			directWriteUpstream:   true,
			expectedStatus:        http.StatusOK,
			expectRetryAfter:      false,
			expectedAuthorizer:    1,
			expectedUpstream:      1,
			expectedAudits:        0,
			expectedAuditActor:    "",
			expectedKey:           "",
			expectedBodySubstring: `"jobs"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fx := newFixture(t, testConfig())
			if tc.limitStoreError {
				stubLimiter(fx, appport.RateLimitDecision{}, admin.ErrStoreDown)
			} else {
				stubLimiter(fx, tc.limitDecision, nil)
			}

			fx.upstream.directWrite = tc.directWriteUpstream

			cookie := ""
			if tc.sessionSubject != "" {
				cookie = sessionCookie(seedSession(t, fx, tc.sessionSubject))
			}

			rec := doRequest(fx, tc.method, "https://"+testHost+tc.path, cookie, "", tc.extraHeaders)
			assert.Equal(t, tc.expectedStatus, rec.Code)

			if tc.expectRetryAfter {
				assert.NotEmpty(t, rec.Header().Get("Retry-After"))
			}

			assert.Len(t, fx.authorizer.Calls, tc.expectedAuthorizer)
			assert.Len(t, fx.upstream.Calls, tc.expectedUpstream)
			assert.Len(t, auditEvents(fx), tc.expectedAudits)

			if tc.expectedAudits == 1 {
				assert.Equal(t, tc.expectedAuditActor, auditEvents(fx)[0].Actor)
			}

			if tc.expectedKey != "" {
				require.Len(t, limiterKeys(fx), 1)
				assert.Equal(t, tc.expectedKey, limiterKeys(fx)[0])
			}

			if tc.expectedBodySubstring != "" {
				assert.Contains(t, rec.Body.String(), tc.expectedBodySubstring)
			}

			assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
		})
	}
}

func TestRateLimitBootstrap(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		limitDecision    appport.RateLimitDecision
		limitStoreError  bool
		bearerSubject    string
		bearerScopes     []string
		bearerErr        bool
		expectedStatus   int
		expectRetryAfter bool
		expectedKey      string
	}

	testCases := []testCase{
		{
			name: "unauthenticated bootstrap charges the anonymous bucket",
			limitDecision: appport.RateLimitDecision{
				Allowed:   true,
				Remaining: 99,
			},
			limitStoreError:  false,
			bearerSubject:    "",
			bearerScopes:     nil,
			bearerErr:        true,
			expectedStatus:   http.StatusUnauthorized,
			expectRetryAfter: false,
			expectedKey:      "ratelimit:cron-ui:anonymous",
		},
		{
			name:             "bootstrap store error fails closed",
			limitDecision:    appport.RateLimitDecision{},
			limitStoreError:  true,
			bearerSubject:    "",
			bearerScopes:     nil,
			bearerErr:        true,
			expectedStatus:   http.StatusServiceUnavailable,
			expectRetryAfter: false,
			expectedKey:      "ratelimit:cron-ui:anonymous",
		},
		{
			name: "invalid subject bootstrap honors limiter denial",
			limitDecision: appport.RateLimitDecision{
				Allowed:    false,
				RetryAfter: 30 * time.Second,
			},
			limitStoreError:  false,
			bearerSubject:    "bad:subject",
			bearerScopes:     []string{"cron:read"},
			bearerErr:        false,
			expectedStatus:   http.StatusTooManyRequests,
			expectRetryAfter: true,
			expectedKey:      "ratelimit:cron-ui:anonymous",
		},
		{
			name:             "invalid subject bootstrap honors limiter outage",
			limitDecision:    appport.RateLimitDecision{},
			limitStoreError:  true,
			bearerSubject:    "bad:subject",
			bearerScopes:     []string{"cron:read"},
			bearerErr:        false,
			expectedStatus:   http.StatusServiceUnavailable,
			expectRetryAfter: false,
			expectedKey:      "ratelimit:cron-ui:anonymous",
		},
		{
			name: "missing scope bootstrap honors limiter denial",
			limitDecision: appport.RateLimitDecision{
				Allowed:    false,
				RetryAfter: 30 * time.Second,
			},
			limitStoreError:  false,
			bearerSubject:    "op-reader",
			bearerScopes:     []string{"other:read"},
			bearerErr:        false,
			expectedStatus:   http.StatusTooManyRequests,
			expectRetryAfter: true,
			expectedKey:      "ratelimit:cron-ui:op-reader",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fx := newFixture(t, testConfig())
			if tc.limitStoreError {
				stubLimiter(fx, appport.RateLimitDecision{}, admin.ErrStoreDown)
			} else {
				stubLimiter(fx, tc.limitDecision, nil)
			}

			bearer := admin.VerifiedBearer{}

			bearerErr := admin.ErrBadBearer
			if !tc.bearerErr {
				bearer = admin.VerifiedBearer{
					Subject:   tc.bearerSubject,
					Scopes:    tc.bearerScopes,
					ExpiresAt: time.Now().Add(time.Hour),
				}
				bearerErr = error(nil)
			}

			fx.verifier.On(
				"VerifyBearer",
				mock.Anything,
				mock.Anything,
			).Return(bearer, bearerErr)
			rec := doRequest(fx, http.MethodPost, "https://"+testHost+"/admin/session", "", testOrigin, map[string]string{
				"Authorization": "Bearer case-token",
			})
			assert.Equal(t, tc.expectedStatus, rec.Code)
			assert.Empty(t, rec.Result().Cookies())

			if tc.expectRetryAfter {
				assert.NotEmpty(t, rec.Header().Get("Retry-After"))
			}

			require.Len(t, limiterKeys(fx), 1)
			assert.Equal(t, tc.expectedKey, limiterKeys(fx)[0])
			assert.Empty(t, fx.authorizer.Calls)
			assert.Empty(t, fx.upstream.Calls)
		})
	}
}

func TestRateLimitSubjectKey(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		subject        string
		expectedStatus int
		expectedKey    string
		expectUpstream bool
	}

	testCases := []testCase{
		{
			name:           "empty subject uses anonymous",
			subject:        "",
			expectedStatus: http.StatusUnauthorized,
			expectedKey:    "ratelimit:cron-ui:anonymous",
			expectUpstream: false,
		},
		{
			name:           "colon subject uses anonymous",
			subject:        "tenant:user",
			expectedStatus: http.StatusUnauthorized,
			expectedKey:    "ratelimit:cron-ui:anonymous",
			expectUpstream: false,
		},
		{
			name:           "control character uses anonymous",
			subject:        "op\x01reader",
			expectedStatus: http.StatusUnauthorized,
			expectedKey:    "ratelimit:cron-ui:anonymous",
			expectUpstream: false,
		},
		{
			name:           "padded subject uses anonymous",
			subject:        " op-reader ",
			expectedStatus: http.StatusUnauthorized,
			expectedKey:    "ratelimit:cron-ui:anonymous",
			expectUpstream: false,
		},
		{
			name:           "overlong subject uses anonymous",
			subject:        strings.Repeat("x", 129),
			expectedStatus: http.StatusUnauthorized,
			expectedKey:    "ratelimit:cron-ui:anonymous",
			expectUpstream: false,
		},
		{
			name:           "single byte accepted",
			subject:        "x",
			expectedStatus: http.StatusForbidden,
			expectedKey:    "ratelimit:cron-ui:x",
			expectUpstream: false,
		},
		{
			name:           "128 byte boundary accepted",
			subject:        strings.Repeat("y", 128),
			expectedStatus: http.StatusForbidden,
			expectedKey:    "ratelimit:cron-ui:" + strings.Repeat("y", 128),
			expectUpstream: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fx := newFixture(t, testConfig())
			stubAllowAll(fx)
			cookie := sessionCookie(seedSession(t, fx, tc.subject))
			rec := doRequest(fx, http.MethodGet, "https://"+testHost+"/api/jobs", cookie, "", nil)
			assert.Equal(t, tc.expectedStatus, rec.Code)
			require.Len(t, limiterKeys(fx), 1)
			assert.Equal(t, tc.expectedKey, limiterKeys(fx)[0])
			assert.Equal(t, tc.expectUpstream, len(fx.upstream.Calls) == 1)
		})
	}
}
