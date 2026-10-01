package rbac_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/auth/rbac"
	mockrbac "github.com/kadekutama/go-template/test/mock/rbac"
)

func testEnforcer(t *testing.T, hook func(rbac.Decision)) *rbac.Enforcer {
	t.Helper()

	enforcer, err := rbac.NewEnforcer(rbac.RBACParams{
		Policies: []rbac.PolicyRule{
			{Sub: "admin", Dom: "tenant-acme", Obj: "ledger", Act: "write", Effect: "allow"},
			{Sub: "viewer", Dom: "tenant-acme", Obj: "ledger", Act: "read", Effect: "allow"},
		},
		Grouping: []rbac.GroupingRule{
			{User: "alice", Role: "admin", Dom: "tenant-acme"},
			{User: "bob", Role: "viewer", Dom: "tenant-acme"},
		},
		ThresholdAmountMinor: 100000,
		AuditHook:            hook,
	})
	require.NoError(t, err)

	return enforcer
}

func TestEnforcerAuthorize(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		ctx           context.Context
		subject       appport.Subject
		action        string
		resource      string
		attrs         rbac.Attributes
		expectedError error
	}

	testCases := []testCase{
		{
			name: "admin write allowed",
			ctx:  context.Background(),
			subject: appport.Subject{
				ID:       "alice",
				TenantID: valueobject.TenantID("tenant-acme"),
			},
			action:        "write",
			resource:      "ledger",
			attrs:         rbac.Attributes{},
			expectedError: nil,
		},
		{
			name: "viewer write denied",
			ctx:  context.Background(),
			subject: appport.Subject{
				ID:       "bob",
				TenantID: valueobject.TenantID("tenant-acme"),
			},
			action:        "write",
			resource:      "ledger",
			attrs:         rbac.Attributes{},
			expectedError: rbac.ErrForbidden,
		},
		{
			name: "cross tenant denied",
			ctx:  context.Background(),
			subject: appport.Subject{
				ID:       "alice",
				TenantID: valueobject.TenantID("tenant-beta"),
			},
			action:        "write",
			resource:      "ledger",
			attrs:         rbac.Attributes{},
			expectedError: rbac.ErrForbidden,
		},
		{
			name: "above threshold same approver denied",
			ctx:  context.Background(),
			subject: appport.Subject{
				ID:       "alice",
				TenantID: valueobject.TenantID("tenant-acme"),
			},
			action:   "write",
			resource: "ledger",
			attrs: rbac.Attributes{
				AmountMinor: 200000,
				Approver:    "alice",
				Resolver:    "alice",
			},
			expectedError: rbac.ErrForbidden,
		},
		{
			name: "above threshold distinct approver allowed",
			ctx:  context.Background(),
			subject: appport.Subject{
				ID:       "alice",
				TenantID: valueobject.TenantID("tenant-acme"),
			},
			action:   "write",
			resource: "ledger",
			attrs: rbac.Attributes{
				AmountMinor: 200000,
				Approver:    "carol",
				Resolver:    "alice",
			},
			expectedError: nil,
		},
		{
			name: "canceled context",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			subject: appport.Subject{
				ID:       "alice",
				TenantID: valueobject.TenantID("tenant-acme"),
			},
			action:        "write",
			resource:      "ledger",
			attrs:         rbac.Attributes{},
			expectedError: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var decisions []rbac.Decision
			enforcer := testEnforcer(t, func(d rbac.Decision) {
				decisions = append(decisions, d)
			})

			err := enforcer.AuthorizeWithAttrs(tc.ctx, tc.subject, tc.action, tc.resource, tc.attrs)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				return
			}

			assert.NoError(t, err)
			require.NotEmpty(t, decisions)
			assert.True(t, decisions[0].Allow)
		})
	}
}

func TestEnforcerReload(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		policies      []rbac.PolicyRule
		grouping      []rbac.GroupingRule
		action        string
		expectedError error
	}

	testCases := []testCase{
		{
			name: "reload grants write",
			policies: []rbac.PolicyRule{
				{Sub: "viewer", Dom: "tenant-acme", Obj: "ledger", Act: "write", Effect: "allow"},
			},
			grouping: []rbac.GroupingRule{
				{User: "bob", Role: "viewer", Dom: "tenant-acme"},
			},
			action:        "write",
			expectedError: nil,
		},
		{
			name:          "reload empty denies",
			policies:      nil,
			grouping:      nil,
			action:        "read",
			expectedError: rbac.ErrForbidden,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			enforcer := testEnforcer(t, nil)
			require.NoError(t, enforcer.Reload(tc.policies, tc.grouping))

			err := enforcer.Authorize(context.Background(), appport.Subject{
				ID:       "bob",
				TenantID: valueobject.TenantID("tenant-acme"),
			}, tc.action, "ledger")

			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestEnforcerReloadFrom(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		enforcer      *rbac.Enforcer
		loader        func(t *testing.T) rbac.PolicyLoader
		action        string
		expectedError error
	}

	testCases := []testCase{
		{
			name:     "successful reload from loader",
			enforcer: testEnforcer(t, nil),
			loader: func(t *testing.T) rbac.PolicyLoader {
				m := mockrbac.NewMockPolicyLoader(t)
				m.EXPECT().Load().Return([]rbac.PolicyRule{
					{Sub: "viewer", Dom: "tenant-acme", Obj: "ledger", Act: "write", Effect: "allow"},
				}, []rbac.GroupingRule{
					{User: "bob", Role: "viewer", Dom: "tenant-acme"},
				}, nil).Once()
				return m
			},
			action:        "write",
			expectedError: nil,
		},
		{
			name:          "nil loader returns error",
			enforcer:      testEnforcer(t, nil),
			loader:        nil,
			action:        "read",
			expectedError: rbac.ErrPolicyInvalid,
		},
		{
			name:     "nil enforcer returns ErrNotInit",
			enforcer: nil,
			loader: func(t *testing.T) rbac.PolicyLoader {
				return mockrbac.NewMockPolicyLoader(t)
			},
			action:        "read",
			expectedError: rbac.ErrNotInit,
		},
		{
			name:     "loader returns error",
			enforcer: testEnforcer(t, nil),
			loader: func(t *testing.T) rbac.PolicyLoader {
				m := mockrbac.NewMockPolicyLoader(t)
				m.EXPECT().Load().Return(nil, nil, rbac.ErrPolicyInvalid).Once()
				return m
			},
			action:        "read",
			expectedError: rbac.ErrPolicyInvalid,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var l rbac.PolicyLoader
			if tc.loader != nil {
				l = tc.loader(t)
			}
			err := tc.enforcer.ReloadFrom(l)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				return
			}

			assert.NoError(t, err)
			authErr := tc.enforcer.Authorize(context.Background(), appport.Subject{
				ID:       "bob",
				TenantID: valueobject.TenantID("tenant-acme"),
			}, tc.action, "ledger")
			assert.NoError(t, authErr)
		})
	}
}

func TestNewEnforcerWithLoader(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		params        func(t *testing.T) rbac.RBACParams
		expectedError error
	}

	testCases := []testCase{
		{
			name: "loader supplies initial policies",
			params: func(t *testing.T) rbac.RBACParams {
				m := mockrbac.NewMockPolicyLoader(t)
				m.EXPECT().Load().Return([]rbac.PolicyRule{
					{Sub: "admin", Dom: "tenant-acme", Obj: "ledger", Act: "delete", Effect: "allow"},
				}, []rbac.GroupingRule{
					{User: "alice", Role: "admin", Dom: "tenant-acme"},
				}, nil).Once()
				return rbac.RBACParams{Loader: m, ThresholdAmountMinor: 100000}
			},
			expectedError: nil,
		},
		{
			name: "loader error fails construction",
			params: func(t *testing.T) rbac.RBACParams {
				m := mockrbac.NewMockPolicyLoader(t)
				m.EXPECT().Load().Return(nil, nil, rbac.ErrPolicyInvalid).Once()
				return rbac.RBACParams{Loader: m, ThresholdAmountMinor: 100000}
			},
			expectedError: rbac.ErrPolicyInvalid,
		},
		{
			name: "zero threshold rejected",
			params: func(t *testing.T) rbac.RBACParams {
				return rbac.RBACParams{ThresholdAmountMinor: 0}
			},
			expectedError: rbac.ErrPolicyInvalid,
		},
		{
			name: "negative threshold rejected",
			params: func(t *testing.T) rbac.RBACParams {
				return rbac.RBACParams{ThresholdAmountMinor: -1}
			},
			expectedError: rbac.ErrPolicyInvalid,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			enf, err := rbac.NewEnforcer(tc.params(t))
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				return
			}

			assert.NoError(t, err)
			require.NotNil(t, enf)
			authErr := enf.Authorize(context.Background(), appport.Subject{
				ID:       "alice",
				TenantID: valueobject.TenantID("tenant-acme"),
			}, "delete", "ledger")
			assert.NoError(t, authErr)
		})
	}
}

func TestEnforcerConcurrentReload(t *testing.T) {
	t.Parallel()

	enforcer, err := rbac.NewEnforcer(rbac.RBACParams{
		Policies: []rbac.PolicyRule{
			{Sub: "admin", Dom: "tenant-acme", Obj: "ledger", Act: "write", Effect: "allow"},
		},
		Grouping: []rbac.GroupingRule{
			{User: "alice", Role: "admin", Dom: "tenant-acme"},
		},
		ThresholdAmountMinor: 100000,
	})
	require.NoError(t, err)

	alice := appport.Subject{ID: "alice", TenantID: valueobject.TenantID("tenant-acme")}
	allow := []rbac.PolicyRule{
		{Sub: "admin", Dom: "tenant-acme", Obj: "ledger", Act: "write", Effect: "allow"},
	}
	deny := []rbac.PolicyRule{
		{Sub: "viewer", Dom: "tenant-acme", Obj: "ledger", Act: "read", Effect: "allow"},
	}
	grouping := []rbac.GroupingRule{
		{User: "alice", Role: "admin", Dom: "tenant-acme"},
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = enforcer.Authorize(context.Background(), alice, "write", "ledger")
				}
			}
		}()
	}

	for i := 0; i < 20; i++ {
		if i%2 == 0 {
			assert.NoError(t, enforcer.Reload(allow, grouping))
		} else {
			assert.NoError(t, enforcer.Reload(deny, grouping))
		}
	}

	close(stop)
	wg.Wait()

	require.NoError(t, enforcer.Reload(allow, grouping))
	assert.NoError(t, enforcer.Authorize(context.Background(), alice, "write", "ledger"))
}
