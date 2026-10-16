package featureflag

import (
	"testing"

	unleashContext "github.com/Unleash/unleash-go-sdk/v6/context"
	"github.com/open-feature/go-sdk/openfeature"
	"github.com/stretchr/testify/assert"
)

// TestToUnleashEvalContext verifies the flattened-attribute mapping,
// including the targetingKey that carries the user identity for Unleash
// user/tenant targeting (CR-001). It needs no doubles: the mapping is pure.
func TestToUnleashEvalContext(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		evalCtx        openfeature.FlattenedContext
		expectedResult unleashContext.Context
		expectError    bool
	}

	testCases := []testCase{
		{
			name: "targeting key maps to user id",
			evalCtx: openfeature.FlattenedContext{
				openfeature.TargetingKey: "usr_123",
			},
			expectedResult: unleashContext.Context{
				UserId:     "usr_123",
				Properties: map[string]string{},
			},
			expectError: false,
		},
		{
			name: "typed fields and custom properties map",
			evalCtx: openfeature.FlattenedContext{
				"UserId":        "u1",
				"SessionId":     "s1",
				"RemoteAddress": "1.2.3.4",
				"AppName":       "shop",
				"Environment":   "prod",
				"CurrentTime":   "now",
				"user_tier":     "gold",
			},
			expectedResult: unleashContext.Context{
				UserId:        "u1",
				SessionId:     "s1",
				RemoteAddress: "1.2.3.4",
				AppName:       "shop",
				Environment:   "prod",
				CurrentTime:   "now",
				Properties:    map[string]string{"user_tier": "gold"},
			},
			expectError: false,
		},
		{
			name:           "empty context maps empty",
			evalCtx:        openfeature.FlattenedContext{},
			expectedResult: unleashContext.Context{},
			expectError:    false,
		},
		{
			name: "non scalar attribute rejected",
			evalCtx: openfeature.FlattenedContext{
				"custom": []string{"x"},
			},
			expectedResult: unleashContext.Context{},
			expectError:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actualResult, err := toUnleashEvalContext(tc.evalCtx)
			assert.Equal(t, tc.expectedResult, actualResult)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
