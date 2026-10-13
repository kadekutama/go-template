package deps_test

import (
	"sync/atomic"
	"testing"
	"time"

	gocron "github.com/go-co-op/gocron/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGocronV2SchedulerConstruction proves the v2 pin (E09.1-T01-S03): a
// scheduler constructs, accepts an unstarted no-op job, never starts it, and
// shuts down cleanly.
func TestGocronV2SchedulerConstruction(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		jobDefinition gocron.JobDefinition
	}

	testCases := []testCase{
		{
			name:          "duration job stays unstarted",
			jobDefinition: gocron.DurationJob(time.Hour),
		},
		{
			name:          "cron job stays unstarted",
			jobDefinition: gocron.CronJob("0 * * * * *", true),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			scheduler, err := gocron.NewScheduler()
			require.NoError(t, err)
			require.NotNil(t, scheduler)

			var calls atomic.Int32

			job, err := scheduler.NewJob(
				tc.jobDefinition,
				gocron.NewTask(func() {
					calls.Add(1)
				}),
			)
			require.NoError(t, err)
			require.NotNil(t, job)
			assert.NotEqual(t, "00000000-0000-0000-0000-000000000000", job.ID().String())
			assert.Equal(t, int32(0), calls.Load())

			assert.NoError(t, scheduler.Shutdown())
			assert.Equal(t, int32(0), calls.Load())
		})
	}
}
