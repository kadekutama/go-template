package hybrid_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/kadekutama/go-template/internal/infrastructure/cache/store"
	mockstore "github.com/kadekutama/go-template/test/mock/store"
)

// newMockCacheStore returns a thread-safe MockExpiringStore backed by sync.Map.
func newMockCacheStore(t *testing.T) *mockstore.MockExpiringStore {
	m := mockstore.NewMockExpiringStore(t)
	var data sync.Map
	var exp sync.Map

	m.EXPECT().Get(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, key string) ([]byte, error) {
		val, ok := data.Load(key)
		if !ok {
			return nil, store.ErrMiss
		}
		b, ok := val.([]byte)
		if !ok || b == nil {
			return nil, nil
		}
		cp := make([]byte, len(b))
		copy(cp, b)
		return cp, nil
	}).Maybe()

	m.EXPECT().Set(mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, key string, val []byte, ttl time.Duration) error {
		if ttl <= 0 {
			return errors.New("ttl must be positive")
		}
		if val == nil {
			data.Store(key, []byte(nil))
			exp.Store(key, ttl)
			return nil
		}
		cp := make([]byte, len(val))
		copy(cp, val)
		data.Store(key, cp)
		exp.Store(key, ttl)
		return nil
	}).Maybe()

	m.EXPECT().Delete(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, key string) error {
		data.Delete(key)
		exp.Delete(key)
		return nil
	}).Maybe()

	m.EXPECT().TTL(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, key string) (time.Duration, error) {
		val, ok := exp.Load(key)
		if !ok {
			return 0, store.ErrMiss
		}
		return val.(time.Duration), nil
	}).Maybe()

	return m
}
