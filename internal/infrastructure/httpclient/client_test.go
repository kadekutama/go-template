package httpclient_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/kadekutama/go-template/internal/infrastructure/httpclient"
)

func TestNewShared(t *testing.T) {
	t.Parallel()

	validParams := httpclient.Params{
		Timeout:             10 * time.Second,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	type testCase struct {
		name          string
		params        httpclient.Params
		expectedError bool
	}

	testCases := []testCase{
		{
			name:          "valid params build tuned client",
			params:        validParams,
			expectedError: false,
		},
		{
			name: "custom timeouts honored",
			params: func() httpclient.Params {
				p := validParams
				p.Timeout = 3 * time.Second
				p.MaxIdleConnsPerHost = 4
				return p
			}(),
			expectedError: false,
		},
		{
			name: "zero timeout rejected",
			params: func() httpclient.Params {
				p := validParams
				p.Timeout = 0
				return p
			}(),
			expectedError: true,
		},
		{
			name: "negative idle rejected",
			params: func() httpclient.Params {
				p := validParams
				p.MaxIdleConns = -1
				return p
			}(),
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := httpclient.NewShared(tc.params)
			if tc.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.params.Timeout, client.Timeout)

			transport, ok := client.Transport.(*http.Transport)
			require.True(t, ok)
			assert.Equal(t, tc.params.MaxIdleConns, transport.MaxIdleConns)
			assert.Equal(t, tc.params.MaxIdleConnsPerHost, transport.MaxIdleConnsPerHost)
		})
	}
}

func TestModule(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		params          httpclient.Params
		expectedTimeout time.Duration
	}

	testCases := []testCase{
		{
			name: "standard parameters provided",
			params: httpclient.Params{
				Timeout:             10 * time.Second,
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
			expectedTimeout: 10 * time.Second,
		},
		{
			name: "custom parameters honored",
			params: httpclient.Params{
				Timeout:             5 * time.Second,
				MaxIdleConns:        50,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     30 * time.Second,
			},
			expectedTimeout: 5 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var client *http.Client
			var doer httpclient.Doer

			app := fxtest.New(t,
				httpclient.Module(),
				fx.Provide(func() httpclient.Params {
					return tc.params
				}),
				fx.Populate(&client, &doer),
			)
			app.RequireStart()
			defer app.RequireStop()

			require.NotNil(t, client)
			require.NotNil(t, doer)
			assert.Equal(t, tc.expectedTimeout, client.Timeout)
		})
	}
}
