package valueobject_test

import (
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
			expectedError: valueobject.ErrPayoutPolicyTenantIsRequired,
		},
		{
			name: "missing asset code",
			policy: func() valueobject.PayoutPolicy {
				p := valid
				p.AssetCode = ""

				return p
			}(),
			expectedError: valueobject.ErrPayoutPolicyAssetCodeIsRequired,
		},
		{
			name: "negative minimum minor",
			policy: func() valueobject.PayoutPolicy {
				p := valid
				p.MinimumMinor = -1

				return p
			}(),
			expectedError: valueobject.ErrPayoutPolicyMinimumMustBeNonNegative,
		},
		{
			name: "negative first payout hold days",
			policy: func() valueobject.PayoutPolicy {
				p := valid
				p.FirstPayoutHoldDays = -1

				return p
			}(),
			expectedError: valueobject.ErrPayoutPolicyFirstPayoutHoldMustBeNonNegative,
		},
		{
			name: "negative reserve bps",
			policy: func() valueobject.PayoutPolicy {
				p := valid
				p.ReserveBPS = -1

				return p
			}(),
			expectedError: valueobject.ErrPayoutPolicyReserveBpsMustBeWithin010000,
		},
		{
			name: "reserve bps above 10000",
			policy: func() valueobject.PayoutPolicy {
				p := valid
				p.ReserveBPS = 10001

				return p
			}(),
			expectedError: valueobject.ErrPayoutPolicyReserveBpsMustBeWithin010000,
		},
		{
			name: "missing version",
			policy: func() valueobject.PayoutPolicy {
				p := valid
				p.Version = ""

				return p
			}(),
			expectedError: valueobject.ErrPayoutPolicyVersionIsRequired,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.policy.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
