package safe_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/shared/kernel/safe"
)

func TestGroupWaitsForGoroutines(t *testing.T) {
	t.Parallel()

	var runs atomic.Int64

	group := safe.NewGroup(nil, nil)
	group.Go(context.Background(), func(context.Context) {
		time.Sleep(10 * time.Millisecond)

		runs.Add(1)
	})
	group.Go(context.Background(), func(context.Context) {
		runs.Add(1)
	})

	group.Wait()

	assert.Equal(t, int64(2), runs.Load())
}

func TestGroupContainsPanic(t *testing.T) {
	t.Parallel()

	panics := make(chan safe.PanicError, 1)
	group := safe.NewGroup(nil, func(p safe.PanicError) { panics <- p })

	group.Go(context.Background(), func(context.Context) {
		panic("renewal exploded")
	})
	group.Wait()

	p := <-panics

	assert.Equal(t, "renewal exploded", p.Value)
	assert.NotEmpty(t, p.Stack)
}

func TestGroupZeroValueAndNil(t *testing.T) {
	t.Parallel()

	assert.False(t, func() bool {
		var panicked bool
		defer func() { panicked = recover() != nil }()

		var zeroGroup safe.Group
		zeroGroup.Go(context.Background(), func(context.Context) {})
		zeroGroup.Wait()

		var nilGroup *safe.Group
		nilGroup.Go(context.Background(), func(context.Context) {})
		nilGroup.Wait()

		group := safe.NewGroup(nil, nil)
		group.Go(context.Background(), nil)
		group.Wait()

		return panicked
	}())
}
