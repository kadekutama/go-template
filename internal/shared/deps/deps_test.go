package deps_test

import (
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	gocron "github.com/go-co-op/gocron/v2"
	otelpyroscope "github.com/grafana/otel-profiling-go"
	"github.com/grafana/pyroscope-go"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
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

// TestLoMapProjectsSlice proves the samber/lo v1.53.0 pin (E09.2-T01-R01): an
// infallible slice projection preserves ordering and passes the element index.
func TestLoMapProjectsSlice(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		collection     []string
		iteratee       func(item string, index int) string
		expectedResult []string
	}

	testCases := []testCase{
		{
			name:           "empty collection projects to empty slice",
			collection:     nil,
			iteratee:       func(item string, index int) string { return item + strconv.Itoa(index) },
			expectedResult: []string{},
		},
		{
			name:           "index aware projection preserves order",
			collection:     []string{"alpha", "beta"},
			iteratee:       func(item string, index int) string { return item + strconv.Itoa(index) },
			expectedResult: []string{"alpha0", "beta1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actualResult := lo.Map(tc.collection, tc.iteratee)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

// TestLoKeyByIndexesCollection proves keyed lookup indexing (E09.2-T01-R01),
// the replacement for manual `byID := make(map[string]X)` loops.
func TestLoKeyByIndexesCollection(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		collection     []string
		iteratee       func(item string) string
		expectedResult map[string]string
	}

	testCases := []testCase{
		{
			name:           "empty collection indexes to empty map",
			collection:     nil,
			iteratee:       func(item string) string { return item },
			expectedResult: map[string]string{},
		},
		{
			name:           "duplicate keys keep the last item",
			collection:     []string{"dup-first", "other", "dup-last"},
			iteratee:       func(item string) string { return item[:3] },
			expectedResult: map[string]string{"dup": "dup-last", "oth": "other"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actualResult := lo.KeyBy(tc.collection, tc.iteratee)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

// TestLoGroupByBucketsCollection proves multi-bucket grouping (E09.2-T01-R01),
// the replacement for manual `map[string][]X` accumulation loops.
func TestLoGroupByBucketsCollection(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		collection     []string
		iteratee       func(item string) string
		expectedResult map[string][]string
	}

	testCases := []testCase{
		{
			name:           "empty collection groups to empty map",
			collection:     nil,
			iteratee:       func(item string) string { return item },
			expectedResult: map[string][]string{},
		},
		{
			name:       "items bucket by projected key",
			collection: []string{"a-1", "b-1", "a-2"},
			iteratee:   func(item string) string { return item[:1] },
			expectedResult: map[string][]string{
				"a": {"a-1", "a-2"},
				"b": {"b-1"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actualResult := lo.GroupBy(tc.collection, tc.iteratee)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

// TestLoUniqMapDeduplicatesProjection proves deduplicated key extraction
// (E09.2-T01-R01), the replacement for manual `seen := map[string]struct{}`
// loops. lo.UniqMap returns lo.Keys of its deduplication map, so it promises
// unique values, not input order; call sites that need ordering sort or query
// ORDER BY explicitly.
func TestLoUniqMapDeduplicatesProjection(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		collection     []string
		iteratee       func(item string, index int) string
		expectedResult []string
	}

	testCases := []testCase{
		{
			name:           "empty collection deduplicates to empty slice",
			collection:     nil,
			iteratee:       func(item string, _ int) string { return item },
			expectedResult: []string{},
		},
		{
			name:           "duplicate projections collapse regardless of iteration order",
			collection:     []string{"acct-a", "acct-b", "acct-a"},
			iteratee:       func(item string, _ int) string { return item },
			expectedResult: []string{"acct-a", "acct-b"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actualResult := lo.UniqMap(tc.collection, tc.iteratee)
			assert.ElementsMatch(t, tc.expectedResult, actualResult)
		})
	}
}

// TestPyroscopeProfileTypePin proves the pyroscope-go v1.4.3 pin (E09.2-T04-
// R01): the profile-type identifiers the adapter passes to the SDK keep their
// wire names.
func TestPyroscopeProfileTypePin(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		profileType    pyroscope.ProfileType
		expectedResult string
	}

	testCases := []testCase{
		{
			name:           "cpu profile type",
			profileType:    pyroscope.ProfileCPU,
			expectedResult: "cpu",
		},
		{
			name:           "mutex count profile type",
			profileType:    pyroscope.ProfileMutexCount,
			expectedResult: "mutex_count",
		},
		{
			name:           "block duration profile type",
			profileType:    pyroscope.ProfileBlockDuration,
			expectedResult: "block_duration",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actualResult := string(tc.profileType)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

// TestOTelProfilingBridgeConstruction proves the otel-profiling-go v0.7.0 pin
// (E09.2-T04-R01): the bridge wraps a provider without starting a profiler.
func TestOTelProfilingBridgeConstruction(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		provider       oteltrace.TracerProvider
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "noop provider wraps successfully",
			provider:       noop.NewTracerProvider(),
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actualResult := otelpyroscope.NewTracerProvider(tc.provider)
			assert.Equal(t, tc.expectedResult, actualResult == nil)
		})
	}
}
