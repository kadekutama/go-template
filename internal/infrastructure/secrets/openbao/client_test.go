package openbao_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/httpclient"
	"github.com/kadekutama/go-template/internal/infrastructure/secrets/openbao"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
)

// mockOpenBaoBase is the fake OpenBao origin. No traffic leaves the process:
// httpmock stubs the transport, so tests never bind a port.
const mockOpenBaoBase = "https://openbao.test"

// mockHTTPClient returns a client whose transport serves the mock OpenBao
// endpoints. Each caller gets an isolated transport, so parallel subtests
// never share responder state.
func mockHTTPClient() *http.Client {
	transport := httpmock.NewMockTransport()
	transport.RegisterResponder("GET", "=~^https://openbao\\.test/v1/kv/data/.+",
		func(req *http.Request) (*http.Response, error) {
			if strings.HasSuffix(req.URL.Path, "/missing") {
				return httpmock.NewStringResponse(http.StatusNotFound, ""), nil
			}

			return httpmock.NewStringResponse(http.StatusOK,
				`{"data":{"data":{"value":"s3cr3t-`+req.URL.Path+`"}}}`), nil
		})
	transport.RegisterResponder("GET", mockOpenBaoBase+"/v1/database/creds/app-user-role",
		httpmock.NewStringResponder(http.StatusOK,
			`{"lease_id":"db/creds/abc","renewable":true,"lease_duration":3600,`+
				`"data":{"username":"app-user-x","password":"pw-x"}}`))
	transport.RegisterResponder("POST", mockOpenBaoBase+"/v1/sys/leases/renew",
		httpmock.NewStringResponder(http.StatusOK, `{"lease_duration":3600}`))

	return &http.Client{Transport: transport}
}

func testParams(clk appport.Clock, doer httpclient.Doer) openbao.OpenBaoParams {
	return openbao.OpenBaoParams{
		Address:    mockOpenBaoBase,
		Token:      "test-token-12345678",
		KVMount:    "kv",
		DBMount:    "database",
		DefaultTTL: 5 * time.Minute,
		HTTP:       doer,
		Clock:      clk,
	}
}

func TestOpenBaoValidation(t *testing.T) {
	t.Parallel()

	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).Maybe()

	type testCase struct {
		name          string
		params        openbao.OpenBaoParams
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid params",
			params:        testParams(clk, mockHTTPClient()),
			expectedError: nil,
		},
		{
			name: "missing address",
			params: func() openbao.OpenBaoParams {
				p := testParams(clk, mockHTTPClient())
				p.Address = ""
				return p
			}(),
			expectedError: openbao.ErrConfigRequired,
		},
		{
			name: "missing http",
			params: func() openbao.OpenBaoParams {
				p := testParams(clk, mockHTTPClient())
				p.HTTP = nil
				return p
			}(),
			expectedError: openbao.ErrConfigRequired,
		},
		{
			name: "missing clock",
			params: func() openbao.OpenBaoParams {
				p := testParams(clk, mockHTTPClient())
				p.Clock = nil
				return p
			}(),
			expectedError: openbao.ErrClockRequired,
		},
		{
			name: "zero ttl",
			params: func() openbao.OpenBaoParams {
				p := testParams(clk, mockHTTPClient())
				p.DefaultTTL = 0
				return p
			}(),
			expectedError: openbao.ErrConfigRequired,
		},
		{
			name: "missing db mount rejected",
			params: func() openbao.OpenBaoParams {
				p := testParams(clk, mockHTTPClient())
				p.DBMount = ""
				return p
			}(),
			expectedError: openbao.ErrConfigRequired,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := openbao.NewStore(tc.params)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestOpenBaoGet(t *testing.T) {
	t.Parallel()

	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).Maybe()

	type testCase struct {
		name          string
		ctx           context.Context
		tenant        valueobject.TenantID
		secretName    string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "kv hit",
			ctx:           context.Background(),
			tenant:        valueobject.TenantID("t1"),
			secretName:    "db-password",
			expectedError: nil,
		},
		{
			name:          "missing secret names path",
			ctx:           context.Background(),
			tenant:        valueobject.TenantID("t1"),
			secretName:    "missing",
			expectedError: openbao.ErrSecretNotFound,
		},
		{
			name:          "empty tenant",
			ctx:           context.Background(),
			tenant:        valueobject.TenantID(""),
			secretName:    "db-password",
			expectedError: openbao.ErrSecretNotFound,
		},
		{
			name: "canceled context",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			tenant:        valueobject.TenantID("t1"),
			secretName:    "db-password",
			expectedError: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			store, err := openbao.NewStore(testParams(clk, mockHTTPClient()))
			require.NoError(t, err)

			secret, err := store.Get(tc.ctx, tc.tenant, tc.secretName)
			if tc.expectedError != nil {
				assert.Error(t, err)
				if tc.secretName == "missing" {
					assert.ErrorIs(t, err, openbao.ErrSecretNotFound)
					assert.Contains(t, err.Error(), "missing")
				}
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, secret.Value)

			// Every Get is a strong read (ADR-020: no adapter cache), so a
			// second read re-fetches identical bytes from OpenBao.
			cached, err := store.Get(context.Background(), tc.tenant, tc.secretName)
			require.NoError(t, err)
			assert.Equal(t, secret.Value, cached.Value)
		})
	}
}

func TestOpenBaoDynamicDB(t *testing.T) {
	t.Parallel()

	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).Maybe()

	type testCase struct {
		name          string
		role          string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "lease and renew",
			role:          "app-user-role",
			expectedError: nil,
		},
		{
			name:          "empty role rejected",
			role:          "",
			expectedError: openbao.ErrLeaseFailed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			store, err := openbao.NewStore(testParams(clk, mockHTTPClient()))
			require.NoError(t, err)

			creds, err := store.LeaseDB(context.Background(), tc.role)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, "app-user-x", creds.Username)
			assert.True(t, creds.Renewable)

			renewed, err := store.RenewLease(context.Background(), creds.LeaseID)
			require.NoError(t, err)
			assert.Equal(t, time.Hour, renewed)
		})
	}
}

func TestResolveReference(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name         string
		ref          string
		expectedPath string
		expectedOK   bool
	}

	testCases := []testCase{
		{
			name:         "secret ref",
			ref:          "{{ secret:database/creds/app-user-role }}",
			expectedPath: "database/creds/app-user-role",
			expectedOK:   true,
		},
		{
			name:         "plain value",
			ref:          "postgres://localhost/app",
			expectedPath: "",
			expectedOK:   false,
		},
		{
			name:         "empty ref",
			ref:          "{{ secret: }}",
			expectedPath: "",
			expectedOK:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path, ok := openbao.ResolveReference(tc.ref)
			assert.Equal(t, tc.expectedPath, path)
			assert.Equal(t, tc.expectedOK, ok)
		})
	}
}

func TestOpenBaoGetServerOutage(t *testing.T) {
	t.Parallel()

	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).Maybe()

	transport := httpmock.NewMockTransport()
	transport.RegisterResponder("GET", "=~^https://openbao\\.test/v1/kv/data/.+",
		httpmock.NewStringResponder(http.StatusServiceUnavailable, `{"errors":["sealed"]}`))
	params := testParams(clk, &http.Client{Transport: transport})

	store, err := openbao.NewStore(params)
	require.NoError(t, err)

	_, err = store.Get(context.Background(), valueobject.TenantID("t1"), "db-password")
	require.Error(t, err)
	assert.ErrorIs(t, err, openbao.ErrSecretUnavailable)
	assert.NotErrorIs(t, err, openbao.ErrSecretNotFound)
}

func TestOpenBaoRenewLeaseFailure(t *testing.T) {
	t.Parallel()

	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).Maybe()

	transport := httpmock.NewMockTransport()
	transport.RegisterResponder("POST", mockOpenBaoBase+"/v1/sys/leases/renew",
		httpmock.NewStringResponder(http.StatusInternalServerError, `{"errors":["boom"]}`))
	params := testParams(clk, &http.Client{Transport: transport})

	store, err := openbao.NewStore(params)
	require.NoError(t, err)

	_, err = store.RenewLease(context.Background(), "db/creds/abc")
	require.Error(t, err)
	assert.ErrorIs(t, err, openbao.ErrLeaseFailed)
}
