package scheduleregress_test

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
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
	matrixOrigin = "https://cron-admin.test:8443"
	matrixHost   = "cron-admin.test:8443"
)

type mockMatrixIssuer struct {
	mock.Mock
}

func (m *mockMatrixIssuer) IssueReadSession(subject string, notAfter time.Time) (string, time.Time, error) {
	args := m.Called(subject, notAfter)
	token, _ := args.Get(0).(string)
	expiresAt, _ := args.Get(1).(time.Time)

	return token, expiresAt, args.Error(2)
}

func (m *mockMatrixIssuer) VerifySessionToken(ctx context.Context, token string) (admin.Session, error) {
	args := m.Called(ctx, token)
	session, _ := args.Get(0).(admin.Session)

	return session, args.Error(1)
}

type mockMatrixVerifier struct {
	mock.Mock
}

func (m *mockMatrixVerifier) VerifyBearer(ctx context.Context, header string) (admin.VerifiedBearer, error) {
	args := m.Called(ctx, header)
	bearer, _ := args.Get(0).(admin.VerifiedBearer)

	return bearer, args.Error(1)
}

type mockMatrixAuthorizer struct {
	mock.Mock
}

func (m *mockMatrixAuthorizer) Authorize(ctx context.Context, subject, action string) (bool, error) {
	args := m.Called(ctx, subject, action)
	allowed, _ := args.Get(0).(bool)

	return allowed, args.Error(1)
}

type mockMatrixAudit struct {
	mock.Mock
}

func (m *mockMatrixAudit) LogDeniedControl(ctx context.Context, event admin.DeniedControlEvent) error {
	args := m.Called(ctx, event)

	return args.Error(0)
}

func matrixAuditEvents(fx *matrixFixture) []admin.DeniedControlEvent {
	events := make([]admin.DeniedControlEvent, 0, len(fx.audit.Calls))

	for _, call := range fx.audit.Calls {
		event, _ := call.Arguments.Get(1).(admin.DeniedControlEvent)
		events = append(events, event)
	}

	return events
}

type mockMatrixUpstream struct {
	mock.Mock
}

func (m *mockMatrixUpstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.Called(w, r)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

type matrixFixture struct {
	handler  http.Handler
	upstream *mockMatrixUpstream
	audit    *mockMatrixAudit
}

func newMatrixFixture(t *testing.T) *matrixFixture {
	t.Helper()

	fx := &matrixFixture{upstream: &mockMatrixUpstream{}, audit: &mockMatrixAudit{}}
	issuer := &mockMatrixIssuer{}
	issuer.On(
		"VerifySessionToken",
		mock.Anything,
		"sess-op-reader",
	).Return(admin.Session{Subject: "op-reader", ExpiresAt: time.Now().Add(time.Hour)}, nil)

	verifier := &mockMatrixVerifier{}
	authorizer := &mockMatrixAuthorizer{}
	authorizer.On(
		"Authorize",
		mock.Anything,
		"op-reader",
		"cron:read",
	).Return(true, nil)
	authorizer.On(
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

	limiter := &mockapplication.MockRateLimiter{}
	limiter.On(
		"Allow",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(appport.RateLimitDecision{Allowed: true, Remaining: 99}, nil)
	handler, err := admin.NewAdminHandler(
		admin.Config{
			Enabled:           true,
			BindAddr:          "127.0.0.1:8443",
			AllowedOrigin:     matrixOrigin,
			TLSConfig:         &tls.Config{},
			RateLimitBudget:   100,
			RateLimitWindow:   time.Minute,
			SessionMaxTTL:     5 * time.Minute,
			SessionScope:      "cron:read",
			SessionCookieName: "__Host-cron_ui_session",
			LimiterNamespace:  "cron-ui",
			AnonymousSubject:  "anonymous",
		},
		func() (http.Handler, error) { return fx.upstream, nil },
		admin.NewCookieSessionAuthenticator(issuer, "__Host-cron_ui_session"),
		verifier,
		issuer,
		authorizer,
		fx.audit,
		limiter,
	)
	require.NoError(t, err)

	fx.handler = handler

	return fx
}

func matrixCookie(token string) string {
	return "__Host-cron_ui_session" + "=" + token
}

func matrixDo(fx *matrixFixture, method, target, token, origin string, headers map[string]string) *httptest.ResponseRecorder {
	cookie := ""
	if token != "" {
		cookie = matrixCookie(token)
	}

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

// TestUIReadOnlySecurity proves the UI matrix end to end (E09.1-T05-S02):
// unauthenticated reads 401, authorized reads forward, cross-origin and
// every mutation route are denied before scheduler access with no wildcard
// CORS response.
func TestUIReadOnlySecurity(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		method         string
		path           string
		cookie         string
		origin         string
		headers        map[string]string
		expectedStatus int
		expectUpstream bool
		expectedAudits int
	}

	reader := "sess-op-reader"

	testCases := []testCase{
		{
			name:           "unauthenticated root",
			method:         http.MethodGet,
			path:           "/",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "unauthenticated api",
			method:         http.MethodGet,
			path:           "/api/jobs",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "unauthenticated websocket",
			method:         http.MethodGet,
			path:           "/ws",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "reader reads jobs",
			method:         http.MethodGet,
			path:           "/api/jobs",
			cookie:         reader,
			expectedStatus: http.StatusOK,
			expectUpstream: true,
		},
		{
			name:           "reader reads config",
			method:         http.MethodGet,
			path:           "/api/config",
			cookie:         reader,
			expectedStatus: http.StatusOK,
			expectUpstream: true,
		},
		{
			name:           "reader websocket forwarded",
			method:         http.MethodGet,
			path:           "/ws",
			cookie:         reader,
			origin:         matrixOrigin,
			headers:        map[string]string{"Upgrade": "websocket", "Connection": "Upgrade"},
			expectedStatus: http.StatusOK,
			expectUpstream: true,
		},
		{
			name:           "cross-origin read denied",
			method:         http.MethodGet,
			path:           "/api/jobs",
			cookie:         reader,
			origin:         "https://evil.test",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "cross-origin websocket denied",
			method:         http.MethodGet,
			path:           "/ws",
			cookie:         reader,
			origin:         "https://evil.test",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "create job denied",
			method:         http.MethodPost,
			path:           "/api/jobs",
			cookie:         reader,
			expectedStatus: http.StatusForbidden,
			expectedAudits: 1,
		},
		{
			name:           "run job denied",
			method:         http.MethodPost,
			path:           "/api/jobs/1/run",
			cookie:         reader,
			expectedStatus: http.StatusForbidden,
			expectedAudits: 1,
		},
		{
			name:           "delete job denied",
			method:         http.MethodDelete,
			path:           "/api/jobs/1",
			cookie:         reader,
			expectedStatus: http.StatusForbidden,
			expectedAudits: 1,
		},
		{
			name:           "scheduler start denied",
			method:         http.MethodPost,
			path:           "/api/scheduler/start",
			cookie:         reader,
			expectedStatus: http.StatusForbidden,
			expectedAudits: 1,
		},
		{
			name:           "scheduler stop denied",
			method:         http.MethodPost,
			path:           "/api/scheduler/stop",
			cookie:         reader,
			expectedStatus: http.StatusForbidden,
			expectedAudits: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newMatrixFixture(t)
			rec := matrixDo(fx, tc.method, "https://"+matrixHost+tc.path, tc.cookie, tc.origin, tc.headers)
			assert.Equal(t, tc.expectedStatus, rec.Code)
			assert.Equal(t, tc.expectUpstream, len(fx.upstream.Calls) == 1)
			assert.Len(t, matrixAuditEvents(fx), tc.expectedAudits)
			assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
		})
	}
}
