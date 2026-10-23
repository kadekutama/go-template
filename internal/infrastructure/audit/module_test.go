package audit_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/audit"
	"github.com/kadekutama/go-template/internal/shared/kernel"
)

// TestModule proves the audit fx module resolves its Params bundle into the
// application audit port. fx.ValidateApp never executes constructors, so the
// test stays hermetic: no database or log exporter is contacted.
func TestModule(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		options          []fx.Option
		expectedHasError bool
	}

	testCases := []testCase{
		{
			name: "valid graph exposes audit logger port",
			options: []fx.Option{
				audit.Module(),
				fx.Provide(func() audit.Params {
					return audit.Params{
						SigningKey: make([]byte, 32),
						Clock:      kernel.SystemClock{},
					}
				}),
				fx.Invoke(func(appport.AuditLogger) {}),
			},
			expectedHasError: false,
		},
		{
			name: "missing audit params fails validation",
			options: []fx.Option{
				audit.Module(),
				fx.Invoke(func(appport.AuditLogger) {}),
			},
			expectedHasError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := fx.ValidateApp(tc.options...)
			assert.Equal(t, tc.expectedHasError, err != nil)
		})
	}
}
