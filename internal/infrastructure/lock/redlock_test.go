package lock_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/valkey"
	"github.com/kadekutama/go-template/internal/infrastructure/lock"
	"github.com/kadekutama/go-template/internal/shared/kernel/safe"
)

func TestNewRedlock(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		params         lock.RedlockParams
		expectedResult bool
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "client provided",
			params: lock.RedlockParams{
				Client: newFakeStore(),
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "nil client rejected",
			params: lock.RedlockParams{
				Client: nil,
			},
			expectedResult: false,
			expectedError:  lock.ErrClientRequired,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			redlock, err := lock.NewRedlock(tc.params)
			assert.ErrorIs(t, err, tc.expectedError)
			assert.Equal(t, tc.expectedResult, redlock != nil)
		})
	}
}

func TestRedlockAcquire(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		r              *lock.Redlock
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
			r:    nil,
			ctx:  context.Background(),
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
			r:    &lock.Redlock{},
			ctx:  context.Background(),
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
			r: func() *lock.Redlock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: newFakeStore()})
				return r
			}(),
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
			r: func() *lock.Redlock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: newFakeStore()})
				return r
			}(),
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
			r: func() *lock.Redlock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: newFakeStore()})
				return r
			}(),
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
			r: func() *lock.Redlock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: newFakeStore()})
				return r
			}(),
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
			r: func() *lock.Redlock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: newFakeStore()})
				return r
			}(),
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
			r: func() *lock.Redlock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: newFakeStore()})
				return r
			}(),
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
			r: func() *lock.Redlock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: newFakeStore()})
				return r
			}(),
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
			r: func() *lock.Redlock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: newFakeStore()})
				return r
			}(),
			ctx:            context.Background(),
			tenant:         valueobject.TenantID("bad:tenant"),
			key:            "recon",
			ttl:            time.Minute,
			preAcquire:     false,
			expectedResult: false,
			expectedError:  lock.ErrKeySegmentColon,
		},
		{
			name: "zero ttl defaults to DefaultTTL",
			r: func() *lock.Redlock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: newFakeStore()})
				return r
			}(),
			ctx: context.Background(),
			tenant: func() valueobject.TenantID {
				t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000001")
				return t
			}(),
			key:            "recon-zero-ttl",
			ttl:            0,
			preAcquire:     false,
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "canceled context aborts",
			r: func() *lock.Redlock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: newFakeStore()})
				return r
			}(),
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
			if tc.preAcquire && tc.r != nil {
				_, err := tc.r.Acquire(context.Background(), tc.tenant, tc.key, tc.ttl)
				require.NoError(t, err)
			}

			lease, err := tc.r.Acquire(tc.ctx, tc.tenant, tc.key, tc.ttl)
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

// fakeStore is an in-memory ValkeyStore for lease-path tests.
type fakeStore struct {
	mu   sync.Mutex
	data map[string][]byte
	fail error
}

func newFakeStore() *fakeStore {
	return &fakeStore{data: make(map[string][]byte)}
}

func (s *fakeStore) Get(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.fail != nil {
		return nil, s.fail
	}

	value, ok := s.data[key]
	if !ok {
		return nil, valkey.ErrMiss
	}

	out := make([]byte, len(value))
	copy(out, value)

	return out, nil
}

func (s *fakeStore) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.fail != nil {
		return s.fail
	}

	out := make([]byte, len(value))
	copy(out, value)
	s.data[key] = out

	return nil
}

func (s *fakeStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.fail != nil {
		return s.fail
	}

	delete(s.data, key)

	return nil
}

func (s *fakeStore) SetNX(_ context.Context, key string, value []byte, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.fail != nil {
		return false, s.fail
	}

	if _, ok := s.data[key]; ok {
		return false, nil
	}

	out := make([]byte, len(value))
	copy(out, value)
	s.data[key] = out

	return true, nil
}

func TestRedlockLeaseRelease(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		lease         func(store *fakeStore) appport.Lock
		ctx           context.Context
		mutateStore   func(ctx context.Context, store *fakeStore)
		repeatRelease bool
		expectedError error
	}

	testCases := []testCase{
		{
			name: "nil lease rejected",
			lease: func(_ *fakeStore) appport.Lock {
				return nil
			},
			ctx:           context.Background(),
			mutateStore:   nil,
			repeatRelease: false,
			expectedError: lock.ErrLeaseNotInit,
		},
		{
			name: "successful release",
			lease: func(store *fakeStore) appport.Lock {
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
			lease: func(store *fakeStore) appport.Lock {
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
			lease: func(store *fakeStore) appport.Lock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: store})
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000003")
				lease, _ := r.Acquire(context.Background(), tenant, "job", time.Minute)
				return lease
			},
			ctx: context.Background(),
			mutateStore: func(ctx context.Context, store *fakeStore) {
				_ = store.Delete(ctx, "lock:01950000-0000-7000-8000-000000000003:job")
			},
			repeatRelease: false,
			expectedError: nil,
		},
		{
			name: "release never steals token overwritten by another holder",
			lease: func(store *fakeStore) appport.Lock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: store})
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000003")
				lease, _ := r.Acquire(context.Background(), tenant, "job", time.Minute)
				return lease
			},
			ctx: context.Background(),
			mutateStore: func(ctx context.Context, store *fakeStore) {
				_ = store.Set(ctx, "lock:01950000-0000-7000-8000-000000000003:job", []byte("other-token"), time.Minute)
			},
			repeatRelease: false,
			expectedError: nil,
		},
		{
			name: "canceled context aborts release",
			lease: func(store *fakeStore) appport.Lock {
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
			store := newFakeStore()
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
		lease         func(store *fakeStore) appport.Lock
		ctx           context.Context
		ttl           time.Duration
		mutateStore   func(ctx context.Context, store *fakeStore)
		expectedError error
	}

	testCases := []testCase{
		{
			name: "nil lease rejected",
			lease: func(_ *fakeStore) appport.Lock {
				return nil
			},
			ctx:           context.Background(),
			ttl:           2 * time.Minute,
			mutateStore:   nil,
			expectedError: lock.ErrLeaseNotInit,
		},
		{
			name: "successful refresh",
			lease: func(store *fakeStore) appport.Lock {
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
			lease: func(store *fakeStore) appport.Lock {
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
			lease: func(store *fakeStore) appport.Lock {
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
			lease: func(store *fakeStore) appport.Lock {
				r, _ := lock.NewRedlock(lock.RedlockParams{Client: store})
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000004")
				lease, _ := r.Acquire(context.Background(), tenant, "job", time.Minute)
				return lease
			},
			ctx: context.Background(),
			ttl: time.Minute,
			mutateStore: func(ctx context.Context, store *fakeStore) {
				_ = store.Delete(ctx, "lock:01950000-0000-7000-8000-000000000004:job")
			},
			expectedError: lock.ErrLockLost,
		},
		{
			name: "canceled context aborts refresh",
			lease: func(store *fakeStore) appport.Lock {
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
			store := newFakeStore()
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

// fakeLocker is an in-memory DistributedLock for guard tests. It records
// refreshes and can simulate a lost lease so auto-renewal is observable.
type fakeLocker struct {
	mu           sync.Mutex
	held         map[string]bool
	refreshes    int
	refreshErr   error
	panicRefresh bool
	acquireErr   error
}

func (f *fakeLocker) Acquire(_ context.Context, tenant valueobject.TenantID, key string, _ time.Duration) (appport.Lock, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.acquireErr != nil {
		return nil, f.acquireErr
	}

	full := tenant.String() + ":" + key
	if f.held[full] {
		return nil, lock.ErrLockHeld
	}

	if f.held == nil {
		f.held = make(map[string]bool)
	}
	f.held[full] = true

	return &fakeLease{locker: f, full: full}, nil
}

func (f *fakeLocker) refreshCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.refreshes
}

func (f *fakeLocker) isHeld(tenant valueobject.TenantID, key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.held[tenant.String()+":"+key]
}

type fakeLease struct {
	locker *fakeLocker
	full   string
}

func (l *fakeLease) Release(_ context.Context) error {
	l.locker.mu.Lock()
	defer l.locker.mu.Unlock()

	delete(l.locker.held, l.full)

	return nil
}

func (l *fakeLease) Refresh(_ context.Context, _ time.Duration) error {
	l.locker.mu.Lock()

	if l.locker.panicRefresh {
		l.locker.mu.Unlock()

		panic("store exploded mid-refresh")
	}

	if l.locker.refreshErr != nil {
		err := l.locker.refreshErr
		l.locker.mu.Unlock()

		return err
	}

	l.locker.refreshes++
	l.locker.mu.Unlock()

	return nil
}

func TestWithLock(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		ctx           context.Context
		params        func(locker *fakeLocker, state map[string]bool) lock.GuardParams
		tenant        valueobject.TenantID
		key           string
		ttl           time.Duration
		fn            func(ctx context.Context) error
		lockerSetup   func(locker *fakeLocker)
		expectedError error
		assertPost    func(t *testing.T, locker *fakeLocker, state map[string]bool)
	}

	testCases := []testCase{
		{
			name: "runs fn and releases",
			ctx:  context.Background(),
			params: func(locker *fakeLocker, state map[string]bool) lock.GuardParams {
				return lock.GuardParams{Locker: locker}
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
			expectedError: nil,
			assertPost: func(t *testing.T, locker *fakeLocker, state map[string]bool) {
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				assert.False(t, locker.isHeld(tenant, "recon"))
			},
		},
		{
			name: "contention surfaces held and fires hook",
			ctx:  context.Background(),
			params: func(locker *fakeLocker, state map[string]bool) lock.GuardParams {
				return lock.GuardParams{
					Locker: locker,
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
			lockerSetup: func(locker *fakeLocker) {
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				_, err := locker.Acquire(context.Background(), tenant, "recon", time.Minute)
				require.NoError(t, err)
			},
			expectedError: lock.ErrLockHeld,
			assertPost: func(t *testing.T, _ *fakeLocker, state map[string]bool) {
				assert.True(t, state["contended"])
			},
		},
		{
			name: "acquire failure does not fire contention hook",
			ctx:  context.Background(),
			params: func(locker *fakeLocker, state map[string]bool) lock.GuardParams {
				return lock.GuardParams{
					Locker: locker,
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
			lockerSetup: func(locker *fakeLocker) {
				locker.acquireErr = errors.New("valkey down")
			},
			expectedError: errors.New("valkey down"),
			assertPost: func(t *testing.T, _ *fakeLocker, state map[string]bool) {
				assert.False(t, state["contended"])
			},
		},
		{
			name: "renews while fn runs",
			ctx:  context.Background(),
			params: func(locker *fakeLocker, state map[string]bool) lock.GuardParams {
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
			lockerSetup:   nil,
			expectedError: nil,
			assertPost: func(t *testing.T, locker *fakeLocker, _ map[string]bool) {
				assert.GreaterOrEqual(t, locker.refreshCount(), 3)
			},
		},
		{
			name: "lost lease cancels fn and surfaces lock lost",
			ctx:  context.Background(),
			params: func(locker *fakeLocker, state map[string]bool) lock.GuardParams {
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
			lockerSetup: func(locker *fakeLocker) {
				locker.refreshErr = lock.ErrLockLost
			},
			expectedError: lock.ErrLockLost,
			assertPost:    nil,
		},
		{
			name: "fn error wins over renewal error",
			ctx:  context.Background(),
			params: func(locker *fakeLocker, state map[string]bool) lock.GuardParams {
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
			lockerSetup: func(locker *fakeLocker) {
				locker.refreshErr = lock.ErrLockLost
			},
			expectedError: errors.New("payment posted but reconciliation diverged"),
			assertPost:    nil,
		},
		{
			name: "released hook fires and lease is freed",
			ctx:  context.Background(),
			params: func(locker *fakeLocker, state map[string]bool) lock.GuardParams {
				return lock.GuardParams{
					Locker: locker,
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
			lockerSetup:   nil,
			expectedError: nil,
			assertPost: func(t *testing.T, locker *fakeLocker, state map[string]bool) {
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				assert.True(t, state["released"])
				assert.False(t, locker.isHeld(tenant, "hook-recon"))
			},
		},
		{
			name: "caller cancellation still releases the lease",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			params: func(locker *fakeLocker, state map[string]bool) lock.GuardParams {
				return lock.GuardParams{Locker: locker}
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
			lockerSetup:   nil,
			expectedError: nil,
			assertPost: func(t *testing.T, locker *fakeLocker, _ map[string]bool) {
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				assert.False(t, locker.isHeld(tenant, "cancel-recon"))
			},
		},
		{
			name: "nil locker rejected",
			ctx:  context.Background(),
			params: func(_ *fakeLocker, _ map[string]bool) lock.GuardParams {
				return lock.GuardParams{Locker: nil}
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
			params: func(locker *fakeLocker, _ map[string]bool) lock.GuardParams {
				return lock.GuardParams{Locker: locker}
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
			params: func(locker *fakeLocker, state map[string]bool) lock.GuardParams {
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
			lockerSetup:   nil,
			expectedError: nil,
			assertPost: func(t *testing.T, locker *fakeLocker, _ map[string]bool) {
				tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000002")
				assert.False(t, locker.isHeld(tenant, "racing-renew"))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			locker := &fakeLocker{}
			state := make(map[string]bool)
			if tc.lockerSetup != nil {
				tc.lockerSetup(locker)
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
				tc.assertPost(t, locker, state)
			}
		})
	}
}

func TestWithLockRenewalPanicContained(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		ctx           context.Context
		params        func(locker *fakeLocker) lock.GuardParams
		tenant        valueobject.TenantID
		key           string
		ttl           time.Duration
		fn            func(ctx context.Context) error
		expectedPanic string
	}

	testCases := []testCase{
		{
			name: "panic in renewal is contained and releases lease",
			ctx:  context.Background(),
			params: func(locker *fakeLocker) lock.GuardParams {
				return lock.GuardParams{
					Locker:          locker,
					RefreshInterval: time.Millisecond,
				}
			},
			tenant: func() valueobject.TenantID {
				t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000003")
				return t
			}(),
			key: "panic-renewal",
			ttl: time.Second,
			fn: func(ctx context.Context) error {
				<-ctx.Done()
				return ctx.Err()
			},
			expectedPanic: "store exploded mid-refresh",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			locker := &fakeLocker{panicRefresh: true}
			guardParams := tc.params(locker)

			err := lock.WithLock(tc.ctx, guardParams, tc.tenant, tc.key, tc.ttl, tc.fn)
			require.Error(t, err)

			var contained safe.Panic
			require.ErrorAs(t, err, &contained)
			assert.Contains(t, contained.Value, tc.expectedPanic)
			assert.False(t, locker.isHeld(tc.tenant, tc.key))
		})
	}
}
