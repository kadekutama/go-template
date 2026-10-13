package cronui

import (
	"testing"

	gocron "github.com/go-co-op/gocron/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUpstreamHandler(t *testing.T) {
	t.Parallel()

	live, err := gocron.NewScheduler()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, live.Shutdown())
	})

	type testCase struct {
		name               string
		scheduler          gocron.Scheduler
		expectedHandlerNil bool
		expectedError      error
	}

	testCases := []testCase{
		{
			name:               "nil scheduler rejected",
			scheduler:          nil,
			expectedHandlerNil: true,
			expectedError:      ErrCronuiSchedulerIsRequired,
		},
		{
			name:               "live scheduler yields handler without listener",
			scheduler:          live,
			expectedHandlerNil: false,
			expectedError:      nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler, err := NewUpstreamHandler(tc.scheduler)
			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, tc.expectedHandlerNil, handler == nil)
		})
	}
}
