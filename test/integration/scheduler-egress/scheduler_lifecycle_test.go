package scheduleregress_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gocron "github.com/go-co-op/gocron/v2"
)

// TestSchedulerLifecycle proves the v2 scheduler starts and shuts down
// cleanly with an unstarted no-op job (E09.1-T05-S01). The no-v1-import
// guard runs as a static check recorded in evidence.
func TestSchedulerLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("v2 start and shutdown cleanly", func(t *testing.T) {
		scheduler, err := gocron.NewScheduler()
		require.NoError(t, err)

		var runs atomic.Int32

		job, err := scheduler.NewJob(
			gocron.DurationJob(time.Hour),
			gocron.NewTask(func() { runs.Add(1) }),
		)
		require.NoError(t, err)
		require.NotEqual(t, "00000000-0000-0000-0000-000000000000", job.ID().String())

		scheduler.Start()
		require.NoError(t, scheduler.Shutdown())
		assert.Equal(t, int32(0), runs.Load())
	})
}
