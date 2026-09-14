package valueobject_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestPayoutPolicyValidate(t *testing.T) {
	t.Parallel()

	valid := valueobject.PayoutPolicy{
		TenantID:            "t-1",
		AssetCode:           "USD",
		MinimumMinor:        1000,
		FirstPayoutHoldDays: 7,
		ReserveBPS:          1000,
		InstantEligible:     true,
		Version:             "v1",
	}

	type testCase struct {
		name          string
		policy        valueobject.PayoutPolicy
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid policy",
			policy:        valid,
			expectedError: nil,
		},
		{
			name: "missing tenant id",
			policy: func() valueobject.PayoutPolicy {
				p := valid
				p.TenantID = ""
				return p
			}(),
			expectedError: errors.New("payout policy: tenant is required"),
		},
		{
			name: "missing asset code",
			policy: func() valueobject.PayoutPolicy {
				p := valid
				p.AssetCode = ""
				return p
			}(),
			expectedError: errors.New("payout policy: asset code is required"),
		},
		{
			name: "negative minimum minor",
			policy: func() valueobject.PayoutPolicy {
				p := valid
				p.MinimumMinor = -1
				return p
			}(),
			expectedError: errors.New("payout policy: minimum must be non-negative"),
		},
		{
			name: "negative first payout hold days",
			policy: func() valueobject.PayoutPolicy {
				p := valid
				p.FirstPayoutHoldDays = -1
				return p
			}(),
			expectedError: errors.New("payout policy: first-payout hold must be non-negative"),
		},
		{
			name: "negative reserve bps",
			policy: func() valueobject.PayoutPolicy {
				p := valid
				p.ReserveBPS = -1
				return p
			}(),
			expectedError: errors.New("payout policy: reserve bps must be within [0,10000]"),
		},
		{
			name: "reserve bps above 10000",
			policy: func() valueobject.PayoutPolicy {
				p := valid
				p.ReserveBPS = 10001
				return p
			}(),
			expectedError: errors.New("payout policy: reserve bps must be within [0,10000]"),
		},
		{
			name: "missing version",
			policy: func() valueobject.PayoutPolicy {
				p := valid
				p.Version = ""
				return p
			}(),
			expectedError: errors.New("payout policy: version is required"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.policy.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
