package kernel

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestFixedClockReplaysTimes(t *testing.T) {
	t.Parallel()

	first := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	second := first.Add(time.Hour)
	clock := NewFixedClock(first, second)

	if got := clock.Now(); !got.Equal(first) {
		t.Errorf("got %v, want %v", got, first)
	}
	if got := clock.Now(); !got.Equal(second) {
		t.Errorf("got %v, want %v", got, second)
	}
	if got := clock.Now(); !got.Equal(second) {
		t.Errorf("last time should repeat, got %v", got)
	}
}

func TestFixedClockEmpty(t *testing.T) {
	t.Parallel()

	clock := NewFixedClock()
	if got := clock.Now(); !got.IsZero() {
		t.Errorf("expected zero time for empty fixed clock, got %v", got)
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
