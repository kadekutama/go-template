package kernel

import "time"

// Clock abstracts time for testability (E01-T06, SPEC §7.10).
type Clock interface {
	Now() time.Time
}

// SystemClock is the production Clock.
type SystemClock struct{}

// Now returns the current wall-clock time.
func (SystemClock) Now() time.Time { return time.Now() }

// FixedClock returns scripted times in order, repeating the last one.
// It is not safe for concurrent use; tests needing concurrency wrap it.
type FixedClock struct {
	times []time.Time
	index int
}

// NewFixedClock builds a Clock replaying the given times.
func NewFixedClock(times ...time.Time) *FixedClock {
	return &FixedClock{times: times}
}

// Now returns the next scripted time (last one repeats forever).
func (c *FixedClock) Now() time.Time {
	if len(c.times) == 0 {
		return time.Time{}
	}
	if c.index >= len(c.times) {
		return c.times[len(c.times)-1]
	}
	current := c.times[c.index]
	c.index++
	return current
}
