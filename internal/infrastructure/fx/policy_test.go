package fx

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/infrastructure/egress"
)

// TestFXPolicyFile proves the committed E10-T02 egress document is a valid
// provider quota: fx/rates, 100 requests per 720h, 5s maximum wait.
func TestFXPolicyFile(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		data           []byte
		expectedResult []egress.Policy
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "committed fx policy file is a free-plan quota",
			data: func() []byte {
				committed, err := os.ReadFile("../../../config/egress/fx.yaml")
				require.NoError(t, err)

				return committed
			}(),
			expectedResult: []egress.Policy{
				{
					Provider:  "fx",
					Operation: "rates",
					Budget:    100,
					Window:    720 * time.Hour,
					MaxWait:   5 * time.Second,
				},
			},
			expectedError: nil,
		},
		{
			name: "zero budget is rejected by the shared parser",
			data: []byte("policies:\n" +
				"  - provider: fx\n" +
				"    operation: rates\n" +
				"    budget: 0\n" +
				"    window: 720h\n" +
				"    max_wait: 5s\n"),
			expectedResult: nil,
			expectedError:  entity.NewError(egress.CodeInvalidPolicy, "egress: budget must be positive"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			policies, err := egress.ParsePolicies(tc.data)
			assert.Equal(t, tc.expectedResult, policies)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
