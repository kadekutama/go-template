package kernel

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestFixedClock(t *testing.T) {
	t.Parallel()

	t1 := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)

	type testCase struct {
		name          string
		times         []time.Time
		calls         int
		expectedTimes []time.Time
	}

	testCases := []testCase{
		{
			name:  "replays specified sequence and repeats last",
			times: []time.Time{t1, t2},
			calls: 3,
			expectedTimes: []time.Time{
				t1,
				t2,
				t2,
			},
		},
		{
			name:  "empty fixed clock returns zero time",
			times: nil,
			calls: 1,
			expectedTimes: []time.Time{
				{},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clock := NewFixedClock(tc.times...)
			for i := 0; i < tc.calls; i++ {
				got := clock.Now()
				assert.Equal(t, tc.expectedTimes[i], got)
			}
		})
	}
}

func TestSystemClockNow(t *testing.T) {
	t.Parallel()

	clock := SystemClock{}
	before := time.Now()
	now := clock.Now()
	after := time.Now()

	if now.Before(before) || now.After(after) {
		t.Errorf("SystemClock.Now() = %v, should be between %v and %v", now, before, after)
	}
}

func TestUUIDv7UniqueAndVersioned(t *testing.T) {
	t.Parallel()

	gen := NewUUIDGenerator()

	const count = 100
	seen := map[string]bool{}
	for i := 0; i < count; i++ {
		id := gen.NewID()
		if len(id) != 36 {
			t.Fatalf("unexpected UUID length %d: %q", len(id), id)
		}
		parsed, err := uuid.Parse(id)
		if err != nil {
			t.Fatalf("unparseable UUID %q: %v", id, err)
		}
		if parsed.Version() != 7 {
			t.Errorf("want UUIDv7, got version %d: %q", parsed.Version(), id)
		}
		if seen[id] {
			t.Fatalf("duplicate UUID: %q", id)
		}
		seen[id] = true
	}
}
