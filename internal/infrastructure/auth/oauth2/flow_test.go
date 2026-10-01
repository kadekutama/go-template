package oauth2_test

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	oauth2 "github.com/kadekutama/go-template/internal/infrastructure/auth/oauth2"
	"github.com/kadekutama/go-template/internal/infrastructure/httpclient"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
	mockoauth2 "github.com/kadekutama/go-template/test/mock/oauth2"
)

// mockOIDCBase is the fake provider origin. No traffic leaves the process:
// httpmock stubs the transport, so tests never bind a port.
const mockOIDCBase = "https://oidc.test"

// mockHTTPClient returns a client whose transport serves the mock OIDC
// endpoints. Each caller gets an isolated transport, so parallel subtests
// never share responder state.
func mockHTTPClient() *http.Client {
	transport := httpmock.NewMockTransport()
	transport.RegisterResponder("POST", mockOIDCBase+"/token",
		func(req *http.Request) (*http.Response, error) {
			if err := req.ParseForm(); err != nil {
				return httpmock.NewStringResponse(http.StatusBadRequest, ""), nil
			}

			if req.Form.Get("code") == "bad-code" {
				return httpmock.NewStringResponse(http.StatusBadRequest, ""), nil
			}

			return httpmock.NewStringResponse(http.StatusOK,
				`{"access_token":"tok-`+req.Form.Get("code")+`","token_type":"Bearer"}`), nil
		})
	transport.RegisterResponder("GET", mockOIDCBase+"/userinfo",
		httpmock.NewStringResponder(http.StatusOK, `{"id":"ext-123","email":"user@example.com"}`))

	return &http.Client{Transport: transport}
}

// errFakeMiss backs the test-only KV double.
var errFakeMiss = errors.New("fake kv: miss")

func newMockKV(t *testing.T) *mockoauth2.MockKVStore {
	t.Helper()

	m := mockoauth2.NewMockKVStore(t)
	var store sync.Map

	m.EXPECT().Get(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, key string) ([]byte, error) {
		val, ok := store.Load(key)
		if !ok {
			return nil, errFakeMiss
		}
		return append([]byte(nil), val.([]byte)...), nil
	}).Maybe()

	m.EXPECT().Set(mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, key string, value []byte, _ time.Duration) error {
		store.Store(key, append([]byte(nil), value...))
		return nil
	}).Maybe()

	m.EXPECT().Delete(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, key string) error {
		store.Delete(key)
		return nil
	}).Maybe()

	m.EXPECT().SetNX(mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, key string, value []byte, _ time.Duration) (bool, error) {
		_, loaded := store.LoadOrStore(key, append([]byte(nil), value...))
		return !loaded, nil
	}).Maybe()

	return m
}

func testParams(t *testing.T, clk appport.Clock, doer httpclient.Doer) oauth2.OAuthParams {
	t.Helper()

	return oauth2.OAuthParams{
		Providers: map[string]oauth2.ProviderConfig{
			"google": {
				ClientID:     "cid",
				ClientSecret: "csecret-that-is-long-enough-for-tests",
				AuthURL:      mockOIDCBase + "/auth",
				TokenURL:     mockOIDCBase + "/token",
				UserInfoURL:  mockOIDCBase + "/userinfo",
				Scopes:       []string{"openid", "email"},
			},
		},
		StateSecret: []byte("0123456789abcdef0123456789abcdef"),
		StateTTL:    10 * time.Minute,
		HTTP:        doer,
		KV:          newMockKV(t),
		Clock:       clk,
	}
}

func TestOAuthValidation(t *testing.T) {
	t.Parallel()

	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).Maybe()

	type testCase struct {
		name          string
		params        oauth2.OAuthParams
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid params",
			params:        testParams(t, clk, mockHTTPClient()),
			expectedError: nil,
		},
		{
			name: "no providers",
			params: func() oauth2.OAuthParams {
				p := testParams(t, clk, mockHTTPClient())
				p.Providers = map[string]oauth2.ProviderConfig{}
				return p
			}(),
			expectedError: oauth2.ErrConfigRequired,
		},
		{
			name: "short state secret",
			params: func() oauth2.OAuthParams {
				p := testParams(t, clk, mockHTTPClient())
				p.StateSecret = []byte("short")
				return p
			}(),
			expectedError: oauth2.ErrConfigRequired,
		},
		{
			name: "missing http",
			params: func() oauth2.OAuthParams {
				p := testParams(t, clk, mockHTTPClient())
				p.HTTP = nil
				return p
			}(),
			expectedError: oauth2.ErrConfigRequired,
		},
		{
			name: "missing kv store",
			params: func() oauth2.OAuthParams {
				p := testParams(t, clk, mockHTTPClient())
				p.KV = nil
				return p
			}(),
			expectedError: oauth2.ErrConfigRequired,
		},
		{
			name: "missing clock",
			params: func() oauth2.OAuthParams {
				p := testParams(t, clk, mockHTTPClient())
				p.Clock = nil
				return p
			}(),
			expectedError: oauth2.ErrClockRequired,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := oauth2.NewProvider(tc.params)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestOAuthFlow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(now).Maybe()
	tenant := valueobject.TenantID("tenant-oauth")

	type testCase struct {
		name          string
		ctx           context.Context
		provider      string
		code          string
		tamperState   bool
		replay        bool
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "full code flow",
			ctx:           context.Background(),
			provider:      "google",
			code:          "code-1",
			tamperState:   false,
			replay:        false,
			expectedError: nil,
		},
		{
			name:          "unknown provider",
			ctx:           context.Background(),
			provider:      "nope",
			code:          "code-2",
			tamperState:   false,
			replay:        false,
			expectedError: oauth2.ErrUnknownProvider,
		},
		{
			name:          "state tamper rejected",
			ctx:           context.Background(),
			provider:      "google",
			code:          "code-3",
			tamperState:   true,
			replay:        false,
			expectedError: oauth2.ErrStateInvalid,
		},
		{
			name:          "replayed code rejected",
			ctx:           context.Background(),
			provider:      "google",
			code:          "code-4",
			tamperState:   false,
			replay:        true,
			expectedError: oauth2.ErrCodeReuse,
		},
		{
			name: "canceled context",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			provider:      "google",
			code:          "code-5",
			tamperState:   false,
			replay:        false,
			expectedError: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider, err := oauth2.NewProvider(testParams(t, clk, mockHTTPClient()))
			require.NoError(t, err)

			sess, err := provider.AuthURL(tc.ctx, tenant, "google", "https://app.example/cb")
			if tc.expectedError == context.Canceled {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, sess.RedirectURL, "code_challenge=")
			assert.Contains(t, sess.RedirectURL, "state=")

			if tc.provider == "nope" {
				_, err = provider.Exchange(context.Background(), tenant, tc.provider, tc.code, sess.State)
				assert.ErrorIs(t, err, oauth2.ErrUnknownProvider)
				return
			}

			state := sess.State
			if tc.tamperState {
				state += "x"
			}

			identity, err := provider.Exchange(context.Background(), tenant, tc.provider, tc.code, state)
			if tc.tamperState {
				assert.ErrorIs(t, err, oauth2.ErrStateInvalid)
				return
			}
			if tc.replay {
				require.NoError(t, err)
				sess2, err := provider.AuthURL(context.Background(), tenant, "google", "https://app.example/cb")
				require.NoError(t, err)
				_, err = provider.Exchange(context.Background(), tenant, tc.provider, tc.code, sess2.State)
				assert.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, "ext-123", identity.ProviderUser)
			assert.Equal(t, "user@example.com", identity.Email)
			assert.Equal(t, tenant, identity.TenantID)
		})
	}
}

func testParamsWithKV(t *testing.T, clk appport.Clock, doer httpclient.Doer, kv oauth2.KVStore) oauth2.OAuthParams {
	t.Helper()

	params := testParams(t, clk, doer)
	params.KV = kv
	return params
}

func TestOAuthStateExpiry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	tenant := valueobject.TenantID("tenant-oauth")

	type testCase struct {
		name          string
		advance       time.Duration
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "fresh state exchanges",
			advance:       0,
			expectedError: nil,
		},
		{
			name:          "expired state rejected",
			advance:       time.Hour,
			expectedError: oauth2.ErrStateExpired,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clk := mockapplication.NewMockClock(t)
			clk.EXPECT().Now().Return(now).Maybe()
			kv := newMockKV(t)

			provider, err := oauth2.NewProvider(testParamsWithKV(t, clk, mockHTTPClient(), kv))
			require.NoError(t, err)

			sess, err := provider.AuthURL(context.Background(), tenant, "google", "https://app.example/cb")
			require.NoError(t, err)

			laterClk := mockapplication.NewMockClock(t)
			laterClk.EXPECT().Now().Return(now.Add(tc.advance)).Maybe()
			later, err := oauth2.NewProvider(testParamsWithKV(t, laterClk, mockHTTPClient(), kv))
			require.NoError(t, err)

			identity, err := later.Exchange(context.Background(), tenant, "google", "code-exp", sess.State)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, "ext-123", identity.ProviderUser)
		})
	}
}

func TestOAuthExchangeCanceledContext(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(now).Maybe()
	tenant := valueobject.TenantID("tenant-oauth")

	provider, err := oauth2.NewProvider(testParams(t, clk, mockHTTPClient()))
	require.NoError(t, err)

	sess, err := provider.AuthURL(context.Background(), tenant, "google", "https://app.example/cb")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = provider.Exchange(ctx, tenant, "google", "code-cancel", sess.State)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}
