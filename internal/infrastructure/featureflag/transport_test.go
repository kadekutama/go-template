package featureflag_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/infrastructure/egress"
	"github.com/kadekutama/go-template/internal/infrastructure/featureflag"
)

func TestGatedTransportRoundTrip(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		egressErr         error
		breakerErr        error
		baseErr           error
		baseStatus        int
		cancelBefore      bool
		expectedStatus    int
		expectedErrorCode string
		expectedWaitCalls int
		expectedBaseCalls int
	}

	testCases := []testCase{
		{
			name:              "admitted request reaches base",
			egressErr:         nil,
			breakerErr:        nil,
			baseErr:           nil,
			baseStatus:        http.StatusOK,
			expectedStatus:    http.StatusOK,
			expectedWaitCalls: 1,
			expectedBaseCalls: 1,
		},
		{
			name:              "limiter store failure fails closed without base call",
			egressErr:         entity.NewError(egress.CodeStoreUnavailable, "egress: limiter store unavailable"),
			breakerErr:        nil,
			baseErr:           nil,
			baseStatus:        http.StatusOK,
			expectedStatus:    0,
			expectedErrorCode: egress.CodeStoreUnavailable,
			expectedWaitCalls: 1,
			expectedBaseCalls: 0,
		},
		{
			name:              "breaker open fast-fails without base call",
			egressErr:         nil,
			breakerErr:        breakerOpen(),
			baseErr:           nil,
			baseStatus:        http.StatusOK,
			expectedStatus:    0,
			expectedErrorCode: "BREAKER_OPEN",
			expectedWaitCalls: 1,
			expectedBaseCalls: 0,
		},
		{
			name:              "base transport error surfaces",
			egressErr:         nil,
			breakerErr:        nil,
			baseErr:           errDialFailed,
			baseStatus:        http.StatusOK,
			expectedStatus:    0,
			expectedErrorCode: "featureflag test: dial failed",
			expectedWaitCalls: 1,
			expectedBaseCalls: 1,
		},
		{
			name:              "pre-canceled context performs no wait or base call",
			egressErr:         nil,
			breakerErr:        nil,
			baseErr:           nil,
			baseStatus:        http.StatusOK,
			cancelBefore:      true,
			expectedStatus:    0,
			expectedErrorCode: "CANCELED",
			expectedWaitCalls: 0,
			expectedBaseCalls: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			egressMock := newMockEgress(tc.egressErr)
			breaker := newMockBreaker(tc.breakerErr)
			base := newMockRoundTripper(tc.baseStatus, tc.baseErr)

			transport, err := featureflag.NewGatedTransport(egressMock, breaker, base, nil)
			require.NoError(t, err)

			ctx := context.Background()

			if tc.cancelBefore {
				var cancel context.CancelFunc

				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unleash:4242/api/client/features", nil)
			require.NoError(t, err)

			resp, err := transport.RoundTrip(req)

			actualStatus := 0
			if resp != nil {
				actualStatus = resp.StatusCode
				_ = resp.Body.Close()
			}

			assert.Equal(t, tc.expectedStatus, actualStatus)
			assert.Equal(t, tc.expectedErrorCode, errorCodeOf(err))

			// Side-effect proof under the packet exception: every attempt
			// admits exactly one permit; limiter failures never reach base.
			assert.Equal(t, tc.expectedWaitCalls, callCount(&egressMock.Mock, "Wait"))
			assert.Equal(t, tc.expectedBaseCalls, callCount(&base.Mock, "RoundTrip"))
		})
	}
}

func TestGatedTransportEgressKeysCarryNoPII(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name         string
		expectedKeys []string
	}

	testCases := []testCase{
		{
			name:         "provider operation key is fixed",
			expectedKeys: []string{"unleash/flags"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			egressMock := newMockEgress(nil)
			breaker := newMockBreaker(nil)
			base := newMockRoundTripper(http.StatusOK, nil)

			transport, err := featureflag.NewGatedTransport(egressMock, breaker, base, nil)
			require.NoError(t, err)

			req, err := http.NewRequestWithContext(
				context.Background(),
				http.MethodGet,
				"http://unleash:4242/api/client/features",
				nil,
			)
			require.NoError(t, err)

			resp, err := transport.RoundTrip(req)

			actualStatus := 0
			if resp != nil {
				actualStatus = resp.StatusCode
				_ = resp.Body.Close()
			}

			assert.Equal(t, http.StatusOK, actualStatus)
			assert.Empty(t, errorCodeOf(err))
			assert.Equal(t, tc.expectedKeys, waitKeys(egressMock))
		})
	}
}
