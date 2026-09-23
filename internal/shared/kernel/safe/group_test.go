package safe_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/shared/kernel/safe"
)

func TestGroupWaitsForGoroutines(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	runs := 0

	group := safe.NewGroup(nil, nil)
	group.Go(context.Background(), func(context.Context) {
		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		runs++
		mu.Unlock()
	})
	group.Go(context.Background(), func(context.Context) {
		mu.Lock()
		runs++
		mu.Unlock()
	})

	group.Wait()

	mu.Lock()
	defer mu.Unlock()

	assert.Equal(t, 2, runs)
}

func TestGroupContainsPanic(t *testing.T) {
	t.Parallel()

	panics := make(chan safe.Panic, 1)
	group := safe.NewGroup(nil, func(p safe.Panic) { panics <- p })

	group.Go(context.Background(), func(context.Context) {
		panic("renewal exploded")
	})
	group.Wait()

	p := <-panics

	assert.Equal(t, "renewal exploded", p.Value)
	assert.Equal(t, true, len(p.Stack) > 0)
}

func TestGroupZeroValueAndNil(t *testing.T) {
	t.Parallel()

	assert.Equal(t, false, func() (panicked bool) {
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
