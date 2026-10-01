package lock_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/valkey"
	"github.com/kadekutama/go-template/internal/infrastructure/lock"
	"github.com/kadekutama/go-template/internal/shared/kernel/safe"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
	mocklock "github.com/kadekutama/go-template/test/mock/lock"
)

func newMockValkeyStore(t *testing.T) *mocklock.MockValkeyStore {
	t.Helper()
	store := mocklock.NewMockValkeyStore(t)
	var data sync.Map

	store.EXPECT().Get(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, key string) ([]byte, error) {
		val, ok := data.Load(key)
		if !ok {
			return nil, valkey.ErrMiss
		}
		value := val.([]byte)
		out := make([]byte, len(value))
		copy(out, value)
		return out, nil
	}).Maybe()

	store.EXPECT().Set(mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, key string, value []byte, _ time.Duration) error {
		out := make([]byte, len(value))
		copy(out, value)
		data.Store(key, out)
		return nil
	}).Maybe()

	store.EXPECT().Delete(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, key string) error {
		data.Delete(key)
		return nil
	}).Maybe()

	store.EXPECT().SetNX(mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, key string, value []byte, _ time.Duration) (bool, error) {
		out := make([]byte, len(value))
		copy(out, value)
		_, loaded := data.LoadOrStore(key, out)
		return !loaded, nil
	}).Maybe()

	return store
}

func TestNewRedlock(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		params         func(t *testing.T) lock.RedlockParams
		expectedResult bool
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "client provided",
			params: func(t *testing.T) lock.RedlockParams {
				return lock.RedlockParams{
					Client: newMockValkeyStore(t),
				}
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "nil client rejected",
			params: func(_ *testing.T) lock.RedlockParams {
				return lock.RedlockParams{
					Client: nil,
				}
			},
			expectedResult: false,
			expectedError:  lock.ErrClientRequired,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			redlock, err := lock.NewRedlock(tc.params(t))
			assert.ErrorIs(t, err, tc.expectedError)
			assert.Equal(t, tc.expectedResult, redlock != nil)
		})
	}
}

func TestRedlockAcquire(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		r              func(t *testing.T) *lock.Redlock
		ctx            context.Context
		tenant         valueobject.TenantID
		key            string
		ttl            time.Duration
		preAcquire     bool
		expectedResult bool
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "nil receiver rejected",
			r: func(_ *testing.T) *lock.Redlock {
				return nil
			},
			ctx: context.Background(),
			tenant: func() valueobject.TenantID {
				t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000001")
				return t
			}(),
			key:            "recon",
			ttl:            time.Minute,
			preAcquire:     false,
			expectedResult: false,
			expectedError:  lock.ErrNotInitialized,
		},
		{
			name: "uninitialized internal client rejected",
			r: func(_ *testing.T) *lock.Redlock {
				return &lock.Redlock{}
			},
			ctx: context.Background(),
			tenant: func() valueobject.TenantID {
				t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000001")
				return t
			}(),
			key:            "recon",
			ttl:            time.Minute,
			preAcquire:     false,
			expectedResult: false,
			expectedError:  lock.ErrNotInitialized,
		},
		{
			name: "successful acquire",
			r: func(t *testing.T) *lock.Redlock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: newMockValkeyStore(t)})
				return r
			},
			ctx: context.Background(),
			tenant: func() valueobject.TenantID {
				t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000001")
				return t
			}(),
			key:            "recon",
			ttl:            time.Minute,
			preAcquire:     false,
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "already held lock returns ErrLockHeld",
			r: func(t *testing.T) *lock.Redlock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: newMockValkeyStore(t)})
				return r
			},
			ctx: context.Background(),
			tenant: func() valueobject.TenantID {
				t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000001")
				return t
			}(),
			key:            "recon",
			ttl:            time.Minute,
			preAcquire:     true,
			expectedResult: false,
			expectedError:  lock.ErrLockHeld,
		},
		{
			name: "empty key rejected",
			r: func(t *testing.T) *lock.Redlock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: newMockValkeyStore(t)})
				return r
			},
			ctx: context.Background(),
			tenant: func() valueobject.TenantID {
				t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000001")
				return t
			}(),
			key:            "",
			ttl:            time.Minute,
			preAcquire:     false,
			expectedResult: false,
			expectedError:  lock.ErrKeySegmentRequired,
		},
		{
			name: "whitespace key rejected",
			r: func(t *testing.T) *lock.Redlock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: newMockValkeyStore(t)})
				return r
			},
			ctx: context.Background(),
			tenant: func() valueobject.TenantID {
				t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000001")
				return t
			}(),
			key:            "   ",
			ttl:            time.Minute,
			preAcquire:     false,
			expectedResult: false,
			expectedError:  lock.ErrKeySegmentRequired,
		},
		{
			name: "colon in key rejected",
			r: func(t *testing.T) *lock.Redlock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: newMockValkeyStore(t)})
				return r
			},
			ctx: context.Background(),
			tenant: func() valueobject.TenantID {
				t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000001")
				return t
			}(),
			key:            "a:b",
			ttl:            time.Minute,
			preAcquire:     false,
			expectedResult: false,
			expectedError:  lock.ErrKeySegmentColon,
		},
		{
			name: "key too long rejected",
			r: func(t *testing.T) *lock.Redlock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: newMockValkeyStore(t)})
				return r
			},
			ctx: context.Background(),
			tenant: func() valueobject.TenantID {
				t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000001")
				return t
			}(),
			key:            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			ttl:            time.Minute,
			preAcquire:     false,
			expectedResult: false,
			expectedError:  lock.ErrKeySegmentTooLong,
		},
		{
			name: "empty tenant rejected",
			r: func(t *testing.T) *lock.Redlock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: newMockValkeyStore(t)})
				return r
			},
			ctx:            context.Background(),
			tenant:         valueobject.TenantID(""),
			key:            "recon",
			ttl:            time.Minute,
			preAcquire:     false,
			expectedResult: false,
			expectedError:  lock.ErrKeySegmentRequired,
		},
		{
			name: "colon in tenant rejected",
			r: func(t *testing.T) *lock.Redlock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: newMockValkeyStore(t)})
				return r
			},
			ctx:            context.Background(),
			tenant:         valueobject.TenantID("bad:tenant"),
			key:            "recon",
			ttl:            time.Minute,
			preAcquire:     false,
			expectedResult: false,
			expectedError:  lock.ErrKeySegmentColon,
		},
		{
			name: "zero ttl rejected",
			r: func(t *testing.T) *lock.Redlock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: newMockValkeyStore(t)})
				return r
			},
			ctx: context.Background(),
			tenant: func() valueobject.TenantID {
				t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000001")
				return t
			}(),
			key:            "recon-zero-ttl",
			ttl:            0,
			preAcquire:     false,
			expectedResult: false,
			expectedError:  lock.ErrTTLPositive,
		},
		{
			name: "canceled context aborts",
			r: func(t *testing.T) *lock.Redlock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: newMockValkeyStore(t)})
				return r
			},
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			tenant: func() valueobject.TenantID {
				t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000001")
				return t
			}(),
			key:            "recon",
			ttl:            time.Minute,
			preAcquire:     false,
			expectedResult: false,
			expectedError:  context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := tc.r(t)
			if tc.preAcquire && r != nil {
				_, err := r.Acquire(context.Background(), tc.tenant, tc.key, tc.ttl)
				require.NoError(t, err)
			}

			if r == nil {
				lease, err := r.Acquire(tc.ctx, tc.tenant, tc.key, tc.ttl)
				assert.ErrorIs(t, err, tc.expectedError)
				assert.Nil(t, lease)
				return
			}

			lease, err := r.Acquire(tc.ctx, tc.tenant, tc.key, tc.ttl)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				assert.Nil(t, lease)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, lease != nil)
		})
	}
}

func TestRedlockLeaseRelease(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		lease         func(store *mocklock.MockValkeyStore) appport.Lock
		ctx           context.Context
		mutateStore   func(ctx context.Context, store *mocklock.MockValkeyStore)
		repeatRelease bool
		expectedError error
	}

	testCases := []testCase{
		{
			name: "nil lease rejected",
			lease: func(_ *mocklock.MockValkeyStore) appport.Lock {
				return nil
			},
			ctx:           context.Background(),
			mutateStore:   nil,
			repeatRelease: false,
			expectedError: lock.ErrLeaseNotInit,
		},
		{
			name: "successful release",
			lease: func(store *mocklock.MockValkeyStore) appport.Lock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: store})
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000003")
				lease, _ := r.Acquire(context.Background(), tenant, "job", time.Minute)
				return lease
			},
			ctx:           context.Background(),
			mutateStore:   nil,
			repeatRelease: false,
			expectedError: nil,
		},
		{
			name: "idempotent release",
			lease: func(store *mocklock.MockValkeyStore) appport.Lock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: store})
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000003")
				lease, _ := r.Acquire(context.Background(), tenant, "job", time.Minute)
				return lease
			},
			ctx:           context.Background(),
			mutateStore:   nil,
			repeatRelease: true,
			expectedError: nil,
		},
		{
			name: "release after key deleted externally succeeds",
			lease: func(store *mocklock.MockValkeyStore) appport.Lock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: store})
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000003")
				lease, _ := r.Acquire(context.Background(), tenant, "job", time.Minute)
				return lease
			},
			ctx: context.Background(),
			mutateStore: func(ctx context.Context, store *mocklock.MockValkeyStore) {
				_ = store.Delete(ctx, "lock:01950000-0000-7000-8000-000000000003:job")
			},
			repeatRelease: false,
			expectedError: nil,
		},
		{
			name: "release never steals token overwritten by another holder",
			lease: func(store *mocklock.MockValkeyStore) appport.Lock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: store})
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000003")
				lease, _ := r.Acquire(context.Background(), tenant, "job", time.Minute)
				return lease
			},
			ctx: context.Background(),
			mutateStore: func(ctx context.Context, store *mocklock.MockValkeyStore) {
				_ = store.Set(ctx, "lock:01950000-0000-7000-8000-000000000003:job", []byte("other-token"), time.Minute)
			},
			repeatRelease: false,
			expectedError: nil,
		},
		{
			name: "canceled context aborts release",
			lease: func(store *mocklock.MockValkeyStore) appport.Lock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: store})
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000003")
				lease, _ := r.Acquire(context.Background(), tenant, "job", time.Minute)
				return lease
			},
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			mutateStore:   nil,
			repeatRelease: false,
			expectedError: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := newMockValkeyStore(t)
			lease := tc.lease(store)

			if tc.mutateStore != nil {
				tc.mutateStore(context.Background(), store)
			}

			if lease == nil {
				assert.ErrorIs(t, tc.expectedError, lock.ErrLeaseNotInit)
				return
			}

			err := lease.Release(tc.ctx)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				return
			}
			require.NoError(t, err)

			if tc.repeatRelease {
				assert.NoError(t, lease.Release(tc.ctx))
			}
		})
	}
}

func TestRedlockLeaseRefresh(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		lease         func(store *mocklock.MockValkeyStore) appport.Lock
		ctx           context.Context
		ttl           time.Duration
		mutateStore   func(ctx context.Context, store *mocklock.MockValkeyStore)
		expectedError error
	}

	testCases := []testCase{
		{
			name: "nil lease rejected",
			lease: func(_ *mocklock.MockValkeyStore) appport.Lock {
				return nil
			},
			ctx:           context.Background(),
			ttl:           2 * time.Minute,
			mutateStore:   nil,
			expectedError: lock.ErrLeaseNotInit,
		},
		{
			name: "successful refresh",
			lease: func(store *mocklock.MockValkeyStore) appport.Lock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: store})
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000004")
				lease, _ := r.Acquire(context.Background(), tenant, "job", time.Minute)
				return lease
			},
			ctx:           context.Background(),
			ttl:           2 * time.Minute,
			mutateStore:   nil,
			expectedError: nil,
		},
		{
			name: "zero ttl rejected",
			lease: func(store *mocklock.MockValkeyStore) appport.Lock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: store})
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000004")
				lease, _ := r.Acquire(context.Background(), tenant, "job", time.Minute)
				return lease
			},
			ctx:           context.Background(),
			ttl:           0,
			mutateStore:   nil,
			expectedError: lock.ErrTTLPositive,
		},
		{
			name: "negative ttl rejected",
			lease: func(store *mocklock.MockValkeyStore) appport.Lock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: store})
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000004")
				lease, _ := r.Acquire(context.Background(), tenant, "job", time.Minute)
				return lease
			},
			ctx:           context.Background(),
			ttl:           -time.Second,
			mutateStore:   nil,
			expectedError: lock.ErrTTLPositive,
		},
		{
			name: "refresh fails after lock lost externally",
			lease: func(store *mocklock.MockValkeyStore) appport.Lock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: store})
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000004")
				lease, _ := r.Acquire(context.Background(), tenant, "job", time.Minute)
				return lease
			},
			ctx: context.Background(),
			ttl: time.Minute,
			mutateStore: func(ctx context.Context, store *mocklock.MockValkeyStore) {
				_ = store.Delete(ctx, "lock:01950000-0000-7000-8000-000000000004:job")
			},
			expectedError: lock.ErrLockLost,
		},
		{
			name: "canceled context aborts refresh",
			lease: func(store *mocklock.MockValkeyStore) appport.Lock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: store})
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000004")
				lease, _ := r.Acquire(context.Background(), tenant, "job", time.Minute)
				return lease
			},
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			ttl:           time.Minute,
			mutateStore:   nil,
			expectedError: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := newMockValkeyStore(t)
			lease := tc.lease(store)

			if tc.mutateStore != nil {
				tc.mutateStore(context.Background(), store)
			}

			if lease == nil {
				assert.ErrorIs(t, tc.expectedError, lock.ErrLeaseNotInit)
				return
			}

			err := lease.Refresh(tc.ctx, tc.ttl)
			if tc.expectedError != nil {
				if errors.Is(tc.expectedError, lock.ErrLockLost) || errors.Is(tc.expectedError, context.Canceled) {
					assert.ErrorIs(t, err, tc.expectedError)
				} else {
					assert.EqualError(t, err, tc.expectedError.Error())
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestWithLock(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		ctx           context.Context
		params        func(locker *mockapplication.MockDistributedLock, state map[string]bool) lock.GuardParams
		tenant        valueobject.TenantID
		key           string
		ttl           time.Duration
		fn            func(ctx context.Context) error
		lockerSetup   func(t *testing.T, locker *mockapplication.MockDistributedLock, state map[string]bool)
		expectedError error
		assertPost    func(t *testing.T, state map[string]bool)
	}

	testCases := []testCase{
		{
			name: "runs fn and releases",
			ctx:  context.Background(),
			params: func(locker *mockapplication.MockDistributedLock, state map[string]bool) lock.GuardParams {
				return lock.GuardParams{
					Locker:          locker,
					RefreshInterval: 5 * time.Millisecond,
				}
			},
			tenant: func() valueobject.TenantID {
				t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				return t
			}(),
			key: "recon",
			ttl: time.Minute,
			fn: func(_ context.Context) error {
				return nil
			},
			lockerSetup: func(t *testing.T, locker *mockapplication.MockDistributedLock, _ map[string]bool) {
				lease := mockapplication.NewMockLock(t)
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				locker.EXPECT().Acquire(mock.Anything, tenant, "recon", time.Minute).Return(lease, nil)
				lease.EXPECT().Release(mock.Anything).Return(nil)
			},
			expectedError: nil,
			assertPost:    nil,
		},
		{
			name: "contention surfaces held and fires hook",
			ctx:  context.Background(),
			params: func(locker *mockapplication.MockDistributedLock, state map[string]bool) lock.GuardParams {
				return lock.GuardParams{
					Locker:          locker,
					RefreshInterval: 5 * time.Millisecond,
					OnContended: func(_ context.Context, _ valueobject.TenantID, _ string) {
						state["contended"] = true
					},
				}
			},
			tenant: func() valueobject.TenantID {
				t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				return t
			}(),
			key: "recon",
			ttl: time.Minute,
			fn: func(_ context.Context) error {
				return nil
			},
			lockerSetup: func(t *testing.T, locker *mockapplication.MockDistributedLock, _ map[string]bool) {
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				locker.EXPECT().Acquire(mock.Anything, tenant, "recon", time.Minute).Return(nil, lock.ErrLockHeld)
			},
			expectedError: lock.ErrLockHeld,
			assertPost: func(t *testing.T, state map[string]bool) {
				assert.True(t, state["contended"])
			},
		},
		{
			name: "acquire failure does not fire contention hook",
			ctx:  context.Background(),
			params: func(locker *mockapplication.MockDistributedLock, state map[string]bool) lock.GuardParams {
				return lock.GuardParams{
					Locker:          locker,
					RefreshInterval: 5 * time.Millisecond,
					OnContended: func(_ context.Context, _ valueobject.TenantID, _ string) {
						state["contended"] = true
					},
				}
			},
			tenant: func() valueobject.TenantID {
				t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				return t
			}(),
			key: "recon",
			ttl: time.Minute,
			fn: func(_ context.Context) error {
				return nil
			},
			lockerSetup: func(t *testing.T, locker *mockapplication.MockDistributedLock, _ map[string]bool) {
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				locker.EXPECT().Acquire(mock.Anything, tenant, "recon", time.Minute).Return(nil, errors.New("valkey down"))
			},
			expectedError: errors.New("valkey down"),
			assertPost: func(t *testing.T, state map[string]bool) {
				assert.False(t, state["contended"])
			},
		},
		{
			name: "renews while fn runs",
			ctx:  context.Background(),
			params: func(locker *mockapplication.MockDistributedLock, state map[string]bool) lock.GuardParams {
				return lock.GuardParams{
					Locker:          locker,
					RefreshInterval: 5 * time.Millisecond,
				}
			},
			tenant: func() valueobject.TenantID {
				t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				return t
			}(),
			key: "long-recon",
			ttl: 30 * time.Second,
			fn: func(ctx context.Context) error {
				time.Sleep(40 * time.Millisecond)
				return ctx.Err()
			},
			lockerSetup: func(t *testing.T, locker *mockapplication.MockDistributedLock, state map[string]bool) {
				lease := mockapplication.NewMockLock(t)
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				locker.EXPECT().Acquire(mock.Anything, tenant, "long-recon", 30*time.Second).Return(lease, nil)
				var count atomic.Int64
				lease.EXPECT().Refresh(mock.Anything, 30*time.Second).RunAndReturn(func(context.Context, time.Duration) error {
					if count.Add(1) >= 3 {
						state["refreshed_enough"] = true
					}
					return nil
				}).Maybe()
				lease.EXPECT().Release(mock.Anything).Return(nil).Maybe()
			},
			expectedError: nil,
			assertPost: func(t *testing.T, state map[string]bool) {
				assert.True(t, state["refreshed_enough"])
			},
		},
		{
			name: "lost lease cancels fn and surfaces lock lost",
			ctx:  context.Background(),
			params: func(locker *mockapplication.MockDistributedLock, state map[string]bool) lock.GuardParams {
				return lock.GuardParams{
					Locker:          locker,
					RefreshInterval: 5 * time.Millisecond,
				}
			},
			tenant: func() valueobject.TenantID {
				t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				return t
			}(),
			key: "lost-recon",
			ttl: 30 * time.Second,
			fn: func(ctx context.Context) error {
				<-ctx.Done()
				return ctx.Err()
			},
			lockerSetup: func(t *testing.T, locker *mockapplication.MockDistributedLock, _ map[string]bool) {
				lease := mockapplication.NewMockLock(t)
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				locker.EXPECT().Acquire(mock.Anything, tenant, "lost-recon", 30*time.Second).Return(lease, nil)
				lease.EXPECT().Refresh(mock.Anything, 30*time.Second).Return(lock.ErrLockLost)
				lease.EXPECT().Release(mock.Anything).Return(nil).Maybe()
			},
			expectedError: lock.ErrLockLost,
			assertPost:    nil,
		},
		{
			name: "fn error wins over renewal error",
			ctx:  context.Background(),
			params: func(locker *mockapplication.MockDistributedLock, state map[string]bool) lock.GuardParams {
				return lock.GuardParams{
					Locker:          locker,
					RefreshInterval: 1 * time.Millisecond,
				}
			},
			tenant: func() valueobject.TenantID {
				t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				return t
			}(),
			key: "diverged-recon",
			ttl: 30 * time.Second,
			fn: func(context.Context) error {
				time.Sleep(20 * time.Millisecond)
				return errors.New("payment posted but reconciliation diverged")
			},
			lockerSetup: func(t *testing.T, locker *mockapplication.MockDistributedLock, _ map[string]bool) {
				lease := mockapplication.NewMockLock(t)
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				locker.EXPECT().Acquire(mock.Anything, tenant, "diverged-recon", 30*time.Second).Return(lease, nil)
				lease.EXPECT().Refresh(mock.Anything, 30*time.Second).Return(lock.ErrLockLost).Maybe()
				lease.EXPECT().Release(mock.Anything).Return(nil).Maybe()
			},
			expectedError: errors.New("payment posted but reconciliation diverged"),
			assertPost:    nil,
		},
		{
			name: "released hook fires and lease is freed",
			ctx:  context.Background(),
			params: func(locker *mockapplication.MockDistributedLock, state map[string]bool) lock.GuardParams {
				return lock.GuardParams{
					Locker:          locker,
					RefreshInterval: 5 * time.Millisecond,
					OnReleased: func(context.Context, valueobject.TenantID, string) {
						state["released"] = true
					},
				}
			},
			tenant: func() valueobject.TenantID {
				t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				return t
			}(),
			key: "hook-recon",
			ttl: time.Minute,
			fn: func(context.Context) error {
				return nil
			},
			lockerSetup: func(t *testing.T, locker *mockapplication.MockDistributedLock, _ map[string]bool) {
				lease := mockapplication.NewMockLock(t)
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				locker.EXPECT().Acquire(mock.Anything, tenant, "hook-recon", time.Minute).Return(lease, nil)
				lease.EXPECT().Release(mock.Anything).Return(nil)
			},
			expectedError: nil,
			assertPost: func(t *testing.T, state map[string]bool) {
				assert.True(t, state["released"])
			},
		},
		{
			name: "caller cancellation still releases the lease",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			params: func(locker *mockapplication.MockDistributedLock, state map[string]bool) lock.GuardParams {
				return lock.GuardParams{
					Locker:          locker,
					RefreshInterval: 5 * time.Millisecond,
				}
			},
			tenant: func() valueobject.TenantID {
				t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				return t
			}(),
			key: "cancel-recon",
			ttl: time.Minute,
			fn: func(context.Context) error {
				return nil
			},
			lockerSetup: func(t *testing.T, locker *mockapplication.MockDistributedLock, _ map[string]bool) {
				lease := mockapplication.NewMockLock(t)
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				locker.EXPECT().Acquire(mock.Anything, tenant, "cancel-recon", time.Minute).Return(lease, nil)
				lease.EXPECT().Release(mock.Anything).Return(nil)
			},
			expectedError: nil,
			assertPost:    nil,
		},
		{
			name: "nil locker rejected",
			ctx:  context.Background(),
			params: func(_ *mockapplication.MockDistributedLock, _ map[string]bool) lock.GuardParams {
				return lock.GuardParams{
					Locker:          nil,
					RefreshInterval: 5 * time.Millisecond,
				}
			},
			tenant: func() valueobject.TenantID {
				t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				return t
			}(),
			key: "recon",
			ttl: time.Minute,
			fn: func(_ context.Context) error {
				return nil
			},
			lockerSetup:   nil,
			expectedError: lock.ErrLockerRequired,
			assertPost:    nil,
		},
		{
			name: "nil fn rejected",
			ctx:  context.Background(),
			params: func(locker *mockapplication.MockDistributedLock, _ map[string]bool) lock.GuardParams {
				return lock.GuardParams{
					Locker:          locker,
					RefreshInterval: 5 * time.Millisecond,
				}
			},
			tenant: func() valueobject.TenantID {
				t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				return t
			}(),
			key:           "recon",
			ttl:           time.Minute,
			fn:            nil,
			lockerSetup:   nil,
			expectedError: lock.ErrFnRequired,
			assertPost:    nil,
		},
		{
			name: "fn completes while renewal interval is active without context error",
			ctx:  context.Background(),
			params: func(locker *mockapplication.MockDistributedLock, state map[string]bool) lock.GuardParams {
				return lock.GuardParams{
					Locker:          locker,
					RefreshInterval: time.Millisecond,
				}
			},
			tenant: func() valueobject.TenantID {
				t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				return t
			}(),
			key: "racing-renew",
			ttl: time.Second,
			fn: func(_ context.Context) error {
				time.Sleep(2 * time.Millisecond)
				return nil
			},
			lockerSetup: func(t *testing.T, locker *mockapplication.MockDistributedLock, _ map[string]bool) {
				lease := mockapplication.NewMockLock(t)
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				locker.EXPECT().Acquire(mock.Anything, tenant, "racing-renew", time.Second).Return(lease, nil)
				lease.EXPECT().Refresh(mock.Anything, time.Second).Return(nil).Maybe()
				lease.EXPECT().Release(mock.Anything).Return(nil)
			},
			expectedError: nil,
			assertPost:    nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			locker := mockapplication.NewMockDistributedLock(t)
			state := make(map[string]bool)
			if tc.lockerSetup != nil {
				tc.lockerSetup(t, locker, state)
			}

			guardParams := tc.params(locker, state)
			err := lock.WithLock(tc.ctx, guardParams, tc.tenant, tc.key, tc.ttl, tc.fn)
			if tc.expectedError != nil {
				if errors.Is(tc.expectedError, lock.ErrLockHeld) || errors.Is(tc.expectedError, lock.ErrLockLost) {
					assert.ErrorIs(t, err, tc.expectedError)
				} else {
					assert.EqualError(t, err, tc.expectedError.Error())
				}
				return
			}
			require.NoError(t, err)
			if tc.assertPost != nil {
				tc.assertPost(t, state)
			}
		})
	}
}

func TestWithLockRenewalPanicContained(t *testing.T) {
	t.Parallel()

	locker := mockapplication.NewMockDistributedLock(t)
	lease := mockapplication.NewMockLock(t)
	tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000003")
	locker.EXPECT().Acquire(mock.Anything, tenant, "panic-renewal", time.Second).Return(lease, nil)
	lease.EXPECT().Refresh(mock.Anything, time.Second).Panic("store exploded mid-refresh")
	lease.EXPECT().Release(mock.Anything).Return(nil)

	guardParams := lock.GuardParams{
		Locker:          locker,
		RefreshInterval: time.Millisecond,
	}

	err := lock.WithLock(context.Background(), guardParams, tenant, "panic-renewal", time.Second, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	require.Error(t, err)

	var contained safe.Panic
	require.ErrorAs(t, err, &contained)
	assert.Contains(t, contained.Value, "store exploded mid-refresh")
}
